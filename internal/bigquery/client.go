package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/lgarciaaco/claude-build-analyzer/pkg/shared"
	"google.golang.org/api/iterator"
)

const (
	defaultProjectID = "openshift-art"
	defaultDatasetID = "events"
	defaultTableID   = "builds"
	taskRunTableID   = "taskruns"
	
	// Concurrency limits to prevent resource overwhelm
	maxConcurrentQueries = 10
	
	// Standard partition window for optimal BigQuery performance
	standardPartitionDays = 90
)

// truncateToLastLines truncates log output to the last N lines to optimize memory usage
// Most error information appears at the end of logs
func truncateToLastLines(logOutput string, maxLines int) string {
	if logOutput == "" {
		return logOutput
	}
	
	lines := strings.Split(logOutput, "\n")
	if len(lines) <= maxLines {
		return logOutput
	}
	
	// Return the last maxLines lines
	startIndex := len(lines) - maxLines
	return strings.Join(lines[startIndex:], "\n")
}

// containerJSON represents the JSON structure for efficient parsing
// This matches the ContainerInfo fields to avoid map[string]interface{} overhead
type containerJSON struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Reason   string `json:"reason"`
	Image    string `json:"image"`
	IsInit   string `json:"is_init"`
	ExitCode string `json:"exit_code"`
	LogOutput string `json:"log_output"`
}

// toContainerInfo efficiently converts containerJSON to ContainerInfo
func (cj *containerJSON) toContainerInfo() shared.ContainerInfo {
	container := shared.ContainerInfo{
		Name:   cj.Name,
		State:  cj.State,
		Reason: cj.Reason,
		Image:  cj.Image,
		IsInit: cj.IsInit == "true",
	}

	// Handle exit_code conversion
	if cj.ExitCode != "" && cj.ExitCode != "0" {
		container.ExitCode = bigquery.NullInt64{Valid: true, Int64: 1}
	} else if cj.ExitCode == "0" {
		container.ExitCode = bigquery.NullInt64{Valid: true, Int64: 0}
	}

	// Handle log output with truncation
	if cj.LogOutput != "" {
		truncatedLog := truncateToLastLines(cj.LogOutput, 50)
		container.LogOutput = bigquery.NullString{Valid: true, StringVal: truncatedLog}
	}

	return container
}

// semaphore provides a simple semaphore implementation for limiting concurrent operations
type semaphore struct {
	semaCh chan struct{}
}

// newSemaphore creates a new semaphore with the specified limit
func newSemaphore(maxConcurrency int) *semaphore {
	return &semaphore{
		semaCh: make(chan struct{}, maxConcurrency),
	}
}

// acquire blocks until a semaphore slot is available
func (s *semaphore) acquire() {
	s.semaCh <- struct{}{}
}

// release frees up a semaphore slot
func (s *semaphore) release() {
	<-s.semaCh
}

// errorCollector accumulates errors from concurrent operations for proper aggregation
type errorCollector struct {
	errors []error
	mu     sync.Mutex
}

// addError safely adds an error to the collector
func (ec *errorCollector) addError(err error) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if err != nil {
		ec.errors = append(ec.errors, err)
	}
}

// hasErrors returns true if any errors were collected
func (ec *errorCollector) hasErrors() bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.errors) > 0
}

// combinedError returns a formatted error combining all collected errors
func (ec *errorCollector) combinedError() error {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	
	if len(ec.errors) == 0 {
		return nil
	}
	
	if len(ec.errors) == 1 {
		return ec.errors[0]
	}
	
	var errorStrings []string
	for _, err := range ec.errors {
		errorStrings = append(errorStrings, err.Error())
	}
	return fmt.Errorf("multiple errors occurred: %s", strings.Join(errorStrings, "; "))
}

// compareArchitectures efficiently compares two architecture slices using O(n) set operations
func compareArchitectures(arches1, arches2 []string) []string {
	// Create sets for O(1) lookup
	set1 := make(map[string]bool, len(arches1))
	set2 := make(map[string]bool, len(arches2))
	
	for _, arch := range arches1 {
		set1[arch] = true
	}
	for _, arch := range arches2 {
		set2[arch] = true
	}
	
	// Find symmetric difference (architectures present in only one build)
	var differences []string
	
	// Check for architectures in build1 but not in build2
	for arch := range set1 {
		if !set2[arch] {
			differences = append(differences, arch)
		}
	}
	
	// Check for architectures in build2 but not in build1
	for arch := range set2 {
		if !set1[arch] {
			differences = append(differences, arch)
		}
	}
	
	return differences
}

