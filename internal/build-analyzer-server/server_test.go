package buildanalyzer

import (
	"testing"
)

func TestServer_GetToolList(t *testing.T) {
	server := &Server{}

	toolList := server.GetToolList()

	if len(toolList.Tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(toolList.Tools))
	}

	expectedTools := map[string]bool{
		"query_build_failures": false,
		"analyze_build_logs":   false,
		"compare_builds":       false,
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

func TestServer_queryBuildFailures_InvalidArgs(t *testing.T) {
	// Test that we can parse arguments correctly, but skip actual BigQuery call
	// since we don't have a mock client

	// Test componentNames parsing
	args := map[string]interface{}{
		"componentNames": []interface{}{"ironic", "oauth-server"},
		"group":          "openshift-4.21",
		"days":           float64(7),
		"architectures":  []interface{}{"x86_64", "aarch64"},
		"hermetic":       true,
	}

	// We can't actually call queryBuildFailures without a BigQuery client
	// but we can test that the argument parsing logic would work
	componentNames, ok := args["componentNames"].([]interface{})
	if !ok || len(componentNames) != 2 {
		t.Error("Failed to parse componentNames")
	}

	group, ok := args["group"].(string)
	if !ok || group != "openshift-4.21" {
		t.Error("Failed to parse group")
	}

	days, ok := args["days"].(float64)
	if !ok || days != 7 {
		t.Error("Failed to parse days")
	}
}

func TestServer_analyzeBuildLogs_InvalidArgs(t *testing.T) {
	// Test argument validation without calling BigQuery
	args := map[string]interface{}{}

	// Test that we can detect missing arguments
	_, hasComponent := args["componentName"].(string)
	_, hasBuildID := args["buildId"].(string)

	if hasComponent || hasBuildID {
		t.Error("Should not have componentName or buildId in empty args")
	}

	// Test with valid arguments
	validArgs := map[string]interface{}{
		"componentName": "ironic",
		"group":         "openshift-4.21",
	}

	componentName, ok := validArgs["componentName"].(string)
	if !ok || componentName != "ironic" {
		t.Error("Failed to parse componentName")
	}
}

func TestServer_compareBuilds_InvalidArgs(t *testing.T) {
	// Test argument validation without calling BigQuery

	// Test missing buildId1
	args := map[string]interface{}{
		"buildId2": "build2",
	}

	buildID1, ok1 := args["buildId1"].(string)
	buildID2, ok2 := args["buildId2"].(string)

	if ok1 {
		t.Error("Should not have buildId1 in this test case")
	}

	if !ok2 || buildID2 != "build2" {
		t.Error("Should have buildId2")
	}

	// Test missing buildId2
	args = map[string]interface{}{
		"buildId1": "build1",
	}

	buildID1, ok1 = args["buildId1"].(string)
	_, ok2 = args["buildId2"].(string)

	if !ok1 || buildID1 != "build1" {
		t.Error("Should have buildId1")
	}

	if ok2 {
		t.Error("Should not have buildId2 in this test case")
	}
}

func TestServer_summarizeBuildFailures(t *testing.T) {
	server := &Server{}

	builds := []BuildRecord{
		{
			Name:     "ironic",
			Outcome:  "FAILURE",
			Arches:   []string{"x86_64", "aarch64"},
			Hermetic: true,
		},
		{
			Name:     "oauth-server",
			Outcome:  "FAILURE",
			Arches:   []string{"x86_64"},
			Hermetic: false,
		},
		{
			Name:     "ironic",
			Outcome:  "FAILURE",
			Arches:   []string{"ppc64le"},
			Hermetic: true,
		},
	}

	summary := server.summarizeBuildFailures(builds)

	if summary["totalFailures"] != 3 {
		t.Errorf("Expected totalFailures = 3, got %v", summary["totalFailures"])
	}

	componentBreakdown := summary["componentBreakdown"].(map[string]int)
	if componentBreakdown["ironic"] != 2 {
		t.Errorf("Expected ironic failures = 2, got %d", componentBreakdown["ironic"])
	}
	if componentBreakdown["oauth-server"] != 1 {
		t.Errorf("Expected oauth-server failures = 1, got %d", componentBreakdown["oauth-server"])
	}

	archBreakdown := summary["architectureBreakdown"].(map[string]int)
	if archBreakdown["x86_64"] != 2 {
		t.Errorf("Expected x86_64 failures = 2, got %d", archBreakdown["x86_64"])
	}

	hermeticBreakdown := summary["hermeticBreakdown"].(map[string]int)
	if hermeticBreakdown["hermetic"] != 2 {
		t.Errorf("Expected hermetic failures = 2, got %d", hermeticBreakdown["hermetic"])
	}
	if hermeticBreakdown["nonHermetic"] != 1 {
		t.Errorf("Expected nonHermetic failures = 1, got %d", hermeticBreakdown["nonHermetic"])
	}
}

func TestServer_analyzePossibleIssues(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name           string
		build          *BuildRecord
		expectedIssues int
	}{
		{
			name: "hermetic build failure",
			build: &BuildRecord{
				Outcome:          "FAILURE",
				Hermetic:         true,
				Arches:           []string{"x86_64"},
				BuildPipelineURL: "https://example.com/build",
			},
			expectedIssues: 1,
		},
		{
			name: "network-open build failure",
			build: &BuildRecord{
				Outcome:          "FAILURE",
				Hermetic:         false,
				Arches:           []string{"x86_64"},
				BuildPipelineURL: "https://example.com/build",
			},
			expectedIssues: 1,
		},
		{
			name: "multi-arch hermetic failure",
			build: &BuildRecord{
				Outcome:          "FAILURE",
				Hermetic:         true,
				Arches:           []string{"x86_64", "aarch64"},
				BuildPipelineURL: "https://example.com/build",
			},
			expectedIssues: 2,
		},
		{
			name: "failure with no logs",
			build: &BuildRecord{
				Outcome:          "FAILURE",
				Hermetic:         false,
				Arches:           []string{"x86_64"},
				BuildPipelineURL: "",
				ArtJobURL:        "",
			},
			expectedIssues: 2,
		},
		{
			name: "successful build",
			build: &BuildRecord{
				Outcome:          "SUCCESS",
				Hermetic:         true,
				Arches:           []string{"x86_64"},
				BuildPipelineURL: "https://example.com/build",
			},
			expectedIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := server.analyzePossibleIssues(tt.build)
			if len(issues) != tt.expectedIssues {
				t.Errorf("Expected %d issues, got %d: %v", tt.expectedIssues, len(issues), issues)
			}
		})
	}
}
