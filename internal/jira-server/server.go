package jiraserver

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/lgarciaaco/claude-build-analyzer/internal/jira"
	"github.com/lgarciaaco/claude-build-analyzer/pkg/shared"
)

// Server implements the JIRA MCP server for issue analysis
type Server struct {
	*shared.BaseMCPServer
	jiraClient *jira.Client
	ctx        context.Context
}

// JiraIssueQuery represents parameters for querying JIRA issues
type JiraIssueQuery struct {
	IssueKey   string            `json:"issueKey,omitempty"`
	JQL        string            `json:"jql,omitempty"`
	CVE        string            `json:"cve,omitempty"`
	Component  string            `json:"component,omitempty"`
	Filters    map[string]string `json:"filters,omitempty"`
	MaxResults int               `json:"maxResults,omitempty"`
	StartAt    int               `json:"startAt,omitempty"`
}

// JiraIssueDetails represents detailed JIRA issue information
type JiraIssueDetails struct {
	Issue            *jira.Issue            `json:"issue"`
	SecurityAnalysis *jira.SecurityAnalysis `json:"securityAnalysis,omitempty"`
	RelatedCVEs      []string               `json:"relatedCVEs"`
	BuildImpact      *BuildImpactAnalysis   `json:"buildImpact,omitempty"`
	Recommendations  []string               `json:"recommendations"`
}

// BuildImpactAnalysis represents analysis of how the issue affects builds
type BuildImpactAnalysis struct {
	AffectedComponents []string `json:"affectedComponents"`
	AffectedArchs      []string `json:"affectedArchitectures"`
	BuildSystems       []string `json:"buildSystems"`
	ImpactLevel        string   `json:"impactLevel"`
	RequiresHermetic   bool     `json:"requiresHermetic"`
	ActionRequired     []string `json:"actionRequired"`
}

// JiraSearchResults represents JIRA search results with analysis
type JiraSearchResults struct {
	Issues      []jira.Issue        `json:"issues"`
	Total       int                 `json:"total"`
	Summary     *SearchSummary      `json:"summary"`
	CVEMappings map[string][]string `json:"cveMappings"`
	Patterns    *IssuePatterns      `json:"patterns"`
}

// SearchSummary provides high-level summary of search results
type SearchSummary struct {
	TotalIssues        int      `json:"totalIssues"`
	SecurityIssues     int      `json:"securityIssues"`
	BuildFailures      int      `json:"buildFailures"`
	OpenIssues         int      `json:"openIssues"`
	CriticalIssues     int      `json:"criticalIssues"`
	AffectedComponents []string `json:"affectedComponents"`
	UniqueCVEs         []string `json:"uniqueCVEs"`
}

// IssuePatterns identifies common patterns in issues
type IssuePatterns struct {
	CommonComponents []string `json:"commonComponents"`
	FrequentErrors   []string `json:"frequentErrors"`
	SecurityThemes   []string `json:"securityThemes"`
	BuildThemes      []string `json:"buildThemes"`
}

// JiraCommentsAnalysis represents analysis of issue comments
type JiraCommentsAnalysis struct {
	Comments       []jira.Comment `json:"comments"`
	UpdateSummary  *UpdateSummary `json:"updateSummary"`
	TechnicalNotes []string       `json:"technicalNotes"`
	StatusChanges  []string       `json:"statusChanges"`
	KeyFindings    []string       `json:"keyFindings"`
}

// UpdateSummary provides summary of issue updates
type UpdateSummary struct {
	LastUpdate        string   `json:"lastUpdate"`
	RecentActivity    bool     `json:"recentActivity"`
	StatusProgression []string `json:"statusProgression"`
	KeyContributors   []string `json:"keyContributors"`
}