// QueryBuilder provides shared SQL query construction for BigQuery operations
type QueryBuilder struct {
	projectID string
	datasetID string
	tableID   string
}

// newQueryBuilder creates a new QueryBuilder instance
func (c *Client) newQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		projectID: c.projectID,
		datasetID: c.datasetID,
		tableID:   c.tableID,
	}
}

// baseSelectClause returns the standard column selection for build records
func (qb *QueryBuilder) baseSelectClause() string {
	return `SELECT 
		name, ` + "`group`" + `, version, release, assembly, el_target,
		arches, start_time, end_time, outcome, image_pullspec,
		build_pipeline_url, art_job_url, nvr, build_id, record_id,
		hermetic, embargoed, source_repo, commitish,
		installed_packages, parent_images`
}

// fromClause returns the FROM clause with proper table reference
func (qb *QueryBuilder) fromClause() string {
	return fmt.Sprintf("FROM `%s.%s.%s`", qb.projectID, qb.datasetID, qb.tableID)
}

// buildQuery constructs complete SQL with base SELECT, FROM, and custom WHERE clause
func (qb *QueryBuilder) buildQuery(whereClause string) string {
	return fmt.Sprintf("%s\n%s\n%s", 
		qb.baseSelectClause(), 
		qb.fromClause(), 
		whereClause)
}

// Client provides BigQuery operations for build data
type Client struct {
	client    *bigquery.Client
	projectID string
	datasetID string
	tableID   string
	mu        sync.RWMutex
}

// NewClient creates a new BigQuery client
func NewClient(ctx context.Context, projectID string) (*Client, error) {
	if projectID == "" {
		projectID = defaultProjectID
	}

	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %w", err)
	}

	return &Client{
		client:    client,
		projectID: projectID,
		datasetID: defaultDatasetID,
		tableID:   defaultTableID,
	}, nil
}

// Close closes the BigQuery client
func (c *Client) Close() error {
	return c.client.Close()
}

// QueryBuildFailures queries for build failures based on the provided filters
func (c *Client) QueryBuildFailures(ctx context.Context, query shared.BuildFailureQuery) ([]shared.BuildRecord, error) {
	// Set defaults
	if query.Assembly == "" {
		query.Assembly = "stream"
	}
	if query.Outcome == "" {
		query.Outcome = "FAILURE"
	}
	if query.Days == 0 {
		query.Days = 7
	}

	sqlQuery, params := c.buildFailureQuery(query)

	log.Printf("Executing BigQuery: %s", sqlQuery)

	q := c.client.Query(sqlQuery)
	for key, value := range params {
		q.Parameters = append(q.Parameters, bigquery.QueryParameter{
			Name:  key,
			Value: value,
		})
	}
	q.Location = "US"

	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var builds []shared.BuildRecord
	for {
		var build shared.BuildRecord
		err := it.Next(&build)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read query result: %w", err)
		}
		builds = append(builds, build)
	}

	log.Printf("Found %d build records", len(builds))
	return builds, nil
}

// GetLatestBuildForComponent gets the most recent build for a component
func (c *Client) GetLatestBuildForComponent(ctx context.Context, componentName, group, assembly string) (*shared.BuildRecord, error) {
	if assembly == "" {
		assembly = "stream"
	}

	// Add start_time filter for partition elimination
	startDate := time.Now().AddDate(0, 0, -standardPartitionDays)

	qb := c.newQueryBuilder()
	sqlQuery := qb.buildQuery(`WHERE start_time >= @startDate
		AND name = @componentName
		AND assembly = @assembly`)

	params := map[string]interface{}{
		"startDate":     startDate,
		"componentName": componentName,
		"assembly":      assembly,
	}

	if group != "" {
		sqlQuery += " AND `group` = @group"
		params["group"] = group
	}

	sqlQuery += " ORDER BY start_time DESC LIMIT 1"

	q := c.client.Query(sqlQuery)
	for key, value := range params {
		q.Parameters = append(q.Parameters, bigquery.QueryParameter{
			Name:  key,
			Value: value,
		})
	}
	q.Location = "US"

	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var build shared.BuildRecord
	err = it.Next(&build)
	if err == iterator.Done {
		return nil, nil // No builds found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read query result: %w", err)
	}

	return &build, nil
}

