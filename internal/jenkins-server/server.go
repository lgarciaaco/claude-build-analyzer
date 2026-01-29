package jenkinsserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/lgarciaaco/claude-build-analyzer/internal/jenkins"
	"github.com/lgarciaaco/claude-build-analyzer/pkg/shared"
)

// Server implements the Jenkins MCP server for Konflux build log analysis
type Server struct {
	*shared.BaseMCPServer
	jenkinsClient *jenkins.Client
	ctx           context.Context
}

// JenkinsJobQuery represents parameters for querying Jenkins builds
type JenkinsJobQuery struct {
	ComponentName string   `json:"componentName,omitempty"`
	Assembly      string   `json:"assembly,omitempty"`
	Group         string   `json:"group,omitempty"`
	Days          int      `json:"days,omitempty"`
	Status        []string `json:"status,omitempty"`
}

// JenkinsLogAnalysis represents Jenkins log analysis results
type JenkinsLogAnalysis struct {
	BuildNumber      int                    `json:"buildNumber"`
	JobName          string                 `json:"jobName"`
	Status           string                 `json:"status"`
	Timestamp        time.Time              `json:"timestamp"`
	Duration         time.Duration          `json:"duration"`
	LogURL           string                 `json:"logUrl"`
	Component        string                 `json:"component"`
	Assembly         string                 `json:"assembly"`
	Group            string                 `json:"group"`
	ErrorSummary     []string               `json:"errorSummary"`
	LogExcerpts      []string               `json:"logExcerpts"`
	Parameters       map[string]string      `json:"parameters"`
	Analysis         map[string]interface{} `json:"analysis"`
	FailedComponents []ComponentFailure     `json:"failedComponents,omitempty"`
	KonfluxLinks     []KonfluxLink          `json:"konfluxLinks"`
}

// ComponentFailure represents a failed component with its details
type ComponentFailure struct {
	Name         string `json:"name"`
	ErrorType    string `json:"errorType"`
	ErrorMessage string `json:"errorMessage"`
	KonfluxURL   string `json:"konfluxUrl,omitempty"`
}

// KonfluxLink represents a Konflux build link found in logs
type KonfluxLink struct {
	Component string `json:"component"`
	URL       string `json:"url"`
	Context   string `json:"context"`
}

// JenkinsCorrelation represents correlation between Jenkins and BigQuery data
type JenkinsCorrelation struct {
	JenkinsJob   jenkins.KonfluxJob     `json:"jenkinsJob"`
	CorrelatedID string                 `json:"correlatedBuildId,omitempty"`
	Confidence   float64                `json:"confidence"`
	MatchFactors []string               `json:"matchFactors"`
	Analysis     map[string]interface{} `json:"analysis"`
}

// NewServer creates a new Jenkins MCP server
func NewServer(ctx context.Context) (*Server, error) {
	jenkinsURL := "https://art-jenkins.apps.prod-stable-spoke1-dc-iad2.itup.redhat.com"

	// Check for Jenkins authentication from environment
	var auth *jenkins.JenkinsAuth
	if username := os.Getenv("JENKINS_USERNAME"); username != "" {
		if token := os.Getenv("JENKINS_TOKEN"); token != "" {
			auth = &jenkins.JenkinsAuth{
				Username: username,
				Token:    token,
			}
		}
	}

	jenkinsClient := jenkins.NewClient(jenkinsURL, auth)

	server := &Server{
		jenkinsClient: jenkinsClient,
		ctx:           ctx,
	}

	// Create base server with tool list function
	server.BaseMCPServer = shared.NewBaseMCPServer("jenkins-server", "1.0.0", ctx, server.GetToolList)

	// Register Jenkins-focused tools
	server.RegisterTool("query_jenkins_builds", server.queryJenkinsBuilds)
	server.RegisterTool("analyze_jenkins_logs", server.analyzeJenkinsLogs)
	server.RegisterTool("correlate_jenkins_builds", server.correlateJenkinsBuilds)
	server.RegisterTool("open_browser_links", server.openBrowserLinks)

	return server, nil
}

// Initialize initializes the Jenkins server
func (s *Server) Initialize() error {
	log.Println("Initializing Jenkins Server...")
	log.Println("Jenkins Server initialized successfully")
	return nil
}