// NewServer creates a new JIRA MCP server
func NewServer(ctx context.Context) (*Server, error) {
	jiraURL := "https://issues.redhat.com"

	// Check for JIRA authentication from environment
	var auth *jira.JiraAuth
	if token := os.Getenv("JIRA_TOKEN"); token != "" {
		auth = &jira.JiraAuth{
			Token: token,
		}
	}

	jiraClient := jira.NewClient(jiraURL, auth)

	server := &Server{
		jiraClient: jiraClient,
		ctx:        ctx,
	}

	// Create base server with tool list function
	server.BaseMCPServer = shared.NewBaseMCPServer("jira-server", "1.0.0", ctx, server.GetToolList)

	// Register JIRA-focused tools
	server.RegisterTool("get_issue", server.getIssue)
	server.RegisterTool("search_issues", server.searchIssues)
	server.RegisterTool("get_issue_comments", server.getIssueComments)
	server.RegisterTool("analyze_security_impact", server.analyzeSecurityImpact)

	return server, nil
}

// Initialize sets up the JIRA server
func (s *Server) Initialize() error {
	log.Println("JIRA MCP Server initialized successfully")
	return nil
}

// Close cleans up server resources
func (s *Server) Close() error {
	log.Println("JIRA MCP Server shutting down...")
	return nil
}

// GetToolList returns the list of available MCP tools
func (s *Server) GetToolList() shared.ToolList {
	return shared.ToolList{
		Tools: []shared.Tool{
			{
				Name:        "get_issue",
				Description: "Retrieve detailed information about a specific JIRA issue, including security analysis for OCPBUGS tickets",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"issueKey": {
							Type:        "string",
							Description: "JIRA issue key (e.g., OCPBUGS-12345)",
						},
						"includeSecurityAnalysis": {
							Type:        "boolean",
							Description: "Include security impact analysis (default: true)",
						},
						"includeBuildImpact": {
							Type:        "boolean",
							Description: "Include build impact analysis (default: true)",
						},
					},
					Required: []string{"issueKey"},
				},
			},
			{
				Name:        "search_issues",
				Description: "Search JIRA issues using JQL or predefined filters, with analysis of patterns and CVE mappings",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"jql": {
							Type:        "string",
							Description: "JQL query string for advanced search",
						},
						"cve": {
							Type:        "string",
							Description: "Search for issues related to a specific CVE",
						},
						"component": {
							Type:        "string",
							Description: "Search for issues affecting a specific OpenShift component",
						},
						"filters": {
							Type:        "object",
							Description: "Search filters (status, priority, labels, etc.)",
						},
						"maxResults": {
							Type:        "integer",
							Description: "Maximum number of results to return (default: 50)",
						},
						"startAt": {
							Type:        "integer",
							Description: "Starting offset for results (default: 0)",
						},
					},
				},
			},
			{
				Name:        "get_issue_comments",
				Description: "Retrieve and analyze comments from a JIRA issue for technical insights and status updates",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"issueKey": {
							Type:        "string",
							Description: "JIRA issue key to get comments for",
						},
						"maxResults": {
							Type:        "integer",
							Description: "Maximum number of comments to return (default: 50)",
						},
						"startAt": {
							Type:        "integer",
							Description: "Starting offset for comments (default: 0)",
						},
						"analyzeTechnical": {
							Type:        "boolean",
							Description: "Extract technical insights from comments (default: true)",
						},
					},
					Required: []string{"issueKey"},
				},
			},
			{
				Name:        "analyze_security_impact",
				Description: "Perform detailed security impact analysis for CVE-related issues, including component and architecture mapping",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"issueKey": {
							Type:        "string",
							Description: "JIRA issue key for security analysis",
						},
						"includeArchitectureImpact": {
							Type:        "boolean",
							Description: "Include architecture-specific impact analysis (default: true)",
						},
						"includeBuildRecommendations": {
							Type:        "boolean",
							Description: "Include build system recommendations (default: true)",
						},
					},
					Required: []string{"issueKey"},
				},
			},
		},
	}
}