// CompareBuilds compares two builds by their build IDs
func (c *Client) CompareBuilds(ctx context.Context, buildID1, buildID2 string) (*shared.BuildComparison, error) {
	// Add start_time filter for partition elimination
	startDate := time.Now().AddDate(0, 0, -standardPartitionDays)

	qb := c.newQueryBuilder()
	sqlQuery := qb.buildQuery(`WHERE start_time >= @startDate
		AND build_id IN (@buildId1, @buildId2)
		ORDER BY start_time DESC`)

	q := c.client.Query(sqlQuery)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "startDate", Value: startDate},
		{Name: "buildId1", Value: buildID1},
		{Name: "buildId2", Value: buildID2},
	}
	q.Location = "US"

	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var builds []shared.BuildRecord
	for {
		var build shared.BuildRecord
		err := it.Next(&build)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read query result: %w", err)
		}
		builds = append(builds, build)
	}

	var build1, build2 *shared.BuildRecord
	for i := range builds {
		if builds[i].BuildID == buildID1 {
			build1 = &builds[i]
		}
		if builds[i].BuildID == buildID2 {
			build2 = &builds[i]
		}
	}

	differences := c.findBuildDifferences(build1, build2)

	return &shared.BuildComparison{
		Build1:      build1,
		Build2:      build2,
		Differences: differences,
	}, nil
}

// SearchBuildsByPattern searches for builds using name pattern matching
func (c *Client) SearchBuildsByPattern(ctx context.Context, namePattern, group string, days int) ([]shared.BuildRecord, error) {
	if days == 0 {
		days = 7
	}

	startDate := time.Now().AddDate(0, 0, -days)

	sqlQuery := fmt.Sprintf(`
		SELECT DISTINCT
			name, `+"`group`"+`, version, release, assembly, el_target,
			arches, start_time, end_time, outcome, image_pullspec,
			build_pipeline_url, art_job_url, nvr, build_id, record_id,
			hermetic, embargoed, source_repo, commitish,
			installed_packages, parent_images
		FROM `+"`%s.%s.%s`"+`
		WHERE REGEXP_CONTAINS(name, @namePattern)
		AND start_time >= @startDate`,
		c.projectID, c.datasetID, c.tableID)

	params := map[string]interface{}{
		"namePattern": namePattern,
		"startDate":   startDate.Format("2006-01-02"),
	}

	if group != "" {
		sqlQuery += " AND `group` = @group"
		params["group"] = group
	}

	sqlQuery += " ORDER BY start_time DESC LIMIT 50"

	q := c.client.Query(sqlQuery)
	for key, value := range params {
		q.Parameters = append(q.Parameters, bigquery.QueryParameter{
			Name:  key,
			Value: value,
		})
	}
	q.Location = "US"

	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var builds []shared.BuildRecord
	for {
		var build shared.BuildRecord
		err := it.Next(&build)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read query result: %w", err)
		}
		builds = append(builds, build)
	}

	return builds, nil
}

