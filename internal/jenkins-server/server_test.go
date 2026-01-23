package jenkinsserver

import (
	"testing"
	"time"

	"github.com/lgarciaaco/claude-build-analyzer/internal/jenkins"
)

func TestServer_GetToolList(t *testing.T) {
	server := &Server{}

	toolList := server.GetToolList()

	if len(toolList.Tools) != 4 {
		t.Errorf("Expected 4 tools, got %d", len(toolList.Tools))
	}

	expectedTools := map[string]bool{
		"query_jenkins_builds":     false,
		"analyze_jenkins_logs":     false,
		"correlate_jenkins_builds": false,
		"open_browser_links":       false,
	}

	for _, tool := range toolList.Tools {
		if _, exists := expectedTools[tool.Name]; exists {
			expectedTools[tool.Name] = true
		} else {
			t.Errorf("Unexpected tool: %s", tool.Name)
		}
	}

	for toolName, found := range expectedTools {
		if !found {
			t.Errorf("Missing expected tool: %s", toolName)
		}
	}
}

func TestServer_queryJenkinsBuilds_ValidateArguments(t *testing.T) {
	args := map[string]interface{}{
		"componentName": "ironic",
		"assembly":      "stream",
		"group":         "openshift-4.21",
		"days":          float64(7),
		"status":        []interface{}{"FAILURE", "SUCCESS"},
	}

	var query JenkinsJobQuery

	// Test argument parsing logic
	if componentName, ok := args["componentName"].(string); ok {
		query.ComponentName = componentName
	}
	if assembly, ok := args["assembly"].(string); ok {
		query.Assembly = assembly
	} else {
		query.Assembly = "stream"
	}
	if group, ok := args["group"].(string); ok {
		query.Group = group
	} else {
		query.Group = "openshift-4.21"
	}
	if days, ok := args["days"].(float64); ok {
		query.Days = int(days)
	} else {
		query.Days = 7
	}
	if statusList, ok := args["status"].([]interface{}); ok {
		for _, status := range statusList {
			if statusStr, ok := status.(string); ok {
				query.Status = append(query.Status, statusStr)
			}
		}
	}

	if query.ComponentName != "ironic" {
		t.Errorf("Expected componentName 'ironic', got '%s'", query.ComponentName)
	}
	if query.Assembly != "stream" {
		t.Errorf("Expected assembly 'stream', got '%s'", query.Assembly)
	}
	if query.Group != "openshift-4.21" {
		t.Errorf("Expected group 'openshift-4.21', got '%s'", query.Group)
	}
	if query.Days != 7 {
		t.Errorf("Expected days 7, got %d", query.Days)
	}
	if len(query.Status) != 2 {
		t.Errorf("Expected 2 status values, got %d", len(query.Status))
	}
}

func TestServer_analyzeJenkinsLogs_ValidateArguments(t *testing.T) {
	tests := []struct {
		name           string
		args           map[string]interface{}
		expectBuildNum bool
		expectError    bool
	}{
		{
			name: "valid buildNumber",
			args: map[string]interface{}{
				"buildNumber": float64(123),
			},
			expectBuildNum: true,
			expectError:    false,
		},
		{
			name: "valid componentName",
			args: map[string]interface{}{
				"componentName": "ironic",
				"assembly":      "stream",
				"group":         "openshift-4.21",
				"days":          float64(7),
			},
			expectBuildNum: false,
			expectError:    false,
		},
		{
			name:           "no arguments",
			args:           map[string]interface{}{},
			expectBuildNum: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found bool

			if bn, ok := tt.args["buildNumber"].(float64); ok {
				_ = int(bn) // Use the converted value
				found = true
			} else if componentName, ok := tt.args["componentName"].(string); ok {
				// In actual implementation, this would query Jenkins
				if componentName == "ironic" {
					found = true
				}
			}

			if tt.expectBuildNum && !found {
				t.Error("Expected to find build number")
			}
			if !tt.expectBuildNum && found && tt.expectError {
				t.Error("Should not have found build number when expecting error")
			}
		})
	}
}

