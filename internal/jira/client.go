package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client represents a JIRA REST API client
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// JiraAuth holds authentication credentials
type JiraAuth struct {
	Token string
}

// Issue represents a JIRA issue
type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

// IssueFields represents the fields of a JIRA issue
type IssueFields struct {
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Status      Status      `json:"status"`
	IssueType   IssueType   `json:"issuetype"`
	Priority    Priority    `json:"priority"`
	Components  []Component `json:"components"`
	Labels      []string    `json:"labels"`
	Created     string      `json:"created"`
	Updated     string      `json:"updated"`
	Assignee    *User       `json:"assignee"`
	Reporter    *User       `json:"reporter"`
	Resolution  *Resolution `json:"resolution"`
	FixVersions []Version   `json:"fixVersions"`
	Security    *Security   `json:"security"`
	Environment string      `json:"environment"`
}

// Status represents issue status
type Status struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// IssueType represents issue type
type IssueType struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Priority represents issue priority
type Priority struct {
	Name string `json:"name"`
}

// Component represents issue component
type Component struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// User represents a JIRA user
type User struct {
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

// Resolution represents issue resolution
type Resolution struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Version represents a fix version
type Version struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Released    bool   `json:"released"`
}

// Security represents security level
type Security struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Comment represents a JIRA comment
type Comment struct {
	ID      string    `json:"id"`
	Body    string    `json:"body"`
	Author  User      `json:"author"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// SearchResult represents JIRA search results
type SearchResult struct {
	Issues     []Issue `json:"issues"`
	Total      int     `json:"total"`
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
}

// CommentsResult represents comments on an issue
type CommentsResult struct {
	Comments   []Comment `json:"comments"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
}

// SecurityAnalysis represents security impact analysis
type SecurityAnalysis struct {
	CVEs            []string `json:"cves"`
	SecurityLevel   string   `json:"securityLevel"`
	Impact          string   `json:"impact"`
	Components      []string `json:"affectedComponents"`
	Architectures   []string `json:"affectedArchitectures"`
	BuildSystems    []string `json:"affectedBuildSystems"`
	Recommendations []string `json:"recommendations"`
}

// NewClient creates a new JIRA client
func NewClient(baseURL string, auth *JiraAuth) *Client {
	client := &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if auth != nil {
		client.Token = auth.Token
	}

	return client
}

// makeRequest performs an HTTP request to JIRA API
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := fmt.Sprintf("%s%s", c.BaseURL, endpoint)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	}

	return c.HTTPClient.Do(req)
}

// GetIssue retrieves a JIRA issue by key
func (c *Client) GetIssue(issueKey string) (*Issue, error) {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s", issueKey)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("JIRA API returned status %d: %s", resp.StatusCode, string(body))
	}

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("failed to decode issue response: %w", err)
	}

	return &issue, nil
}

// SearchIssues searches for issues using JQL
func (c *Client) SearchIssues(jql string, maxResults int, startAt int) (*SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 50
	}

	searchBody := map[string]interface{}{
		"jql":        jql,
		"maxResults": maxResults,
		"startAt":    startAt,
		"fields":     []string{"*all"},
	}

	resp, err := c.makeRequest("POST", "/rest/api/2/search", searchBody)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("JIRA search API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &result, nil
}

// GetIssueComments retrieves comments for an issue
func (c *Client) GetIssueComments(issueKey string, maxResults int, startAt int) (*CommentsResult, error) {
	if maxResults <= 0 {
		maxResults = 50
	}

	params := url.Values{}
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	params.Set("startAt", fmt.Sprintf("%d", startAt))

	endpoint := fmt.Sprintf("/rest/api/2/issue/%s/comment?%s", issueKey, params.Encode())

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for issue %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("JIRA comments API returned status %d: %s", resp.StatusCode, string(body))
	}

	var comments CommentsResult
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, fmt.Errorf("failed to decode comments response: %w", err)
	}

	return &comments, nil
}

// SearchByCVE searches for issues related to a specific CVE
func (c *Client) SearchByCVE(cve string, maxResults int) (*SearchResult, error) {
	jql := fmt.Sprintf(`text ~ "%s" OR summary ~ "%s" OR description ~ "%s"`, cve, cve, cve)
	return c.SearchIssues(jql, maxResults, 0)
}

// SearchByComponent searches for issues affecting a specific component
func (c *Client) SearchByComponent(component string, maxResults int) (*SearchResult, error) {
	jql := fmt.Sprintf(`component = "%s" OR text ~ "%s"`, component, component)
	return c.SearchIssues(jql, maxResults, 0)
}

// SearchOCPBUGSIssues searches for OCPBUGS issues specifically
func (c *Client) SearchOCPBUGSIssues(filters map[string]string, maxResults int) (*SearchResult, error) {
	jqlParts := []string{"project = OCPBUGS"}

	for key, value := range filters {
		switch key {
		case "status":
			jqlParts = append(jqlParts, fmt.Sprintf(`status = "%s"`, value))
		case "component":
			jqlParts = append(jqlParts, fmt.Sprintf(`component = "%s"`, value))
		case "priority":
			jqlParts = append(jqlParts, fmt.Sprintf(`priority = "%s"`, value))
		case "labels":
			jqlParts = append(jqlParts, fmt.Sprintf(`labels = "%s"`, value))
		case "text":
			jqlParts = append(jqlParts, fmt.Sprintf(`text ~ "%s"`, value))
		}
	}

	jql := strings.Join(jqlParts, " AND ")
	return c.SearchIssues(jql, maxResults, 0)
}