// getIssue retrieves detailed issue information
func (s *Server) getIssue(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	// Parse arguments
	issueKey, ok := args["issueKey"].(string)
	if !ok || issueKey == "" {
		return shared.ToolResult{}, fmt.Errorf("issueKey is required")
	}

	includeSecurityAnalysis := true
	if val, ok := args["includeSecurityAnalysis"].(bool); ok {
		includeSecurityAnalysis = val
	}

	includeBuildImpact := true
	if val, ok := args["includeBuildImpact"].(bool); ok {
		includeBuildImpact = val
	}

	// Get the issue
	issue, err := s.jiraClient.GetIssue(issueKey)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to get issue %s: %w", issueKey, err)
	}

	details := &JiraIssueDetails{
		Issue:       issue,
		RelatedCVEs: s.extractCVEsFromIssue(issue),
	}

	// Add security analysis if requested
	if includeSecurityAnalysis {
		secAnalysis, err := s.jiraClient.AnalyzeSecurityImpact(issueKey)
		if err != nil {
			log.Printf("Warning: Failed to get security analysis for %s: %v", issueKey, err)
		} else {
			details.SecurityAnalysis = secAnalysis
		}
	}

	// Add build impact analysis if requested
	if includeBuildImpact {
		details.BuildImpact = s.analyzeBuildImpact(issue)
	}

	// Generate recommendations
	details.Recommendations = s.generateIssueRecommendations(issue, details.SecurityAnalysis, details.BuildImpact)

	return shared.FormatJSONResult(details)
}

// searchIssues searches for issues with analysis
func (s *Server) searchIssues(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	// Parse arguments
	var jql string
	var cve string
	var component string
	var filters map[string]string
	maxResults := 50
	startAt := 0

	if val, ok := args["jql"].(string); ok {
		jql = val
	}
	if val, ok := args["cve"].(string); ok {
		cve = val
	}
	if val, ok := args["component"].(string); ok {
		component = val
	}
	if val, ok := args["filters"].(map[string]interface{}); ok {
		filters = make(map[string]string)
		for k, v := range val {
			if str, ok := v.(string); ok {
				filters[k] = str
			}
		}
	}
	if val, ok := args["maxResults"].(float64); ok {
		maxResults = int(val)
	}
	if val, ok := args["startAt"].(float64); ok {
		startAt = int(val)
	}

	var searchResult *jira.SearchResult
	var err error

	// Execute search based on provided parameters
	if cve != "" {
		searchResult, err = s.jiraClient.SearchByCVE(cve, maxResults)
	} else if component != "" {
		searchResult, err = s.jiraClient.SearchByComponent(component, maxResults)
	} else if jql != "" {
		searchResult, err = s.jiraClient.SearchIssues(jql, maxResults, startAt)
	} else if filters != nil {
		searchResult, err = s.jiraClient.SearchOCPBUGSIssues(filters, maxResults)
	} else {
		return shared.ToolResult{}, fmt.Errorf("must provide one of: jql, cve, component, or filters")
	}

	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("search failed: %w", err)
	}

	// Analyze search results
	results := &JiraSearchResults{
		Issues:      searchResult.Issues,
		Total:       searchResult.Total,
		Summary:     s.generateSearchSummary(searchResult.Issues),
		CVEMappings: s.generateCVEMappings(searchResult.Issues),
		Patterns:    s.identifyIssuePatterns(searchResult.Issues),
	}

	return shared.FormatJSONResult(results)
}

// getIssueComments retrieves and analyzes issue comments
func (s *Server) getIssueComments(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	// Parse arguments
	issueKey, ok := args["issueKey"].(string)
	if !ok || issueKey == "" {
		return shared.ToolResult{}, fmt.Errorf("issueKey is required")
	}

	maxResults := 50
	if val, ok := args["maxResults"].(float64); ok {
		maxResults = int(val)
	}

	startAt := 0
	if val, ok := args["startAt"].(float64); ok {
		startAt = int(val)
	}

	analyzeTechnical := true
	if val, ok := args["analyzeTechnical"].(bool); ok {
		analyzeTechnical = val
	}

	// Get comments
	commentsResult, err := s.jiraClient.GetIssueComments(issueKey, maxResults, startAt)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to get comments for %s: %w", issueKey, err)
	}

	analysis := &JiraCommentsAnalysis{
		Comments:      commentsResult.Comments,
		UpdateSummary: s.generateUpdateSummary(commentsResult.Comments),
	}

	if analyzeTechnical {
		analysis.TechnicalNotes = s.extractTechnicalNotes(commentsResult.Comments)
		analysis.StatusChanges = s.extractStatusChanges(commentsResult.Comments)
		analysis.KeyFindings = s.extractKeyFindings(commentsResult.Comments)
	}

	return shared.FormatJSONResult(analysis)
}