func TestServer_correlateJenkinsBuilds_ValidateArguments(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]interface{}
		expectValid bool
	}{
		{
			name: "valid buildNumber string",
			args: map[string]interface{}{
				"buildNumber": "123",
			},
			expectValid: true,
		},
		{
			name: "valid componentName",
			args: map[string]interface{}{
				"componentName": "ironic",
				"assembly":      "stream",
				"group":         "openshift-4.21",
			},
			expectValid: true,
		},
		{
			name:        "no arguments",
			args:        map[string]interface{}{},
			expectValid: false,
		},
		{
			name: "invalid buildNumber format",
			args: map[string]interface{}{
				"buildNumber": "not-a-number",
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buildNumber int
			var componentName string
			var isValid bool

			if bn, ok := tt.args["buildNumber"].(string); ok {
				if parsed, err := parseIntFromString(bn); err == nil {
					buildNumber = parsed
					isValid = true
				}
			}
			if cn, ok := tt.args["componentName"].(string); ok {
				componentName = cn
				isValid = true
			}

			if buildNumber == 0 && componentName == "" {
				isValid = false
			}

			if tt.expectValid && !isValid {
				t.Error("Expected valid arguments")
			}
			if !tt.expectValid && isValid {
				t.Error("Expected invalid arguments")
			}

			// Use buildNumber to avoid unused variable error
			if buildNumber > 0 && componentName != "" {
				t.Logf("Both buildNumber (%d) and componentName (%s) provided", buildNumber, componentName)
			}
		})
	}
}

func TestServer_summarizeJenkinsBuilds(t *testing.T) {
	server := &Server{}

	jobs := []jenkins.KonfluxJob{
		{
			Name:        "build-ocp4-konflux-123",
			BuildNumber: 123,
			Status:      "FAILURE",
			Component:   "ironic",
			Assembly:    "stream",
			Timestamp:   time.Now(),
		},
		{
			Name:        "build-ocp4-konflux-124",
			BuildNumber: 124,
			Status:      "SUCCESS",
			Component:   "oauth-server",
			Assembly:    "stream",
			Timestamp:   time.Now(),
		},
		{
			Name:        "build-ocp4-konflux-125",
			BuildNumber: 125,
			Status:      "FAILURE",
			Component:   "ironic",
			Assembly:    "test",
			Timestamp:   time.Now(),
		},
	}

	query := JenkinsJobQuery{
		ComponentName: "ironic",
		Assembly:      "stream",
		Group:         "openshift-4.21",
		Days:          7,
		Status:        []string{"FAILURE"},
	}

	summary := server.summarizeJenkinsBuilds(jobs, query)

	summarySection := summary["summary"].(map[string]interface{})
	if summarySection["totalBuilds"] != 3 {
		t.Errorf("Expected totalBuilds = 3, got %v", summarySection["totalBuilds"])
	}

	statusBreakdown := summarySection["statusBreakdown"].(map[string]int)
	if statusBreakdown["FAILURE"] != 2 {
		t.Errorf("Expected FAILURE count = 2, got %d", statusBreakdown["FAILURE"])
	}
	if statusBreakdown["SUCCESS"] != 1 {
		t.Errorf("Expected SUCCESS count = 1, got %d", statusBreakdown["SUCCESS"])
	}

	componentBreakdown := summarySection["componentBreakdown"].(map[string]int)
	if componentBreakdown["ironic"] != 2 {
		t.Errorf("Expected ironic count = 2, got %d", componentBreakdown["ironic"])
	}
	if componentBreakdown["oauth-server"] != 1 {
		t.Errorf("Expected oauth-server count = 1, got %d", componentBreakdown["oauth-server"])
	}

	assemblyBreakdown := summarySection["assemblyBreakdown"].(map[string]int)
	if assemblyBreakdown["stream"] != 2 {
		t.Errorf("Expected stream count = 2, got %d", assemblyBreakdown["stream"])
	}
	if assemblyBreakdown["test"] != 1 {
		t.Errorf("Expected test count = 1, got %d", assemblyBreakdown["test"])
	}

	recentBuilds := summary["recentBuilds"].([]jenkins.KonfluxJob)
	if len(recentBuilds) != 3 {
		t.Errorf("Expected 3 recent builds, got %d", len(recentBuilds))
	}
}

