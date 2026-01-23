package jenkins

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	baseURL := "https://jenkins.example.com"
	auth := &JenkinsAuth{
		Username: "testuser",
		Token:    "testtoken",
	}

	client := NewClient(baseURL, auth)

	if client.baseURL != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, client.baseURL)
	}
	if client.auth != auth {
		t.Errorf("Expected auth to be set")
	}
	if client.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestNewClient_NoAuth(t *testing.T) {
	baseURL := "https://jenkins.example.com"
	client := NewClient(baseURL, nil)

	if client.baseURL != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, client.baseURL)
	}
	if client.auth != nil {
		t.Error("Expected auth to be nil")
	}
}

func TestJenkinsAuth(t *testing.T) {
	auth := &JenkinsAuth{
		Username: "user123",
		Token:    "token456",
	}

	if auth.Username != "user123" {
		t.Errorf("Expected username 'user123', got '%s'", auth.Username)
	}
	if auth.Token != "token456" {
		t.Errorf("Expected token 'token456', got '%s'", auth.Token)
	}
}

func TestKonfluxJob_Structure(t *testing.T) {
	timestamp := time.Now()
	job := KonfluxJob{
		Name:        "build-ocp4-konflux-123",
		BuildNumber: 123,
		Status:      "FAILURE",
		Timestamp:   timestamp,
		Duration:    5 * time.Minute,
		LogURL:      "https://jenkins.example.com/job/build/123/console",
		Parameters: map[string]string{
			"COMPONENT": "ironic",
			"ASSEMBLY":  "stream",
		},
		Component: "ironic",
		Assembly:  "stream",
		Group:     "openshift-4.21",
	}

	if job.BuildNumber != 123 {
		t.Errorf("Expected BuildNumber 123, got %d", job.BuildNumber)
	}
	if job.Status != "FAILURE" {
		t.Errorf("Expected Status 'FAILURE', got '%s'", job.Status)
	}
	if job.Component != "ironic" {
		t.Errorf("Expected Component 'ironic', got '%s'", job.Component)
	}
	if job.Assembly != "stream" {
		t.Errorf("Expected Assembly 'stream', got '%s'", job.Assembly)
	}
	if len(job.Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(job.Parameters))
	}
}

func TestJenkinsBuild_Structure(t *testing.T) {
	build := JenkinsBuild{
		Number:    123,
		URL:       "https://jenkins.example.com/job/build/123/",
		Result:    "FAILURE",
		Timestamp: 1640995200000, // Unix timestamp in milliseconds
		Duration:  300000,        // Duration in milliseconds
		Actions: []struct {
			Parameters []struct {
				Name  string      `json:"name"`
				Value interface{} `json:"value"`
			} `json:"parameters,omitempty"`
		}{
			{
				Parameters: []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}{
					{Name: "COMPONENT", Value: "ironic"},
					{Name: "ASSEMBLY", Value: "stream"},
				},
			},
		},
	}

	if build.Number != 123 {
		t.Errorf("Expected Number 123, got %d", build.Number)
	}
	if build.Result != "FAILURE" {
		t.Errorf("Expected Result 'FAILURE', got '%s'", build.Result)
	}
	if len(build.Actions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(build.Actions))
	}
	if len(build.Actions[0].Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(build.Actions[0].Parameters))
	}
}

func TestClient_convertToKonfluxJob(t *testing.T) {
	client := NewClient("https://jenkins.example.com", nil)

	build := JenkinsBuild{
		Number:    123,
		URL:       "https://jenkins.example.com/job/build/123/",
		Result:    "FAILURE",
		Timestamp: 1640995200000, // Unix timestamp in milliseconds
		Duration:  300000,        // 5 minutes in milliseconds
		Actions: []struct {
			Parameters []struct {
				Name  string      `json:"name"`
				Value interface{} `json:"value"`
			} `json:"parameters,omitempty"`
		}{
			{
				Parameters: []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}{
					{Name: "component", Value: "ironic"},
					{Name: "assembly", Value: "test"},
					{Name: "group", Value: "4.21"},
				},
			},
		},
	}

	job := client.convertToKonfluxJob(build)

	if job.BuildNumber != 123 {
		t.Errorf("Expected BuildNumber 123, got %d", job.BuildNumber)
	}
	if job.Status != "FAILURE" {
		t.Errorf("Expected Status 'FAILURE', got '%s'", job.Status)
	}
	if job.Component != "ironic" {
		t.Errorf("Expected Component 'ironic', got '%s'", job.Component)
	}
	if job.Assembly != "test" {
		t.Errorf("Expected Assembly 'test', got '%s'", job.Assembly)
	}
	if job.Group != "openshift-4.21" {
		t.Errorf("Expected Group 'openshift-4.21', got '%s'", job.Group)
	}
	if job.Duration != 5*time.Minute {
		t.Errorf("Expected Duration 5m, got %v", job.Duration)
	}

	expectedLogURL := "https://jenkins.example.com/job/aos-cd-builds/job/build%252Focp4-konflux/123/console"
	if job.LogURL != expectedLogURL {
		t.Errorf("Expected LogURL %s, got %s", expectedLogURL, job.LogURL)
	}
}