// analyzeSecurityImpact performs detailed security analysis
func (s *Server) analyzeSecurityImpact(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	// Parse arguments
	issueKey, ok := args["issueKey"].(string)
	if !ok || issueKey == "" {
		return shared.ToolResult{}, fmt.Errorf("issueKey is required")
	}

	includeArchitectureImpact := true
	if val, ok := args["includeArchitectureImpact"].(bool); ok {
		includeArchitectureImpact = val
	}

	includeBuildRecommendations := true
	if val, ok := args["includeBuildRecommendations"].(bool); ok {
		includeBuildRecommendations = val
	}

	// Get security analysis
	analysis, err := s.jiraClient.AnalyzeSecurityImpact(issueKey)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to analyze security impact for %s: %w", issueKey, err)
	}

	// Enhance analysis with additional context if requested
	if includeArchitectureImpact {
		analysis = s.enhanceArchitectureAnalysis(analysis)
	}

	if includeBuildRecommendations {
		analysis = s.enhanceBuildRecommendations(analysis)
	}

	return shared.FormatJSONResult(analysis)
}

// Helper methods for analysis

func (s *Server) extractCVEsFromIssue(issue *jira.Issue) []string {
	text := fmt.Sprintf("%s %s %s", issue.Fields.Summary, issue.Fields.Description, issue.Fields.Environment)

	// Improved CVE extraction
	cveRegex := regexp.MustCompile(`CVE-\d{4}-\d{4,7}`)
	matches := cveRegex.FindAllString(text, -1)

	// Remove duplicates
	cveMap := make(map[string]bool)
	var cves []string
	for _, cve := range matches {
		if !cveMap[cve] {
			cveMap[cve] = true
			cves = append(cves, cve)
		}
	}

	return cves
}

func (s *Server) analyzeBuildImpact(issue *jira.Issue) *BuildImpactAnalysis {
	text := strings.ToLower(fmt.Sprintf("%s %s", issue.Fields.Summary, issue.Fields.Description))

	analysis := &BuildImpactAnalysis{
		AffectedComponents: s.extractComponentsFromText(text),
		AffectedArchs:      s.extractArchitecturesFromText(text),
		BuildSystems:       s.extractBuildSystemsFromText(text),
		ImpactLevel:        s.assessImpactLevel(issue),
		RequiresHermetic:   s.requiresHermeticBuild(text),
		ActionRequired:     s.generateActionItems(issue, text),
	}

	return analysis
}

func (s *Server) extractComponentsFromText(text string) []string {
	components := []string{
		"rhcos", "openshift4", "cri-o", "ironic", "oauth-server",
		"etcd", "kubernetes", "containers", "networking", "storage",
		"monitoring", "logging", "registry", "authentication", "authorization",
	}

	var found []string
	for _, component := range components {
		if strings.Contains(text, component) {
			found = append(found, component)
		}
	}

	return found
}

func (s *Server) extractArchitecturesFromText(text string) []string {
	archs := map[string]string{
		"x86_64":  "x86_64",
		"amd64":   "x86_64",
		"aarch64": "aarch64",
		"arm64":   "aarch64",
		"ppc64le": "ppc64le",
		"s390x":   "s390x",
	}

	var found []string
	foundMap := make(map[string]bool)

	for archVariant, canonical := range archs {
		if strings.Contains(text, archVariant) && !foundMap[canonical] {
			found = append(found, canonical)
			foundMap[canonical] = true
		}
	}

	return found
}

func (s *Server) extractBuildSystemsFromText(text string) []string {
	systems := []string{"konflux", "osbs", "brew", "jenkins", "tekton", "pipeline"}

	var found []string
	caser := cases.Title(language.Und)
	for _, system := range systems {
		if strings.Contains(text, system) {
			found = append(found, caser.String(system))
		}
	}

	return found
}