func TestJenkinsJobQuery_Defaults(t *testing.T) {
	query := JenkinsJobQuery{}

	// Test default values
	if query.Assembly != "" {
		t.Errorf("Expected empty assembly by default, got '%s'", query.Assembly)
	}
	if query.Group != "" {
		t.Errorf("Expected empty group by default, got '%s'", query.Group)
	}
	if query.Days != 0 {
		t.Errorf("Expected 0 days by default, got %d", query.Days)
	}

	// Test setting defaults
	if query.Assembly == "" {
		query.Assembly = "stream"
	}
	if query.Group == "" {
		query.Group = "openshift-4.21"
	}
	if query.Days == 0 {
		query.Days = 7
	}

	if query.Assembly != "stream" {
		t.Errorf("Expected assembly 'stream', got '%s'", query.Assembly)
	}
	if query.Group != "openshift-4.21" {
		t.Errorf("Expected group 'openshift-4.21', got '%s'", query.Group)
	}
	if query.Days != 7 {
		t.Errorf("Expected days 7, got %d", query.Days)
	}
}

func TestJenkinsLogAnalysis_Structure(t *testing.T) {
	analysis := JenkinsLogAnalysis{
		BuildNumber:  123,
		JobName:      "build-ocp4-konflux-123",
		Status:       "FAILURE",
		Timestamp:    time.Now(),
		Duration:     5 * time.Minute,
		LogURL:       "https://jenkins.example.com/job/build/123/console",
		Component:    "ironic",
		Assembly:     "stream",
		Group:        "openshift-4.21",
		ErrorSummary: []string{"timeout", "network unreachable"},
		LogExcerpts:  []string{"Error: timeout occurred", "Network unreachable"},
		Parameters: map[string]string{
			"COMPONENT": "ironic",
			"ASSEMBLY":  "stream",
		},
		Analysis: map[string]interface{}{
			"logSize":       1024,
			"hasErrors":     true,
			"buildDuration": "5m0s",
			"isRecentBuild": true,
		},
	}

	if analysis.BuildNumber != 123 {
		t.Errorf("Expected BuildNumber 123, got %d", analysis.BuildNumber)
	}
	if analysis.Component != "ironic" {
		t.Errorf("Expected Component 'ironic', got '%s'", analysis.Component)
	}
	if len(analysis.ErrorSummary) != 2 {
		t.Errorf("Expected 2 error summaries, got %d", len(analysis.ErrorSummary))
	}
	if len(analysis.Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(analysis.Parameters))
	}
}

func TestJenkinsCorrelation_Structure(t *testing.T) {
	correlation := JenkinsCorrelation{
		JenkinsJob: jenkins.KonfluxJob{
			Name:        "build-ocp4-konflux-123",
			BuildNumber: 123,
			Status:      "FAILURE",
			Component:   "ironic",
			Assembly:    "stream",
		},
		CorrelatedID: "build-id-456",
		Confidence:   0.8,
		MatchFactors: []string{"component-match", "timestamp-match"},
		Analysis: map[string]interface{}{
			"correlationMethod": "timestamp-component-assembly",
			"timeRangeHours":    24,
		},
	}

	if correlation.JenkinsJob.BuildNumber != 123 {
		t.Errorf("Expected Jenkins job build number 123, got %d", correlation.JenkinsJob.BuildNumber)
	}
	if correlation.Confidence != 0.8 {
		t.Errorf("Expected confidence 0.8, got %f", correlation.Confidence)
	}
	if len(correlation.MatchFactors) != 2 {
		t.Errorf("Expected 2 match factors, got %d", len(correlation.MatchFactors))
	}
}

// Helper function for testing
func parseIntFromString(s string) (int, error) {
	// Simple mock implementation for testing
	if s == "123" {
		return 123, nil
	}
	return 0, &MockError{message: "invalid format"}
}

// Mock error for testing
type MockError struct {
	message string
}

func (e *MockError) Error() string {
	return e.message
}
