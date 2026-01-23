package buildanalyzer

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/lgarciaaco/claude-build-analyzer/internal/bigquery"
	"github.com/lgarciaaco/claude-build-analyzer/pkg/shared"
)

// Server implements the Build Analyzer MCP server for BigQuery-based build failure analysis
type Server struct {
	*shared.BaseMCPServer
	bqClient *bigquery.Client
	ctx      context.Context
}

// Type aliases for BigQuery types from shared package
type BuildRecord = shared.BuildRecord
type BuildFailureQuery = shared.BuildFailureQuery
type BuildComparison = shared.BuildComparison
type TaskRunRecord = shared.TaskRunRecord
type ContainerInfo = shared.ContainerInfo
type BuildLogAnalysis = shared.BuildLogAnalysis

// NewServer creates a new Build Analyzer MCP server
func NewServer(ctx context.Context) (*Server, error) {
	// Initialize BigQuery client
	bqClient, err := bigquery.NewClient(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %w", err)
	}

	server := &Server{
		bqClient: bqClient,
		ctx:      ctx,
	}

	// Create base server with tool list function
	server.BaseMCPServer = shared.NewBaseMCPServer("build-analyzer", "1.0.0", ctx, server.GetToolList)

	// Register BigQuery-focused tools
	server.RegisterTool("query_build_failures", server.queryBuildFailures)
	server.RegisterTool("analyze_build_logs", server.analyzeBuildLogs)
	server.RegisterTool("compare_builds", server.compareBuilds)

	return server, nil
}

// Initialize initializes the Build Analyzer server
func (s *Server) Initialize() error {
	log.Println("Initializing Build Analyzer Server...")
	// BigQuery client is already initialized in NewServer
	log.Println("Build Analyzer Server initialized successfully")
	return nil
}

// GetToolList returns the list of available BigQuery tools
func (s *Server) GetToolList() shared.ToolList {
	return shared.ToolList{
		Tools: []shared.Tool{
			{
				Name:        "query_build_failures",
				Description: "Query recent build failures for specified components. Returns detailed build information including failure reasons, architectures, and build URLs.",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"componentNames": {
							Type:        "array",
							Description: "List of component names to query (e.g., [\"etcd-operator\", \"oauth-server\"])",
						},
						"group": {
							Type:        "string",
							Description: "OpenShift version group (e.g., \"openshift-4.21\"). Defaults to latest if not specified.",
						},
						"assembly": {
							Type:        "string",
							Description: "Assembly type (stream, test, standard, custom, preview). Defaults to 'stream' for continuous builds.",
						},
						"days": {
							Type:        "number",
							Description: "Number of days to look back (default: 7)",
						},
						"architectures": {
							Type:        "array",
							Description: "Filter by specific architectures (x86_64, aarch64, ppc64le, s390x)",
						},
						"hermetic": {
							Type:        "boolean",
							Description: "Filter by hermetic build mode (true/false)",
						},
					},
					Required: []string{"componentNames"},
				},
			},
			{
				Name:        "analyze_build_logs",
				Description: "Get build log URLs and analyze build failures for a specific component or build ID. Provides links to Konflux builds and Jenkins jobs.",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"componentName": {
							Type:        "string",
							Description: "Component name to analyze",
						},
						"buildId": {
							Type:        "string",
							Description: "Specific build ID to analyze (optional)",
						},
						"group": {
							Type:        "string",
							Description: "OpenShift version group (default: openshift-4.21)",
						},
						"assembly": {
							Type:        "string",
							Description: "Assembly type (stream, test, standard, custom, preview). Defaults to 'stream' for continuous builds.",
						},
					},
				},
			},
			{
				Name:        "compare_builds",
				Description: "Compare two builds to identify differences in configuration, outcome, or environment that might explain failures.",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"buildId1": {
							Type:        "string",
							Description: "First build ID to compare",
						},
						"buildId2": {
							Type:        "string",
							Description: "Second build ID to compare",
						},
					},
					Required: []string{"buildId1", "buildId2"},
				},
			},
		},
	}
}