// GetToolList returns the list of available Jenkins tools
func (s *Server) GetToolList() shared.ToolList {
	return shared.ToolList{
		Tools: []shared.Tool{
			{
				Name:        "query_jenkins_builds",
				Description: "Query Jenkins for Konflux builds matching component, assembly, and timeframe criteria. Returns Jenkins job metadata with build status and log URLs.",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"componentName": {
							Type:        "string",
							Description: "Component name to search for in Jenkins builds (optional)",
						},
						"assembly": {
							Type:        "string",
							Description: "Assembly type (stream, test, standard, custom, preview). Defaults to 'stream' for continuous builds.",
						},
						"group": {
							Type:        "string",
							Description: "OpenShift version group (e.g., 'openshift-4.21'). Defaults to 'openshift-4.21'.",
						},
						"days": {
							Type:        "number",
							Description: "Number of days to look back (default: 7)",
						},
						"status": {
							Type:        "array",
							Description: "Filter by build status (SUCCESS, FAILURE, ABORTED, etc.)",
						},
						"jobProject": {
							Type:        "string",
							Description: "Jenkins job project (ocp4-konflux, prepare-release-konflux). Claude selects based on user context. Defaults to 'ocp4-konflux'.",
						},
					},
				},
			},
			{
				Name:        "analyze_jenkins_logs",
				Description: "Retrieve and analyze Jenkins console logs for specific build numbers. Provides detailed failure pattern analysis and log excerpts.",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"buildNumber": {
							Type:        "number",
							Description: "Jenkins build number to analyze",
						},
						"componentName": {
							Type:        "string",
							Description: "Component name to find latest build for analysis (alternative to buildNumber)",
						},
						"assembly": {
							Type:        "string",
							Description: "Assembly type when using componentName (stream, test, standard, custom, preview). Defaults to 'stream'.",
						},
						"group": {
							Type:        "string",
							Description: "OpenShift version group when using componentName. Defaults to 'openshift-4.21'.",
						},
						"days": {
							Type:        "number",
							Description: "Days to look back when finding latest build (default: 7)",
						},
						"fullLog": {
							Type:        "boolean",
							Description: "Retrieve full console log instead of just last 50 lines (default: false)",
						},
						"jobProject": {
							Type:        "string",
							Description: "Jenkins job project (ocp4-konflux, prepare-release-konflux). Claude selects based on user context. Defaults to 'ocp4-konflux'.",
						},
					},
				},
			},
			{
				Name:        "correlate_jenkins_builds",
				Description: "Correlate Jenkins build data with BigQuery build records to provide comprehensive analysis linking Jenkins execution with Konflux build outcomes.",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"buildNumber": {
							Type:        "string",
							Description: "Jenkins build number for correlation",
						},
						"componentName": {
							Type:        "string",
							Description: "Component name to correlate builds for",
						},
						"assembly": {
							Type:        "string",
							Description: "Assembly type (stream, test, standard, custom, preview). Defaults to 'stream'.",
						},
						"group": {
							Type:        "string",
							Description: "OpenShift version group. Defaults to 'openshift-4.21'.",
						},
						"timeRange": {
							Type:        "number",
							Description: "Time range in hours for correlation matching (default: 24)",
						},
						"jobProject": {
							Type:        "string",
							Description: "Jenkins job project (ocp4-konflux, prepare-release-konflux). Claude selects based on user context. Defaults to 'ocp4-konflux'.",
						},
					},
				},
			},
			{
				Name:        "open_browser_links",
				Description: "Open multiple URLs in the default browser with safety validation. Supports Jenkins, Konflux, and GitHub links from build analysis.",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"urls": {
							Type:        "array",
							Description: "List of URLs to open in browser",
							Items: &shared.Items{
								Type: "string",
							},
						},
						"maxLinks": {
							Type:        "number",
							Description: "Maximum number of links to open (default: 10, max: 20)",
						},
					},
					Required: []string{"urls"},
				},
			},
		},
	}
}

// Close closes the server and its resources
func (s *Server) Close() error {
	if s.jenkinsClient != nil {
		return s.jenkinsClient.Close()
	}
	return nil
}