func (s *Server) assessImpactLevel(issue *jira.Issue) string {
	priority := strings.ToLower(issue.Fields.Priority.Name)

	switch priority {
	case "critical", "blocker":
		return "Critical"
	case "major":
		return "High"
	case "minor":
		return "Medium"
	default:
		return "Low"
	}
}

func (s *Server) requiresHermeticBuild(text string) bool {
	hermeticKeywords := []string{"hermetic", "network", "dependency", "cache", "offline"}

	for _, keyword := range hermeticKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}

	return false
}

func (s *Server) generateActionItems(issue *jira.Issue, text string) []string {
	var actions []string

	if strings.Contains(text, "build") || strings.Contains(text, "compile") {
		actions = append(actions, "Review build configuration and dependencies")
	}

	if strings.Contains(text, "security") || strings.Contains(text, "cve") {
		actions = append(actions, "Assess security impact across all architectures")
	}

	if strings.Contains(text, "hermetic") {
		actions = append(actions, "Verify hermetic build compatibility")
	}

	if issue.Fields.Priority.Name == "Critical" {
		actions = append(actions, "Prioritize immediate resolution")
	}

	return actions
}

func (s *Server) generateIssueRecommendations(issue *jira.Issue, secAnalysis *jira.SecurityAnalysis, buildImpact *BuildImpactAnalysis) []string {
	var recommendations []string

	// Basic recommendations based on issue type
	if secAnalysis != nil && len(secAnalysis.CVEs) > 0 {
		recommendations = append(recommendations,
			"Correlate with Jenkins build logs for failure patterns",
			"Check for related security issues in the same component family",
		)
	}

	if buildImpact != nil && len(buildImpact.AffectedComponents) > 1 {
		recommendations = append(recommendations,
			"Assess cross-component dependencies",
			"Review build order and timing issues",
		)
	}

	// Priority-based recommendations
	if issue.Fields.Priority.Name == "Critical" || issue.Fields.Priority.Name == "Blocker" {
		recommendations = append(recommendations,
			"Monitor build pipeline status closely",
			"Consider hotfix procedures if affecting production",
		)
	}

	return recommendations
}

func (s *Server) generateSearchSummary(issues []jira.Issue) *SearchSummary {
	summary := &SearchSummary{
		TotalIssues: len(issues),
	}

	componentMap := make(map[string]bool)
	cveMap := make(map[string]bool)

	for _, issue := range issues {
		// Count issue types
		text := strings.ToLower(fmt.Sprintf("%s %s", issue.Fields.Summary, issue.Fields.Description))

		if strings.Contains(text, "security") || strings.Contains(text, "cve") {
			summary.SecurityIssues++
		}

		if strings.Contains(text, "build") || strings.Contains(text, "compile") || strings.Contains(text, "failure") {
			summary.BuildFailures++
		}

		if issue.Fields.Status.Name != "Closed" && issue.Fields.Status.Name != "Resolved" {
			summary.OpenIssues++
		}

		if issue.Fields.Priority.Name == "Critical" || issue.Fields.Priority.Name == "Blocker" {
			summary.CriticalIssues++
		}

		// Collect components
		for _, comp := range issue.Fields.Components {
			if !componentMap[comp.Name] {
				componentMap[comp.Name] = true
				summary.AffectedComponents = append(summary.AffectedComponents, comp.Name)
			}
		}

		// Collect CVEs
		cves := s.extractCVEsFromIssue(&issue)
		for _, cve := range cves {
			if !cveMap[cve] {
				cveMap[cve] = true
				summary.UniqueCVEs = append(summary.UniqueCVEs, cve)
			}
		}
	}

	return summary
}

func (s *Server) generateCVEMappings(issues []jira.Issue) map[string][]string {
	mappings := make(map[string][]string)

	for _, issue := range issues {
		cves := s.extractCVEsFromIssue(&issue)
		for _, cve := range cves {
			mappings[cve] = append(mappings[cve], issue.Key)
		}
	}

	return mappings
}