// AnalyzeSecurityImpact analyzes security impact of an issue
func (c *Client) AnalyzeSecurityImpact(issueKey string) (*SecurityAnalysis, error) {
	issue, err := c.GetIssue(issueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue for security analysis: %w", err)
	}

	analysis := &SecurityAnalysis{
		CVEs:            extractCVEs(issue),
		Components:      extractComponents(issue),
		Architectures:   extractArchitectures(issue),
		BuildSystems:    extractBuildSystems(issue),
		SecurityLevel:   getSecurityLevel(issue),
		Impact:          getImpactLevel(issue),
		Recommendations: generateRecommendations(issue),
	}

	return analysis, nil
}

// Helper functions for security analysis

func extractCVEs(issue *Issue) []string {
	var cves []string
	text := fmt.Sprintf("%s %s %s", issue.Fields.Summary, issue.Fields.Description, issue.Fields.Environment)

	// Look for CVE patterns in various fields
	cvePattern := `CVE-\d{4}-\d{4,7}`
	matches := findMatches(text, cvePattern)

	for _, match := range matches {
		cves = append(cves, match)
	}

	return removeDuplicates(cves)
}

func extractComponents(issue *Issue) []string {
	var components []string

	// Get components from JIRA fields
	for _, comp := range issue.Fields.Components {
		components = append(components, comp.Name)
	}

	// Also extract from description/summary for OpenShift components
	text := fmt.Sprintf("%s %s", issue.Fields.Summary, issue.Fields.Description)
	openshiftComponents := []string{"rhcos", "openshift4", "cri-o", "ironic", "oauth-server"}

	for _, comp := range openshiftComponents {
		if strings.Contains(strings.ToLower(text), comp) {
			components = append(components, comp)
		}
	}

	return removeDuplicates(components)
}

func extractArchitectures(issue *Issue) []string {
	text := fmt.Sprintf("%s %s %s", issue.Fields.Summary, issue.Fields.Description, issue.Fields.Environment)
	architectures := []string{"x86_64", "aarch64", "ppc64le", "s390x"}

	var found []string
	for _, arch := range architectures {
		if strings.Contains(strings.ToLower(text), strings.ToLower(arch)) {
			found = append(found, arch)
		}
	}

	// If no specific architectures mentioned, assume all are affected for security issues
	if len(found) == 0 && isSecurityIssue(issue) {
		return architectures
	}

	return found
}

func extractBuildSystems(issue *Issue) []string {
	text := fmt.Sprintf("%s %s", issue.Fields.Summary, issue.Fields.Description)
	systems := []string{"Konflux", "OSBS", "Brew"}

	var found []string
	for _, system := range systems {
		if strings.Contains(text, system) {
			found = append(found, system)
		}
	}

	return found
}

func getSecurityLevel(issue *Issue) string {
	if issue.Fields.Security != nil {
		return issue.Fields.Security.Name
	}

	// Infer security level from priority and labels
	if issue.Fields.Priority.Name == "Critical" || issue.Fields.Priority.Name == "Blocker" {
		return "High"
	}

	for _, label := range issue.Fields.Labels {
		if strings.Contains(strings.ToLower(label), "security") {
			return "Medium"
		}
	}

	return "Low"
}

func getImpactLevel(issue *Issue) string {
	priority := issue.Fields.Priority.Name

	switch strings.ToLower(priority) {
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

func generateRecommendations(issue *Issue) []string {
	var recommendations []string

	if isSecurityIssue(issue) {
		recommendations = append(recommendations,
			"Review security impact across all architectures",
			"Check for related CVEs in the same component",
			"Verify hermetic build compatibility",
		)
	}

	if containsBuildFailure(issue) {
		recommendations = append(recommendations,
			"Correlate with Jenkins build logs",
			"Check Konflux build pipeline status",
			"Review component build configuration changes",
		)
	}

	if len(issue.Fields.Components) > 1 {
		recommendations = append(recommendations,
			"Assess cross-component impact",
			"Review dependency chains between affected components",
		)
	}

	return recommendations
}

func isSecurityIssue(issue *Issue) bool {
	text := strings.ToLower(fmt.Sprintf("%s %s", issue.Fields.Summary, issue.Fields.Description))
	securityKeywords := []string{"cve", "security", "vulnerability", "exploit", "patch"}

	for _, keyword := range securityKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}

	for _, label := range issue.Fields.Labels {
		if strings.Contains(strings.ToLower(label), "security") {
			return true
		}
	}

	return false
}

func containsBuildFailure(issue *Issue) bool {
	text := strings.ToLower(fmt.Sprintf("%s %s", issue.Fields.Summary, issue.Fields.Description))
	buildKeywords := []string{"build", "compile", "konflux", "jenkins", "failure", "error"}

	for _, keyword := range buildKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}

	return false
}

func findMatches(text, pattern string) []string {
	// Simple pattern matching for CVE extraction
	var matches []string
	lines := strings.Split(text, " ")

	for _, line := range lines {
		if strings.HasPrefix(line, "CVE-") && len(line) >= 13 {
			matches = append(matches, line)
		}
	}

	return matches
}

func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}