// queryJenkinsBuilds handles the query_jenkins_builds tool
func (s *Server) queryJenkinsBuilds(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	var query JenkinsJobQuery

	// Parse arguments
	if componentName, ok := args["componentName"].(string); ok {
		query.ComponentName = componentName
	}
	if assembly, ok := args["assembly"].(string); ok {
		query.Assembly = assembly
	} else {
		query.Assembly = "stream" // default
	}
	if group, ok := args["group"].(string); ok {
		query.Group = group
	} else {
		query.Group = "openshift-4.21" // default
	}
	if days, ok := args["days"].(float64); ok {
		query.Days = int(days)
	} else {
		query.Days = 7 // default
	}
	if statusList, ok := args["status"].([]interface{}); ok {
		for _, status := range statusList {
			if statusStr, ok := status.(string); ok {
				query.Status = append(query.Status, statusStr)
			}
		}
	}

	jobProject := "ocp4-konflux" // default
	if jp, ok := args["jobProject"].(string); ok && jp != "" {
		jobProject = jp
	}

	// Query Jenkins for builds
	jobs, err := s.jenkinsClient.QueryKonfluxBuilds(ctx, query.ComponentName, query.Assembly, query.Group, query.Days, jobProject)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to query Jenkins builds: %w", err)
	}

	// Filter by status if specified
	if len(query.Status) > 0 {
		filteredJobs := make([]jenkins.KonfluxJob, 0)
		for _, job := range jobs {
			for _, status := range query.Status {
				if strings.EqualFold(job.Status, status) {
					filteredJobs = append(filteredJobs, job)
					break
				}
			}
		}
		jobs = filteredJobs
	}

	// Summarize results
	summary := s.summarizeJenkinsBuilds(jobs, query)

	return shared.FormatJSONResult(summary)
}