// QueryComponentsAsync queries multiple components concurrently with semaphore-based limits
func (c *Client) QueryComponentsAsync(ctx context.Context, componentNames []string, query shared.BuildFailureQuery) (map[string][]shared.BuildRecord, error) {
	type result struct {
		component string
		builds    []shared.BuildRecord
		err       error
	}

	results := make(chan result, len(componentNames))
	var wg sync.WaitGroup
	
	// Create semaphore to limit concurrent operations
	sem := newSemaphore(maxConcurrentQueries)

	// Query each component concurrently with limited concurrency
	for _, component := range componentNames {
		wg.Add(1)
		go func(comp string) {
			defer wg.Done()
			
			// Acquire semaphore slot (blocks if limit reached)
			sem.acquire()
			defer sem.release()

			compQuery := query
			compQuery.ComponentNames = []string{comp}

			builds, err := c.QueryBuildFailures(ctx, compQuery)
			results <- result{
				component: comp,
				builds:    builds,
				err:       err,
			}
		}(component)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results with proper error aggregation
	componentBuilds := make(map[string][]shared.BuildRecord)
	var errCollector errorCollector
	
	for res := range results {
		if res.err != nil {
			log.Printf("Error querying component %s: %v", res.component, res.err)
			errCollector.addError(fmt.Errorf("failed to query component %s: %w", res.component, res.err))
			continue
		}
		componentBuilds[res.component] = res.builds
	}

	// Return partial results with aggregated error if any failures occurred
	if errCollector.hasErrors() {
		return componentBuilds, errCollector.combinedError()
	}

	return componentBuilds, nil
}

// buildFailureQuery constructs the SQL query for build failures
func (c *Client) buildFailureQuery(query shared.BuildFailureQuery) (string, map[string]interface{}) {
	var conditions []string
	params := make(map[string]interface{})

	// Base query using shared builder
	qb := c.newQueryBuilder()
	baseQuery := fmt.Sprintf("%s\n%s\nWHERE 1=1", qb.baseSelectClause(), qb.fromClause())
	sqlQuery := baseQuery

	// Time filter
	startDate := time.Now().AddDate(0, 0, -query.Days)
	conditions = append(conditions, "start_time >= @startDate")
	params["startDate"] = startDate.Format("2006-01-02")

	// Component names filter
	if len(query.ComponentNames) > 0 {
		conditions = append(conditions, "name IN UNNEST(@componentNames)")
		params["componentNames"] = query.ComponentNames
	}

	// Group filter
	if query.Group != "" {
		conditions = append(conditions, "`group` = @group")
		params["group"] = query.Group
	}

	// Assembly filter
	conditions = append(conditions, "assembly = @assembly")
	params["assembly"] = query.Assembly

	// Outcome filter
	conditions = append(conditions, "outcome = @outcome")
	params["outcome"] = query.Outcome

	// Architecture filter
	if len(query.Architectures) > 0 {
		conditions = append(conditions, `
			EXISTS (
				SELECT 1 FROM UNNEST(arches) AS arch 
				WHERE arch IN UNNEST(@architectures)
			)`)
		params["architectures"] = query.Architectures
	}

	// Hermetic filter
	if query.Hermetic != nil {
		conditions = append(conditions, "hermetic = @hermetic")
		params["hermetic"] = *query.Hermetic
	}

	// Build query efficiently using string builder
	var builder strings.Builder
	builder.WriteString(baseQuery)
	
	// Add all conditions
	for _, condition := range conditions {
		builder.WriteString(" AND ")
		builder.WriteString(condition)
	}

	// Order and limit
	builder.WriteString(" ORDER BY start_time DESC LIMIT 100")
	sqlQuery = builder.String()

	return sqlQuery, params
}

// findBuildDifferences identifies differences between two builds
func (c *Client) findBuildDifferences(build1, build2 *shared.BuildRecord) []string {
	if build1 == nil || build2 == nil {
		return []string{"One or both builds not found"}
	}

	var differences []string

	// Compare key fields
	if build1.Outcome != build2.Outcome {
		differences = append(differences, fmt.Sprintf("Outcome: %s vs %s", build1.Outcome, build2.Outcome))
	}

	if build1.Hermetic != build2.Hermetic {
		differences = append(differences, fmt.Sprintf("Hermetic mode: %t vs %t", build1.Hermetic, build2.Hermetic))
	}

	if build1.SourceRepo != build2.SourceRepo {
		differences = append(differences, fmt.Sprintf("Source repo: %s vs %s", build1.SourceRepo, build2.SourceRepo))
	}

	if build1.Commitish != build2.Commitish {
		differences = append(differences, fmt.Sprintf("Commit: %s vs %s", build1.Commitish, build2.Commitish))
	}

	// Compare architectures using optimized O(n) set operations
	archDiff := compareArchitectures(build1.Arches, build2.Arches)

	if len(archDiff) > 0 {
		differences = append(differences, fmt.Sprintf("Architecture differences: %s", strings.Join(archDiff, ", ")))
	}

	return differences
}

// GetTaskRunsForBuild retrieves TaskRun records for a specific build ID
func (c *Client) GetTaskRunsForBuild(ctx context.Context, buildID string) ([]shared.TaskRunRecord, error) {
	// Add start_time filter for partition elimination
	startDate := time.Now().AddDate(0, 0, -standardPartitionDays)

	sqlQuery := fmt.Sprintf(`
		SELECT 
			creation_time, task, task_run, task_run_uid, pipeline_run, 
			pipeline_run_uid, pod_name, pod_phase, scheduled_time, 
			initialized_time, start_time, max_finished_time, containers, 
			TO_JSON_STRING(containers) as containers_json,
			capture_time, success, build_id, record_id
		FROM `+"`%s.%s.%s`"+`
		WHERE creation_time >= @startDate
		AND build_id = @buildId
		ORDER BY creation_time`,
		c.projectID, c.datasetID, taskRunTableID)

	q := c.client.Query(sqlQuery)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "startDate", Value: startDate},
		{Name: "buildId", Value: buildID},
	}
	q.Location = "US"

	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute TaskRun query: %w", err)
	}

	var taskRuns []shared.TaskRunRecord
	for {
		var taskRun shared.TaskRunRecord
		err := it.Next(&taskRun)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read TaskRun result: %w", err)
		}
		
		
		// Parse containers JSON efficiently using structured parsing
		if taskRun.ContainersJSON != "" {
			var jsonContainers []containerJSON
			if err := json.Unmarshal([]byte(taskRun.ContainersJSON), &jsonContainers); err != nil {
				log.Printf("Warning: Failed to parse containers JSON: %v", err)
			} else {
				// Convert JSON containers to ContainerInfo using optimized conversion
				taskRun.Containers = make([]shared.ContainerInfo, len(jsonContainers))
				for i, jsonContainer := range jsonContainers {
					taskRun.Containers[i] = jsonContainer.toContainerInfo()
				}
			}
		}
		
		taskRuns = append(taskRuns, taskRun)
	}

	return taskRuns, nil
}

