package jenkins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client provides access to Jenkins REST API for Konflux builds
type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       *JenkinsAuth
}

// JenkinsAuth holds authentication credentials for Jenkins API
type JenkinsAuth struct {
	Username string
	Token    string
}

// KonfluxJob represents a Jenkins job for Konflux builds
type KonfluxJob struct {
	Name        string            `json:"name"`
	BuildNumber int               `json:"buildNumber"`
	Status      string            `json:"status"`
	Timestamp   time.Time         `json:"timestamp"`
	Duration    time.Duration     `json:"duration"`
	LogURL      string            `json:"logUrl"`
	Parameters  map[string]string `json:"parameters"`
	Component   string            `json:"component"`
	Assembly    string            `json:"assembly"`
	Group       string            `json:"group"`
}

// JenkinsBuild represents Jenkins build response structure
type JenkinsBuild struct {
	Number    int    `json:"number"`
	URL       string `json:"url"`
	Result    string `json:"result"`
	Timestamp int64  `json:"timestamp"`
	Duration  int64  `json:"duration"`
	Actions   []struct {
		Parameters []struct {
			Name  string      `json:"name"`
			Value interface{} `json:"value"` // Can be string, bool, or number
		} `json:"parameters,omitempty"`
	} `json:"actions"`
}

// JenkinsJobResponse represents Jenkins job list response
type JenkinsJobResponse struct {
	Builds []JenkinsBuild `json:"builds"`
}

// NewClient creates a new Jenkins API client
func NewClient(baseURL string, auth *JenkinsAuth) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		auth: auth,
	}
}