// analyzeJenkinsLogs handles the analyze_jenkins_logs tool
func (s *Server) analyzeJenkinsLogs(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	var buildNumber int
	var found bool

	jobProject := "ocp4-konflux" // default
	if jp, ok := args["jobProject"].(string); ok && jp != "" {
		jobProject = jp
	}

	// Check if build number is provided directly
	if bn, ok := args["buildNumber"].(float64); ok {
		buildNumber = int(bn)
		found = true
	} else if componentName, ok := args["componentName"].(string); ok {
		// Find latest build for component
		assembly := "stream"
		if a, ok := args["assembly"].(string); ok {
			assembly = a
		}
		group := "openshift-4.21"
		if g, ok := args["group"].(string); ok {
			group = g
		}
		days := 7
		if d, ok := args["days"].(float64); ok {
			days = int(d)
		}

		jobs, err := s.jenkinsClient.QueryKonfluxBuilds(ctx, componentName, assembly, group, days, jobProject)
		if err != nil {
			return shared.ToolResult{}, fmt.Errorf("failed to find builds for component %s: %w", componentName, err)
		}

		if len(jobs) > 0 {
			buildNumber = jobs[0].BuildNumber // Most recent
			found = true
		}
	}

	if !found {
		return shared.ToolResult{}, fmt.Errorf("either buildNumber or componentName is required")
	}

	// Check fullLog parameter
	fullLog := false
	if fl, ok := args["fullLog"].(bool); ok {
		fullLog = fl
	}

	// Get build details
	buildDetails, err := s.jenkinsClient.GetBuildDetails(ctx, buildNumber, jobProject)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to get build details for %d: %w", buildNumber, err)
	}

	// Retrieve console logs
	logs, err := s.jenkinsClient.GetJenkinsLogs(ctx, buildNumber, jobProject)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to retrieve logs for build %d: %w", buildNumber, err)
	}

	// ALWAYS analyze the FULL log for failure patterns and extract links
	errorSummary := s.jenkinsClient.AnalyzeJenkinsLogs(logs)

	// ALWAYS parse failed components and Konflux links from FULL log (regardless of fullLog parameter)
	var failedComponents []ComponentFailure
	var konfluxLinks []KonfluxLink

	failedComponents = s.ParseFailedComponents(logs)
	konfluxLinks = s.ParseKonfluxLinks(logs)

	// Extract log excerpts - ALWAYS return manageable size regardless of fullLog parameter
	logLines := strings.Split(logs, "\n")
	var excerpts []string

	if fullLog {
		// When fullLog=true, we analyzed the full log but return smart excerpts to prevent token overflow
		excerpts = append(excerpts, "=== FULL LOG ANALYZED - SMART EXCERPT TO PREVENT TOKEN OVERFLOW ===")
		excerpts = append(excerpts, fmt.Sprintf("Analyzed full log: %d lines, %d chars", len(logLines), len(logs)))
		excerpts = append(excerpts, fmt.Sprintf("Found %d Konflux links", len(konfluxLinks)))

		if len(konfluxLinks) > 0 {
			excerpts = append(excerpts, "=== EXTRACTED KONFLUX LINKS ===")
			for i, link := range konfluxLinks {
				excerpts = append(excerpts, fmt.Sprintf("Link %d: Component='%s', URL='%s'", i+1, link.Component, link.URL))
			}
		}

		// Always include just last 50 lines as context (same as default behavior)
		excerpts = append(excerpts, "=== LAST 50 LINES ===")
		if len(logLines) > 50 {
			excerpts = append(excerpts, logLines[len(logLines)-50:]...)
		} else {
			excerpts = append(excerpts, logLines...)
		}
	} else {
		// Default behavior: analyze full log internally, return last 50 lines
		if len(logLines) > 50 {
			excerpts = append(excerpts, "=== LAST 50 LINES ===")
			excerpts = append(excerpts, logLines[len(logLines)-50:]...)
		} else {
			excerpts = logLines
		}
	}

	// Clean up: konfluxLinks should now be populated directly

	// Create analysis result
	analysis := JenkinsLogAnalysis{
		BuildNumber:      buildDetails.BuildNumber,
		JobName:          buildDetails.Name,
		Status:           buildDetails.Status,
		Timestamp:        buildDetails.Timestamp,
		Duration:         buildDetails.Duration,
		LogURL:           buildDetails.LogURL,
		Component:        buildDetails.Component,
		Assembly:         buildDetails.Assembly,
		Group:            buildDetails.Group,
		ErrorSummary:     errorSummary,
		LogExcerpts:      excerpts,
		Parameters:       buildDetails.Parameters,
		FailedComponents: failedComponents,
		KonfluxLinks:     konfluxLinks,
		Analysis: map[string]interface{}{
			"logSize":              len(logs),
			"logLines":             len(logLines),
			"hasErrors":            len(errorSummary) > 0,
			"buildDuration":        buildDetails.Duration.String(),
			"isRecentBuild":        time.Since(buildDetails.Timestamp) < 24*time.Hour,
			"fullLogRetrieved":     fullLog,
			"failedComponentCount": len(failedComponents),
			"konfluxLinksFound":    len(konfluxLinks),
			"tokenLimitPrevention": fullLog && len(logs) > 100000,
		},
	}

	return shared.FormatJSONResult(analysis)
}