// GetTaskRunsForComponent retrieves TaskRun records for the latest build of a component
func (c *Client) GetTaskRunsForComponent(ctx context.Context, componentName, group, assembly string) ([]shared.TaskRunRecord, error) {
	if assembly == "" {
		assembly = "stream"
	}

	// First get the latest build for the component
	build, err := c.GetLatestBuildForComponent(ctx, componentName, group, assembly)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest build for component: %w", err)
	}
	if build == nil {
		return nil, nil
	}

	// Then get TaskRuns for that build
	return c.GetTaskRunsForBuild(ctx, build.BuildID)
}

// ExtractFailedContainerLogs extracts logs from failed containers in TaskRun records
func (c *Client) ExtractFailedContainerLogs(taskRuns []shared.TaskRunRecord) []shared.ContainerInfo {
	var failedContainers []shared.ContainerInfo
	emptyLogCount := 0
	totalContainers := 0

	for _, taskRun := range taskRuns {
		for _, container := range taskRun.Containers {
			totalContainers++
			
			// Include containers that failed or have non-empty log output
			if (container.ExitCode.Valid && container.ExitCode.Int64 != 0) || 
			   container.State == "terminated" && container.Reason != "Completed" ||
			   container.GetLogOutput() != "" {
				
				if container.GetLogOutput() == "" {
					emptyLogCount++
					exitCodeVal := "nil"
					if container.ExitCode.Valid {
						exitCodeVal = fmt.Sprintf("%d", container.ExitCode.Int64)
					}
					log.Printf("Warning: Failed container %s has empty log output (state: %s, reason: %s, exitCode: %s)", 
						container.Name, container.State, container.Reason, exitCodeVal)
				}
				
				failedContainers = append(failedContainers, container)
			}
		}
	}

	if emptyLogCount > 0 {
		log.Printf("Warning: %d out of %d failed containers have empty log output", emptyLogCount, len(failedContainers))
	}
	
	log.Printf("Extracted %d failed containers from %d total containers across %d TaskRuns", 
		len(failedContainers), totalContainers, len(taskRuns))

	return failedContainers
}

// AnalyzeContainerLogs analyzes container logs for common error patterns
func (c *Client) AnalyzeContainerLogs(containers []shared.ContainerInfo) []string {
	var errorSummary []string
	errorPatterns := map[string]string{
		"permission denied":                "Permission/access issues",
		"no space left":                    "Disk space issues",
		"connection refused":               "Network connectivity issues",
		"timeout":                          "Timeout issues",
		"out of memory":                    "Memory issues",
		"killed":                           "Process killed (likely OOM or timeout)",
		"error: failed to":                 "Build step failure",
		"fatal: unable to":                 "Git/source issues",
		"failed to pull":                   "Image pull issues",
		"npm err":                          "NPM/Node.js issues",
		"go: module":                       "Go module issues",
		"python: can't":                    "Python dependency issues",
		"error: failed to solve":           "Docker build issues",
		"error: build failed":              "Generic build failure",
	}

	foundErrors := make(map[string]bool)
	for _, container := range containers {
		logOutput := container.GetLogOutput()
		if logOutput == "" {
			continue
		}
		logContent := strings.ToLower(logOutput)
		for pattern, description := range errorPatterns {
			if strings.Contains(logContent, pattern) && !foundErrors[description] {
				errorSummary = append(errorSummary, fmt.Sprintf("%s (in %s)", description, container.Name))
				foundErrors[description] = true
			}
		}
	}

	return errorSummary
}