// QueryKonfluxBuilds queries Jenkins for Konflux builds matching the criteria
func (c *Client) QueryKonfluxBuilds(ctx context.Context, component, assembly, group string, days int) ([]KonfluxJob, error) {
	if assembly == "" {
		assembly = "stream"
	}
	if group == "" {
		group = "openshift-4.21"
	}

	// Build Jenkins API URL for aos-cd-builds/build%252Focp4-konflux
	jobPath := "job/aos-cd-builds/job/build%252Focp4-konflux/api/json"
	params := url.Values{}
	params.Set("tree", "builds[number,url,result,timestamp,duration,actions[parameters[name,value]]]")

	apiURL := fmt.Sprintf("%s/%s?%s", c.baseURL, jobPath, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.auth != nil {
		req.SetBasicAuth(c.auth.Username, c.auth.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jenkins API returned status %d: %s", resp.StatusCode, string(body))
	}

	var jobResp JenkinsJobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	cutoffTime := time.Now().AddDate(0, 0, -days)
	var jobs []KonfluxJob

	for _, build := range jobResp.Builds {
		buildTime := time.Unix(build.Timestamp/1000, 0)
		if buildTime.Before(cutoffTime) {
			continue
		}

		job := c.convertToKonfluxJob(build)

		// Filter by component if specified
		if component != "" && !strings.Contains(strings.ToLower(job.Component), strings.ToLower(component)) {
			continue
		}

		// Filter by assembly if specified and not default
		if assembly != "stream" && job.Assembly != assembly {
			continue
		}

		// Filter by group if specified
		if group != "" && job.Group != group {
			continue
		}

		jobs = append(jobs, job)
	}

	log.Printf("Found %d Konflux builds matching criteria (component: %s, assembly: %s, group: %s, days: %d)",
		len(jobs), component, assembly, group, days)

	return jobs, nil
}

// GetJenkinsLogs retrieves console logs for a specific Jenkins build
func (c *Client) GetJenkinsLogs(ctx context.Context, buildNumber int) (string, error) {
	logURL := fmt.Sprintf("%s/job/aos-cd-builds/job/build%%252Focp4-konflux/%d/consoleText", c.baseURL, buildNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", logURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create log request: %w", err)
	}

	if c.auth != nil {
		req.SetBasicAuth(c.auth.Username, c.auth.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to retrieve logs, status: %d", resp.StatusCode)
	}

	logBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read log content: %w", err)
	}

	return string(logBytes), nil
}

// AnalyzeJenkinsLogs performs pattern analysis on Jenkins console logs
func (c *Client) AnalyzeJenkinsLogs(logs string) []string {
	var issues []string
	logContent := strings.ToLower(logs)

	// Check for common Konflux build failure patterns
	patterns := map[string]string{
		"authentication failed": "Authentication issues with Konflux or registry",
		"timeout":               "Build timeout - check for hanging processes",
		"out of memory":         "Memory limit exceeded during build",
		"no space left":         "Disk space exhausted",
		"network unreachable":   "Network connectivity issues",
		"permission denied":     "Permission or access control issues",
		"image not found":       "Base image or dependency not found",
		"build failed":          "General build failure",
		"hermetic":              "Hermetic build constraint violation",
		"cachi2":                "Dependency caching issues",
		"lockfile":              "Dependency lockfile issues",
		"conflict":              "Build conflict or resource contention",
	}

	for pattern, description := range patterns {
		if strings.Contains(logContent, pattern) {
			issues = append(issues, description)
		}
	}

	// Look for specific error codes
	if strings.Contains(logContent, "exit code 1") {
		issues = append(issues, "Build process exited with error code 1")
	}
	if strings.Contains(logContent, "exit code 2") {
		issues = append(issues, "Build process exited with error code 2")
	}

	if len(issues) == 0 {
		issues = append(issues, "No specific error patterns detected in logs")
	}

	return issues
}

// GetBuildDetails retrieves detailed information for a specific build
func (c *Client) GetBuildDetails(ctx context.Context, buildNumber int) (*KonfluxJob, error) {
	buildURL := fmt.Sprintf("%s/job/aos-cd-builds/job/build%%252Focp4-konflux/%d/api/json", c.baseURL, buildNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", buildURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.auth != nil {
		req.SetBasicAuth(c.auth.Username, c.auth.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jenkins API returned status %d", resp.StatusCode)
	}

	var build JenkinsBuild
	if err := json.NewDecoder(resp.Body).Decode(&build); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	job := c.convertToKonfluxJob(build)
	return &job, nil
}

// convertToKonfluxJob converts Jenkins build data to KonfluxJob
func (c *Client) convertToKonfluxJob(build JenkinsBuild) KonfluxJob {
	job := KonfluxJob{
		Name:        fmt.Sprintf("build-ocp4-konflux-%d", build.Number),
		BuildNumber: build.Number,
		Status:      build.Result,
		Timestamp:   time.Unix(build.Timestamp/1000, 0),
		Duration:    time.Duration(build.Duration) * time.Millisecond,
		LogURL:      fmt.Sprintf("%s/job/aos-cd-builds/job/build%%252Focp4-konflux/%d/console", c.baseURL, build.Number),
		Parameters:  make(map[string]string),
		Assembly:    "stream",         // default
		Group:       "openshift-4.21", // default
	}

	// Extract parameters from build actions
	for _, action := range build.Actions {
		for _, param := range action.Parameters {
			// Convert parameter value to string
			valueStr := ""
			switch v := param.Value.(type) {
			case string:
				valueStr = v
			case bool:
				if v {
					valueStr = "true"
				} else {
					valueStr = "false"
				}
			case float64:
				valueStr = fmt.Sprintf("%.0f", v)
			case int:
				valueStr = fmt.Sprintf("%d", v)
			default:
				valueStr = fmt.Sprintf("%v", v)
			}

			job.Parameters[param.Name] = valueStr

			// Extract component and assembly from common parameter patterns
			switch strings.ToLower(param.Name) {
			case "component", "component_name":
				job.Component = valueStr
			case "assembly", "assembly_type":
				job.Assembly = valueStr
			case "group", "version", "openshift_version":
				if strings.HasPrefix(valueStr, "openshift-") {
					job.Group = valueStr
				} else {
					job.Group = "openshift-" + valueStr
				}
			}
		}
	}

	return job
}

// Close closes any resources used by the client
func (c *Client) Close() error {
	// HTTP client doesn't require explicit cleanup
	return nil
}