// correlateJenkinsBuilds handles the correlate_jenkins_builds tool
func (s *Server) correlateJenkinsBuilds(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	var buildNumber int
	var componentName string

	if bn, ok := args["buildNumber"].(string); ok {
		if parsed, err := fmt.Sscanf(bn, "%d", &buildNumber); err != nil || parsed != 1 {
			return shared.ToolResult{}, fmt.Errorf("invalid buildNumber format: %s", bn)
		}
	}
	if cn, ok := args["componentName"].(string); ok {
		componentName = cn
	}

	if buildNumber == 0 && componentName == "" {
		return shared.ToolResult{}, fmt.Errorf("either buildNumber or componentName is required")
	}

	assembly := "stream"
	if a, ok := args["assembly"].(string); ok {
		assembly = a
	}
	group := "openshift-4.21"
	if g, ok := args["group"].(string); ok {
		group = g
	}
	timeRange := 24
	if tr, ok := args["timeRange"].(float64); ok {
		timeRange = int(tr)
	}

	jobProject := "ocp4-konflux" // default
	if jp, ok := args["jobProject"].(string); ok && jp != "" {
		jobProject = jp
	}

	var jenkinsJob *jenkins.KonfluxJob
	var err error

	// Get Jenkins job details
	if buildNumber > 0 {
		jenkinsJob, err = s.jenkinsClient.GetBuildDetails(ctx, buildNumber, jobProject)
		if err != nil {
			return shared.ToolResult{}, fmt.Errorf("failed to get Jenkins build %d: %w", buildNumber, err)
		}
	} else {
		// Find latest build for component
		jobs, err := s.jenkinsClient.QueryKonfluxBuilds(ctx, componentName, assembly, group, 7, jobProject)
		if err != nil {
			return shared.ToolResult{}, fmt.Errorf("failed to find Jenkins builds for %s: %w", componentName, err)
		}
		if len(jobs) == 0 {
			return shared.ToolResult{}, fmt.Errorf("no Jenkins builds found for component %s", componentName)
		}
		jenkinsJob = &jobs[0]
	}

	// Create correlation analysis
	correlation := JenkinsCorrelation{
		JenkinsJob:   *jenkinsJob,
		Confidence:   0.8, // Base confidence
		MatchFactors: []string{},
		Analysis: map[string]interface{}{
			"jenkinsJobFound":   true,
			"correlationMethod": "timestamp-component-assembly",
			"timeRangeHours":    timeRange,
			"searchCriteria": map[string]string{
				"component": jenkinsJob.Component,
				"assembly":  jenkinsJob.Assembly,
				"group":     jenkinsJob.Group,
			},
		},
	}

	// Add match factors
	if jenkinsJob.Component != "" {
		correlation.MatchFactors = append(correlation.MatchFactors, "component-match")
		correlation.Confidence += 0.1
	}
	if jenkinsJob.Assembly != "" && jenkinsJob.Assembly != "stream" {
		correlation.MatchFactors = append(correlation.MatchFactors, "assembly-match")
		correlation.Confidence += 0.05
	}

	correlation.Analysis["correlationNote"] = "This provides Jenkins execution context. Use mcp__build-analyzer__analyze_build_logs with component details for BigQuery correlation."

	return shared.FormatJSONResult(correlation)
}

// Helper functions

// summarizeJenkinsBuilds creates a summary of Jenkins build query results
func (s *Server) summarizeJenkinsBuilds(jobs []jenkins.KonfluxJob, query JenkinsJobQuery) map[string]interface{} {
	statusBreakdown := make(map[string]int)
	componentBreakdown := make(map[string]int)
	assemblyBreakdown := make(map[string]int)

	for _, job := range jobs {
		statusBreakdown[job.Status]++
		if job.Component != "" {
			componentBreakdown[job.Component]++
		}
		assemblyBreakdown[job.Assembly]++
	}

	recent := jobs
	if len(jobs) > 10 {
		recent = jobs[:10]
	}

	return map[string]interface{}{
		"query": map[string]interface{}{
			"componentName": query.ComponentName,
			"assembly":      query.Assembly,
			"group":         query.Group,
			"days":          query.Days,
			"statusFilter":  query.Status,
		},
		"summary": map[string]interface{}{
			"totalBuilds":        len(jobs),
			"statusBreakdown":    statusBreakdown,
			"componentBreakdown": componentBreakdown,
			"assemblyBreakdown":  assemblyBreakdown,
		},
		"recentBuilds": recent,
		"analysis": map[string]interface{}{
			"searchPerformed": true,
			"jenkinsURL":      "https://art-jenkins.apps.prod-stable-spoke1-dc-iad2.itup.redhat.com/job/aos-cd-builds/job/build%252Focp4-konflux/",
			"nextSteps":       "Use analyze_jenkins_logs to examine specific build failures, or correlate_jenkins_builds to link with BigQuery data",
		},
	}
}

