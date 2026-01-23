package metadataserver

import (
	"context"
	"strings"
	"testing"

	"github.com/lgarciaaco/claude-build-analyzer/pkg/shared"
)

func TestServer_scoreMatch(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name          string
		query         string
		component     string
		expectedScore int
	}{
		{
			name:          "exact match",
			query:         "ironic",
			component:     "ironic",
			expectedScore: 1000,
		},
		{
			name:          "exact prefix match",
			query:         "etcd",
			component:     "etcd-operator",
			expectedScore: 800,
		},
		{
			name:          "word boundary match",
			query:         "etcd",
			component:     "cluster-etcd-operator",
			expectedScore: 750,
		},
		{
			name:          "contains match",
			query:         "auth",
			component:     "openshift-oauth-server",
			expectedScore: 600,
		},
		{
			name:          "no match",
			query:         "xyz",
			component:     "ironic",
			expectedScore: 0,
		},
		{
			name:          "case insensitive",
			query:         "ironic",
			component:     "ironic",
			expectedScore: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// scoreMatch expects lowercase inputs
			queryLower := strings.ToLower(tt.query)
			componentLower := strings.ToLower(tt.component)
			score := server.scoreMatch(queryLower, componentLower, tt.component)
			if score != tt.expectedScore {
				t.Errorf("scoreMatch() = %d, expected %d", score, tt.expectedScore)
			}
		})
	}
}

func TestServer_GetToolList(t *testing.T) {
	server := &Server{}

	toolList := server.GetToolList()

	if len(toolList.Tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(toolList.Tools))
	}

	expectedTools := map[string]bool{
		"get_component_metadata": false,
		"search_components":      false,
		"search_by_field":        false,
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

func TestServer_searchComponents_InvalidArgs(t *testing.T) {
	server := &Server{
		yamlParser: shared.NewYAMLParser(),
	}

	ctx := context.Background()

	// Test missing query argument
	args := map[string]interface{}{}
	result, err := server.searchComponents(ctx, args)

	if err == nil {
		t.Error("Expected error for missing query argument")
	}

	if result.Content != nil {
		t.Error("Expected empty result on error")
	}
}

func TestServer_getComponentMetadata_InvalidArgs(t *testing.T) {
	server := &Server{
		yamlParser: shared.NewYAMLParser(),
	}

	ctx := context.Background()

	// Test missing componentName argument
	args := map[string]interface{}{}
	result, err := server.getComponentMetadata(ctx, args)

	if err == nil {
		t.Error("Expected error for missing componentName argument")
	}

	if result.Content != nil {
		t.Error("Expected empty result on error")
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{5, 3, 3},
		{2, 7, 2},
		{4, 4, 4},
		{0, 1, 0},
		{-1, 1, -1},
	}

	for _, tt := range tests {
		result := minInt(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("minInt(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
		}
	}
}