func TestClient_convertToKonfluxJob_Defaults(t *testing.T) {
	client := NewClient("https://jenkins.example.com", nil)

	build := JenkinsBuild{
		Number:    456,
		Result:    "SUCCESS",
		Timestamp: 1640995200000,
		Duration:  180000, // 3 minutes
		Actions:   []struct {
			Parameters []struct {
				Name  string      `json:"name"`
				Value interface{} `json:"value"`
			} `json:"parameters,omitempty"`
		}{}, // No parameters
	}

	job := client.convertToKonfluxJob(build)

	if job.Assembly != "stream" {
		t.Errorf("Expected default Assembly 'stream', got '%s'", job.Assembly)
	}
	if job.Group != "openshift-4.21" {
		t.Errorf("Expected default Group 'openshift-4.21', got '%s'", job.Group)
	}
	if job.Component != "" {
		t.Errorf("Expected empty Component, got '%s'", job.Component)
	}
}

func TestClient_AnalyzeJenkinsLogs(t *testing.T) {
	client := NewClient("https://jenkins.example.com", nil)

	tests := []struct {
		name          string
		logs          string
		expectedCount int
		expectedIssue string
	}{
		{
			name:          "authentication failure",
			logs:          "Error: authentication failed to registry.redhat.io",
			expectedCount: 1,
			expectedIssue: "Authentication issues with Konflux or registry",
		},
		{
			name:          "timeout error",
			logs:          "Build failed due to timeout after 60 minutes",
			expectedCount: 2,
			expectedIssue: "Build timeout - check for hanging processes",
		},
		{
			name:          "memory error",
			logs:          "container killed: out of memory",
			expectedCount: 1,
			expectedIssue: "Memory limit exceeded during build",
		},
		{
			name:          "permission error",
			logs:          "Permission denied: cannot write to /var/cache",
			expectedCount: 1,
			expectedIssue: "Permission or access control issues",
		},
		{
			name:          "multiple errors",
			logs:          "Error: timeout occurred\nPermission denied\nHermetic build constraint violated",
			expectedCount: 3,
			expectedIssue: "Build timeout - check for hanging processes",
		},
		{
			name:          "no specific errors",
			logs:          "Build completed successfully without specific error patterns",
			expectedCount: 1,
			expectedIssue: "No specific error patterns detected in logs",
		},
		{
			name:          "exit code 1",
			logs:          "Process exited with exit code 1",
			expectedCount: 1,
			expectedIssue: "Build process exited with error code 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := client.AnalyzeJenkinsLogs(tt.logs)

			if len(issues) != tt.expectedCount {
				t.Errorf("Expected %d issues, got %d: %v", tt.expectedCount, len(issues), issues)
			}

			found := false
			for _, issue := range issues {
				if issue == tt.expectedIssue {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected issue '%s' not found in: %v", tt.expectedIssue, issues)
			}
		})
	}
}

func TestClient_Close(t *testing.T) {
	client := NewClient("https://jenkins.example.com", nil)
	
	err := client.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}
}

func TestJenkinsJobResponse_Structure(t *testing.T) {
	response := JenkinsJobResponse{
		Builds: []JenkinsBuild{
			{
				Number:    123,
				Result:    "FAILURE",
				Timestamp: 1640995200000,
			},
			{
				Number:    124,
				Result:    "SUCCESS",
				Timestamp: 1640995260000,
			},
		},
	}

	if len(response.Builds) != 2 {
		t.Errorf("Expected 2 builds, got %d", len(response.Builds))
	}
	if response.Builds[0].Number != 123 {
		t.Errorf("Expected first build number 123, got %d", response.Builds[0].Number)
	}
	if response.Builds[1].Result != "SUCCESS" {
		t.Errorf("Expected second build result 'SUCCESS', got '%s'", response.Builds[1].Result)
	}
}

func TestClient_convertToKonfluxJob_MixedParameterTypes(t *testing.T) {
	client := NewClient("https://jenkins.example.com", nil)

	build := JenkinsBuild{
		Number:    789,
		URL:       "https://jenkins.example.com/job/build/789/",
		Result:    "SUCCESS",
		Timestamp: 1640995200000,
		Duration:  240000, // 4 minutes
		Actions: []struct {
			Parameters []struct {
				Name  string      `json:"name"`
				Value interface{} `json:"value"`
			} `json:"parameters,omitempty"`
		}{
			{
				Parameters: []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}{
					{Name: "component", Value: "oauth-server"},
					{Name: "assembly", Value: "stream"},
					{Name: "hermetic", Value: true},        // boolean parameter
					{Name: "timeout", Value: 3600.0},       // float64 parameter
					{Name: "retry_count", Value: int(3)},   // int parameter
				},
			},
		},
	}

	job := client.convertToKonfluxJob(build)

	// Test string parameter conversion
	if job.Component != "oauth-server" {
		t.Errorf("Expected Component 'oauth-server', got '%s'", job.Component)
	}

	// Test parameter map contains converted values
	if job.Parameters["hermetic"] != "true" {
		t.Errorf("Expected hermetic parameter 'true', got '%s'", job.Parameters["hermetic"])
	}
	if job.Parameters["timeout"] != "3600" {
		t.Errorf("Expected timeout parameter '3600', got '%s'", job.Parameters["timeout"])
	}
	if job.Parameters["retry_count"] != "3" {
		t.Errorf("Expected retry_count parameter '3', got '%s'", job.Parameters["retry_count"])
	}
}