// ParseFailedComponents extracts failed component information from Jenkins logs
func (s *Server) ParseFailedComponents(logs string) []ComponentFailure {
	var failures []ComponentFailure

	// Regex patterns for common failure scenarios
	patterns := []*regexp.Regexp{
		// PipelineRun failures with status=False and reason=Failed
		regexp.MustCompile(`PipelineRun\s+(ose-4-\d+-[^-]+(?:-[^-]+)*)-[a-z0-9]+\s+.*?\[status=False\]\[reason=(Failed|failed)\]`),
		// Pod failures with phase=Failed
		regexp.MustCompile(`Pod\s+(ose-4-\d+-[^-]+(?:-[^-]+)*)-[a-z0-9]+-[^-]+-pod\s+.*?\[phase=(Failed|failed)\]`),
		// General component failure patterns
		regexp.MustCompile(`(?i)(ose-[^-]+(?:-[^-]+)*)\s+.*?\[.*?(failed|error|timeout)\]`),
		// Component build failures
		regexp.MustCompile(`(?i)(\w+[-\w]*)\s+build\s+(failed|error|timeout)`),
	}

	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		for _, pattern := range patterns {
			matches := pattern.FindStringSubmatch(line)
			if len(matches) >= 3 {
				componentName := strings.TrimSpace(matches[1])
				errorType := strings.ToLower(strings.TrimSpace(matches[2]))

				// Extract error message (use full line as context)
				errorMessage := strings.TrimSpace(line)
				if len(matches) > 3 {
					errorMessage = strings.TrimSpace(matches[3])
				}

				// Skip common false positives
				if s.isValidComponentName(componentName) {
					failure := ComponentFailure{
						Name:         componentName,
						ErrorType:    errorType,
						ErrorMessage: errorMessage,
						KonfluxURL:   s.findKonfluxURLForComponent(logs, componentName),
					}
					failures = append(failures, failure)
				}
			}
		}
	}

	// Deduplicate failures
	return s.deduplicateFailures(failures)
}

// ParseKonfluxLinks extracts Konflux build URLs from Jenkins logs
func (s *Server) ParseKonfluxLinks(logs string) []KonfluxLink {
	// Use EXACT same logic as debugURLExtraction which I KNOW works
	allURLPattern := regexp.MustCompile(`(https://[^\s\]]+)`)
	allMatches := allURLPattern.FindAllString(logs, -1)

	var konfluxLinks []KonfluxLink
	lines := strings.Split(logs, "\n")

	for _, url := range allMatches {
		if strings.Contains(url, "konflux") {
			// Find the line containing this URL for context
			var component string
			var context string

			for i, line := range lines {
				if strings.Contains(line, url) {
					component = s.extractComponentFromContext(lines, i)
					context = strings.TrimSpace(line)
					break
				}
			}

			link := KonfluxLink{
				Component: component,
				URL:       url,
				Context:   context,
			}
			konfluxLinks = append(konfluxLinks, link)
		}
	}

	return s.deduplicateLinksLatestPerComponent(konfluxLinks)
}

// findKonfluxURLForComponent searches for Konflux URL associated with a component
func (s *Server) findKonfluxURLForComponent(logs, componentName string) string {
	lines := strings.Split(logs, "\n")

	// Look for URLs within 5 lines of component mention
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(componentName)) {
			// Search surrounding lines for URLs
			start := max(0, i-5)
			end := min(len(lines), i+5)

			urlPattern := regexp.MustCompile(`(https://console\.redhat\.com/[^\s]+|https://[^.\s]+\.konflux\.[^\s]+)`)
			for j := start; j < end; j++ {
				if match := urlPattern.FindString(lines[j]); match != "" {
					return match
				}
			}
		}
	}

	return ""
}

// isValidComponentName checks if a string is likely a valid component name
func (s *Server) isValidComponentName(name string) bool {
	// Skip common false positives
	invalidNames := []string{
		"error", "failed", "success", "build", "test", "run", "job",
		"pipeline", "stage", "step", "time", "timeout", "abort", "warning",
		"info", "debug", "trace", "log", "output", "result", "status",
	}

	lowerName := strings.ToLower(name)
	for _, invalid := range invalidNames {
		if lowerName == invalid {
			return false
		}
	}

	// Must be reasonable length and format (accept ose- prefixed components)
	return len(name) > 2 && len(name) < 100 && (strings.Contains(name, "-") || strings.HasPrefix(name, "ose"))
}