// Close closes the server and its resources
func (s *Server) Close() error {
	if s.bqClient != nil {
		return s.bqClient.Close()
	}
	return nil
}

// queryBuildFailures handles the query_build_failures tool
func (s *Server) queryBuildFailures(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	var query BuildFailureQuery

	// Parse component names
	if componentNames, ok := args["componentNames"].([]interface{}); ok {
		for _, name := range componentNames {
			if nameStr, ok := name.(string); ok {
				query.ComponentNames = append(query.ComponentNames, nameStr)
			}
		}
	}

	// Parse other arguments
	if group, ok := args["group"].(string); ok {
		query.Group = group
	}
	if assembly, ok := args["assembly"].(string); ok {
		query.Assembly = assembly
	}
	if days, ok := args["days"].(float64); ok {
		query.Days = int(days)
	}
	if architectures, ok := args["architectures"].([]interface{}); ok {
		for _, arch := range architectures {
			if archStr, ok := arch.(string); ok {
				query.Architectures = append(query.Architectures, archStr)
			}
		}
	}
	if hermetic, ok := args["hermetic"].(bool); ok {
		query.Hermetic = &hermetic
	}

	// Query BigQuery directly using shared types
	builds, err := s.bqClient.QueryBuildFailures(s.ctx, query)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to query build failures: %w", err)
	}

	// Summarize the results
	result := s.summarizeBuildFailures(builds)

	return shared.FormatJSONResult(result)
}

// analyzeBuildLogs handles the analyze_build_logs tool
func (s *Server) analyzeBuildLogs(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	componentName, hasComponent := args["componentName"].(string)
	buildID, hasBuildID := args["buildId"].(string)
	group := "openshift-4.21"
	if g, ok := args["group"].(string); ok {
		group = g
	}
	assembly := "stream"
	if a, ok := args["assembly"].(string); ok {
		assembly = a
	}

	var build *BuildRecord
	var err error

	if hasBuildID {
		comparison, err := s.bqClient.CompareBuilds(s.ctx, buildID, buildID)
		if err != nil {
			return shared.ToolResult{}, fmt.Errorf("failed to retrieve build %s: %w", buildID, err)
		}
		build = comparison.Build1
	} else if hasComponent {
		build, err = s.bqClient.GetLatestBuildForComponent(s.ctx, componentName, group, assembly)
		if err != nil {
			return shared.ToolResult{}, fmt.Errorf("failed to get latest build for component %s: %w", componentName, err)
		}
	} else {
		return shared.ToolResult{}, fmt.Errorf("either componentName or buildId is required")
	}

	if build == nil {
		result := map[string]interface{}{
			"error": fmt.Sprintf("No build found for %s", componentName),
		}
		return shared.FormatJSONResult(result)
	}

	// Get TaskRun records with container logs
	var taskRuns []TaskRunRecord
	var failedContainers []ContainerInfo
	var errorSummary []string

	taskRuns, err = s.bqClient.GetTaskRunsForBuild(s.ctx, build.BuildID)
	if err != nil {
		// If TaskRun query fails, still return basic build info but note the failure
		log.Printf("Failed to retrieve TaskRun records for build %s: %v", build.BuildID, err)
		errorSummary = append(errorSummary, fmt.Sprintf("Failed to retrieve container logs: %v", err))
	} else {
		// Extract failed container logs
		failedContainers = s.bqClient.ExtractFailedContainerLogs(taskRuns)
		
		// Analyze container logs for error patterns
		if len(failedContainers) > 0 {
			errorSummary = s.bqClient.AnalyzeContainerLogs(failedContainers)
		}
	}

	// Create comprehensive build log analysis
	analysis := BuildLogAnalysis{
		BuildInfo: *build,
		LogUrls: map[string]string{
			"konfluxBuild": build.BuildPipelineURL,
			"artJob":       build.ArtJobURL,
		},
		Architecture:     build.Arches,
		TaskRuns:         taskRuns,
		FailedContainers: failedContainers,
		ErrorSummary:     errorSummary,
		PossibleIssues:   s.analyzePossibleIssues(build),
	}

	// If we have container logs, enhance the possible issues
	if len(failedContainers) > 0 {
		analysis.PossibleIssues = append(analysis.PossibleIssues, s.analyzeContainerIssues(failedContainers)...)
	}

	return shared.FormatJSONResult(analysis)
}