func (s *Server) identifyIssuePatterns(issues []jira.Issue) *IssuePatterns {
	componentCounts := make(map[string]int)
	errorCounts := make(map[string]int)

	patterns := &IssuePatterns{}

	for _, issue := range issues {
		// Count components
		for _, comp := range issue.Fields.Components {
			componentCounts[comp.Name]++
		}

		// Extract error patterns from summaries
		summary := strings.ToLower(issue.Fields.Summary)
		if strings.Contains(summary, "error") {
			errorCounts[summary]++
		}
	}

	// Get top components
	for component, count := range componentCounts {
		if count >= 2 {
			patterns.CommonComponents = append(patterns.CommonComponents, component)
		}
	}

	// Security and build themes
	patterns.SecurityThemes = []string{"CVE remediation", "Security patches", "Vulnerability assessment"}
	patterns.BuildThemes = []string{"Hermetic conversion", "Dependency management", "Architecture compatibility"}

	return patterns
}

func (s *Server) generateUpdateSummary(comments []jira.Comment) *UpdateSummary {
	summary := &UpdateSummary{}

	if len(comments) > 0 {
		summary.LastUpdate = comments[len(comments)-1].Updated.Format("2006-01-02 15:04:05")
		summary.RecentActivity = len(comments) > 5 // Simple heuristic
	}

	// Extract contributors
	contributors := make(map[string]bool)
	for _, comment := range comments {
		if !contributors[comment.Author.DisplayName] {
			contributors[comment.Author.DisplayName] = true
			summary.KeyContributors = append(summary.KeyContributors, comment.Author.DisplayName)
		}
	}

	return summary
}

func (s *Server) extractTechnicalNotes(comments []jira.Comment) []string {
	var notes []string

	technicalKeywords := []string{"build", "compile", "error", "failure", "fix", "patch", "config", "dependency"}

	for _, comment := range comments {
		text := strings.ToLower(comment.Body)
		for _, keyword := range technicalKeywords {
			if strings.Contains(text, keyword) {
				notes = append(notes, fmt.Sprintf("%s: %s", comment.Author.DisplayName, comment.Body[:min(200, len(comment.Body))]))
				break
			}
		}
	}

	return notes
}

func (s *Server) extractStatusChanges(comments []jira.Comment) []string {
	var changes []string

	statusKeywords := []string{"status", "moved", "resolved", "closed", "reopened", "assigned"}

	for _, comment := range comments {
		text := strings.ToLower(comment.Body)
		for _, keyword := range statusKeywords {
			if strings.Contains(text, keyword) {
				changes = append(changes, fmt.Sprintf("%s: Status change noted", comment.Created.Format("2006-01-02")))
				break
			}
		}
	}

	return changes
}

func (s *Server) extractKeyFindings(comments []jira.Comment) []string {
	var findings []string

	findingKeywords := []string{"root cause", "solution", "workaround", "investigation", "analysis"}

	for _, comment := range comments {
		text := strings.ToLower(comment.Body)
		for _, keyword := range findingKeywords {
			if strings.Contains(text, keyword) {
				findings = append(findings, fmt.Sprintf("Key finding: %s", comment.Body[:min(300, len(comment.Body))]))
				break
			}
		}
	}

	return findings
}

func (s *Server) enhanceArchitectureAnalysis(analysis *jira.SecurityAnalysis) *jira.SecurityAnalysis {
	// Add architecture-specific recommendations
	if len(analysis.Architectures) == 0 {
		analysis.Architectures = []string{"x86_64", "aarch64", "ppc64le", "s390x"}
		analysis.Recommendations = append(analysis.Recommendations,
			"Verify impact across all supported architectures")
	}

	return analysis
}

func (s *Server) enhanceBuildRecommendations(analysis *jira.SecurityAnalysis) *jira.SecurityAnalysis {
	buildRecommendations := []string{
		"Review Konflux build pipeline configuration",
		"Check hermetic build compatibility",
		"Verify dependency management and caching",
		"Assess impact on multi-architecture builds",
	}

	analysis.Recommendations = append(analysis.Recommendations, buildRecommendations...)

	return analysis
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