// extractComponentFromContext extracts component name from surrounding log context
func (s *Server) extractComponentFromContext(lines []string, lineIndex int) string {
	currentLine := lines[lineIndex]

	// First, try to extract from "Created PipelineRun:" pattern in current line
	if strings.Contains(currentLine, "Created PipelineRun:") {
		// Extract from PipelineRun name pattern: ose-4-19-ironic-nkqsq -> ironic
		pipelinePattern := regexp.MustCompile(`ose-4-\d+-([^-\s]+)(?:-[a-z0-9]+)?`)
		if match := pipelinePattern.FindStringSubmatch(currentLine); len(match) > 1 {
			return match[1]
		}
	}

	// Also try to extract from URL path if present in current line
	urlPathPattern := regexp.MustCompile(`/pipelineruns/ose-4-\d+-([^-\s]+)(?:-[a-z0-9]+)?`)
	if match := urlPathPattern.FindStringSubmatch(currentLine); len(match) > 1 {
		return match[1]
	}

	// Look for component names in surrounding lines
	start := max(0, lineIndex-3)
	end := min(len(lines), lineIndex+3)

	// Look for explicit component mentions
	componentPattern := regexp.MustCompile(`(?i)component[:\s]+(\w+[-\w]*)`)
	for i := start; i < end; i++ {
		if match := componentPattern.FindStringSubmatch(lines[i]); len(match) > 1 {
			return match[1]
		}
	}

	// Look for konflux_image_builder.[component] pattern
	builderPattern := regexp.MustCompile(`konflux_image_builder\.\[([^\]]+)\]`)
	for i := start; i < end; i++ {
		if match := builderPattern.FindStringSubmatch(lines[i]); len(match) > 1 {
			return match[1]
		}
	}

	// Look for image names or other identifiers
	imagePattern := regexp.MustCompile(`(?i)(?:image|ose|openshift)[-_]([a-z0-9-]+)`)
	for i := start; i < end; i++ {
		if match := imagePattern.FindStringSubmatch(lines[i]); len(match) > 1 {
			return match[1]
		}
	}

	return ""
}

// deduplicateFailures removes duplicate component failures
func (s *Server) deduplicateFailures(failures []ComponentFailure) []ComponentFailure {
	seen := make(map[string]bool)
	var result []ComponentFailure

	for _, failure := range failures {
		key := failure.Name + "|" + failure.ErrorType
		if !seen[key] {
			seen[key] = true
			result = append(result, failure)
		}
	}

	return result
}

// deduplicateLinksLatestPerComponent groups by component and keeps only latest build per component
func (s *Server) deduplicateLinksLatestPerComponent(links []KonfluxLink) []KonfluxLink {
	if len(links) == 0 {
		return links
	}

	// Group links by component name
	componentGroups := make(map[string][]KonfluxLink)
	for _, link := range links {
		component := link.Component
		if component == "" {
			component = "unknown"
		}
		componentGroups[component] = append(componentGroups[component], link)
	}

	var result []KonfluxLink

	// For each component, keep only the latest build
	for _, componentLinks := range componentGroups {
		if len(componentLinks) == 1 {
			result = append(result, componentLinks[0])
		} else {
			// Keep the last one found (assuming log order is chronological)
			latestLink := componentLinks[len(componentLinks)-1]
			result = append(result, latestLink)
		}
	}

	return result
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// openBrowserLinks handles the open_browser_links tool
func (s *Server) openBrowserLinks(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	// Parse URLs from arguments
	urlsInterface, ok := args["urls"]
	if !ok {
		return shared.ToolResult{}, fmt.Errorf("urls parameter is required")
	}

	urlsArray, ok := urlsInterface.([]interface{})
	if !ok {
		return shared.ToolResult{}, fmt.Errorf("urls must be an array")
	}

	var urls []string
	for _, urlInterface := range urlsArray {
		if urlStr, ok := urlInterface.(string); ok {
			urls = append(urls, urlStr)
		} else {
			return shared.ToolResult{}, fmt.Errorf("all URLs must be strings")
		}
	}

	if len(urls) == 0 {
		return shared.ToolResult{}, fmt.Errorf("at least one URL is required")
	}

	// Parse maxLinks parameter
	maxLinks := 10 // default
	if maxLinksFloat, ok := args["maxLinks"].(float64); ok {
		maxLinks = int(maxLinksFloat)
		if maxLinks > 20 {
			maxLinks = 20 // enforce maximum
		}
		if maxLinks < 1 {
			maxLinks = 1
		}
	}

	// Create browser configuration with safe defaults
	config := shared.DefaultBrowserConfig()
	config.MaxLinksPerOp = maxLinks

	// Open browser links
	response := shared.OpenBrowserLinks(urls, config)

	// Convert response to JSON
	resultJSON, err := json.Marshal(response)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to marshal response: %v", err)
	}

	return shared.ToolResult{
		Content: []shared.Content{
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	}, nil
}