// compareBuilds handles the compare_builds tool
func (s *Server) compareBuilds(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	buildID1, ok1 := args["buildId1"].(string)
	buildID2, ok2 := args["buildId2"].(string)

	if !ok1 || !ok2 {
		return shared.ToolResult{}, fmt.Errorf("both buildId1 and buildId2 are required")
	}

	comparison, err := s.bqClient.CompareBuilds(s.ctx, buildID1, buildID2)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to compare builds %s and %s: %w", buildID1, buildID2, err)
	}

	return shared.FormatJSONResult(comparison)
}

// Helper functions

// summarizeBuildFailures creates a summary of build failures
func (s *Server) summarizeBuildFailures(builds []BuildRecord) map[string]interface{} {
	componentBreakdown := make(map[string]int)
	archBreakdown := make(map[string]int)
	hermeticCount := 0

	for _, build := range builds {
		componentBreakdown[build.Name]++
		for _, arch := range build.Arches {
			archBreakdown[arch]++
		}
		if build.Hermetic {
			hermeticCount++
		}
	}

	recent := builds
	if len(builds) > 10 {
		recent = builds[:10]
	}

	return map[string]interface{}{
		"totalFailures":         len(builds),
		"componentBreakdown":    componentBreakdown,
		"architectureBreakdown": archBreakdown,
		"hermeticBreakdown": map[string]int{
			"hermetic":    hermeticCount,
			"nonHermetic": len(builds) - hermeticCount,
		},
		"recentFailures": recent,
	}
}

// analyzePossibleIssues analyzes a build for possible issues
func (s *Server) analyzePossibleIssues(build *BuildRecord) []string {
	var issues []string

	if build.Outcome == "FAILURE" {
		if build.Hermetic {
			issues = append(issues, "Hermetic build failure - check for network access issues or missing dependencies")
		} else {
			issues = append(issues, "Network-open build failure - check for external dependency issues")
		}

		if len(build.Arches) > 1 {
			issues = append(issues, "Multi-architecture build - check if failure is architecture-specific")
		}

		if build.ArtJobURL == "" && build.BuildPipelineURL == "" {
			issues = append(issues, "No build logs available - build may have failed before execution")
		}
	}

	return issues
}

// analyzeContainerIssues analyzes failed containers for specific issues
func (s *Server) analyzeContainerIssues(containers []ContainerInfo) []string {
	var issues []string

	for _, container := range containers {
		if container.ExitCode.Valid && container.ExitCode.Int64 != 0 {
			issues = append(issues, fmt.Sprintf("Container %s failed with exit code %d", container.Name, container.ExitCode.Int64))
		}

		if container.State == "terminated" && container.Reason != "Completed" {
			issues = append(issues, fmt.Sprintf("Container %s terminated unexpectedly: %s", container.Name, container.Reason))
		}

		// Check for specific error patterns in logs
		logOutput := container.GetLogOutput()
		if logOutput == "" {
			continue
		}
		logContent := strings.ToLower(logOutput)
		if strings.Contains(logContent, "permission denied") {
			issues = append(issues, fmt.Sprintf("Permission issues detected in %s", container.Name))
		}
		if strings.Contains(logContent, "no space left") {
			issues = append(issues, fmt.Sprintf("Disk space issues detected in %s", container.Name))
		}
		if strings.Contains(logContent, "out of memory") || strings.Contains(logContent, "oom") {
			issues = append(issues, fmt.Sprintf("Memory issues detected in %s", container.Name))
		}
		if strings.Contains(logContent, "timeout") {
			issues = append(issues, fmt.Sprintf("Timeout issues detected in %s", container.Name))
		}
	}

	return issues
}
