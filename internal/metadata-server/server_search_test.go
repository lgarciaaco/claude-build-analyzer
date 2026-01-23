package metadataserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lgarciaaco/claude-build-analyzer/pkg/shared"
)

// mockGitRepository implements GitRepository for testing
type mockGitRepository struct {
	files  map[string][]byte
	branch string
}

func (m *mockGitRepository) Initialize() error {
	return nil
}

func (m *mockGitRepository) GetFile(path string) ([]byte, error) {
	if data, exists := m.files[path]; exists {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockGitRepository) ListFiles(pattern string) ([]string, error) {
	var files []string
	for path := range m.files {
		if matched, _ := filepath.Match(pattern, path); matched {
			files = append(files, path)
		}
	}
	return files, nil
}

func (m *mockGitRepository) GetCommitHistory(path string, limit int) ([]shared.Commit, error) {
	return []shared.Commit{}, nil
}

func (m *mockGitRepository) GetBranch() string {
	return m.branch
}

func (m *mockGitRepository) Update() error {
	return nil
}

// mockGitManager implements GitManager for testing
type mockGitManager struct {
	repositories map[string]*mockGitRepository
}

func (m *mockGitManager) Initialize() error {
	return nil
}

func (m *mockGitManager) GetRepository(branch string) (shared.GitRepository, error) {
	if repo, exists := m.repositories[branch]; exists {
		return repo, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockGitManager) GetAllBranches() []string {
	var branches []string
	for branch := range m.repositories {
		branches = append(branches, branch)
	}
	return branches
}

func (m *mockGitManager) UpdateAll() error {
	return nil
}

// createTestServer creates a server instance for testing
func createTestServer() (*Server, *mockGitManager) {
	manager := &mockGitManager{
		repositories: make(map[string]*mockGitRepository),
	}

	// Create test repository with sample YAML files
	testRepo := &mockGitRepository{
		files:  make(map[string][]byte),
		branch: "openshift-4.21",
	}

	// Add sample YAML files with owners
	testRepo.files["images/cluster-monitoring-operator.yml"] = []byte(`
name: cluster-monitoring-operator
owners:
  - team-monitoring@redhat.com
labels:
  io.k8s.description: "Manages the lifecycle of monitoring stack"
  io.k8s.display-name: "OpenShift monitoring operator"
  License: "ASL 2.0"
content:
  source:
    git:
      url: git@github.com:openshift-priv/cluster-monitoring-operator.git
      branch:
        target: release-{MAJOR}.{MINOR}
from:
  member: openshift-enterprise-base-rhel9
konflux:
  network_mode: open
`)

	testRepo.files["images/monitoring-plugin.yml"] = []byte(`
name: monitoring-plugin
owners:
  - team-monitoring@redhat.com
labels:
  io.k8s.description: "Frontend plugin for monitoring UI"
  License: "Apache 2.0"
content:
  source:
    git:
      url: git@github.com:openshift-priv/monitoring-plugin.git
from:
  member: openshift-enterprise-base-rhel9
konflux:
  network_mode: open
`)

	testRepo.files["images/oauth-server.yml"] = []byte(`
name: oauth-server
owners:
  - team-auth@redhat.com
labels:
  io.k8s.description: "OAuth authentication server"
  License: "ASL 2.0"
content:
  source:
    git:
      url: git@github.com:openshift-priv/oauth-server.git
from:
  member: openshift-enterprise-base-rhel9
`)

	testRepo.files["images/etcd-operator.yml"] = []byte(`
name: etcd-operator
maintainers:
  - team-etcd@redhat.com
labels:
  io.k8s.description: "Etcd cluster operator"
  component: etcd
content:
  source:
    git:
      url: git@github.com:openshift-priv/etcd-operator.git
`)

	manager.repositories["openshift-4.21"] = testRepo

	server := &Server{
		gitManager: manager,
		yamlParser: shared.NewYAMLParser(),
		cache:      newMetadataCache(100, 10*time.Minute),
		ctx:        context.Background(),
	}

	return server, manager
}

func TestServer_SearchByField_Owners(t *testing.T) {
	server, _ := createTestServer()

	tests := []struct {
		name               string
		field              string
		value              string
		version            string
		maxResults         int
		caseSensitive      bool
		expectedCount      int
		expectedComponents []string
	}{
		{
			name:               "search owners for team-monitoring exact",
			field:              "owners",
			value:              "team-monitoring@redhat.com",
			version:            "4.21",
			maxResults:         10,
			caseSensitive:      false,
			expectedCount:      2,
			expectedComponents: []string{"cluster-monitoring-operator", "monitoring-plugin"},
		},
		{
			name:               "search owners for monitoring partial",
			field:              "owners",
			value:              "monitoring",
			version:            "4.21",
			maxResults:         10,
			caseSensitive:      false,
			expectedCount:      2,
			expectedComponents: []string{"cluster-monitoring-operator", "monitoring-plugin"},
		},
		{
			name:               "search owners for team-auth",
			field:              "owners",
			value:              "team-auth@redhat.com",
			version:            "4.21",
			maxResults:         10,
			caseSensitive:      false,
			expectedCount:      1,
			expectedComponents: []string{"oauth-server"},
		},
		{
			name:               "search owners for non-existent team",
			field:              "owners",
			value:              "team-nonexistent@redhat.com",
			version:            "4.21",
			maxResults:         10,
			caseSensitive:      false,
			expectedCount:      0,
			expectedComponents: []string{},
		},
		{
			name:               "search maintainers field",
			field:              "maintainers",
			value:              "team-etcd@redhat.com",
			version:            "4.21",
			maxResults:         10,
			caseSensitive:      false,
			expectedCount:      1,
			expectedComponents: []string{"etcd-operator"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"field":         tt.field,
				"value":         tt.value,
				"version":       tt.version,
				"maxResults":    float64(tt.maxResults),
				"caseSensitive": tt.caseSensitive,
			}

			result, err := server.searchByField(context.Background(), args)
			if err != nil {
				t.Fatalf("searchByField() error = %v", err)
			}

			if len(result.Content) == 0 {
				t.Fatal("Expected result content")
			}

			// Parse the JSON result
			var searchResult shared.FieldSearchResult
			err = shared.ParseJSONResult(result, &searchResult)
			if err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			if searchResult.TotalFound != tt.expectedCount {
				t.Errorf("Expected %d matches, got %d", tt.expectedCount, searchResult.TotalFound)
			}

			if len(searchResult.Matches) != tt.expectedCount {
				t.Errorf("Expected %d matches in array, got %d", tt.expectedCount, len(searchResult.Matches))
			}

			// Check that all expected components are found
			foundComponents := make(map[string]bool)
			for _, match := range searchResult.Matches {
				foundComponents[match.ComponentName] = true
			}

			for _, expectedComponent := range tt.expectedComponents {
				if !foundComponents[expectedComponent] {
					t.Errorf("Expected to find component %s", expectedComponent)
				}
			}

			// Verify match metadata
			for _, match := range searchResult.Matches {
				if match.Field != tt.field {
					t.Errorf("Expected field %s, got %s", tt.field, match.Field)
				}
				if match.MatchType == "" {
					t.Error("Expected non-empty match type")
				}
			}
		})
	}
}

func TestServer_SearchByField_NestedFields(t *testing.T) {
	server, _ := createTestServer()

	tests := []struct {
		name               string
		field              string
		value              string
		expectedCount      int
		expectedComponents []string
	}{
		{
			name:               "search nested labels description",
			field:              "labels.io.k8s.description",
			value:              "monitoring",
			expectedCount:      3, // cluster-monitoring-operator, monitoring-plugin, oauth-server (OAuth authentication server doesn't match)
			expectedComponents: []string{"cluster-monitoring-operator", "monitoring-plugin"},
		},
		{
			name:               "search content git url",
			field:              "content.source.git.url",
			value:              "monitoring",
			expectedCount:      2,
			expectedComponents: []string{"cluster-monitoring-operator", "monitoring-plugin"},
		},
		{
			name:               "search labels license",
			field:              "labels.License",
			value:              "ASL 2.0",
			expectedCount:      2,
			expectedComponents: []string{"cluster-monitoring-operator", "oauth-server"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"field":         tt.field,
				"value":         tt.value,
				"version":       "4.21",
				"maxResults":    float64(10),
				"caseSensitive": false,
			}

			result, err := server.searchByField(context.Background(), args)
			if err != nil {
				t.Fatalf("searchByField() error = %v", err)
			}

			var searchResult shared.FieldSearchResult
			err = shared.ParseJSONResult(result, &searchResult)
			if err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			// At least the expected components should be found
			if searchResult.TotalFound < len(tt.expectedComponents) {
				t.Errorf("Expected at least %d matches, got %d", len(tt.expectedComponents), searchResult.TotalFound)
			}

			foundComponents := make(map[string]bool)
			for _, match := range searchResult.Matches {
				foundComponents[match.ComponentName] = true
			}

			for _, expectedComponent := range tt.expectedComponents {
				if !foundComponents[expectedComponent] {
					t.Errorf("Expected to find component %s", expectedComponent)
				}
			}
		})
	}
}

func TestServer_SearchByField_ErrorCases(t *testing.T) {
	server, _ := createTestServer()

	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
	}{
		{
			name: "missing field parameter",
			args: map[string]interface{}{
				"value":   "test",
				"version": "4.21",
			},
			expectError: true,
		},
		{
			name: "missing value parameter",
			args: map[string]interface{}{
				"field":   "owners",
				"version": "4.21",
			},
			expectError: true,
		},
		{
			name: "invalid version",
			args: map[string]interface{}{
				"field":   "owners",
				"value":   "test",
				"version": "invalid-version",
			},
			expectError: true,
		},
		{
			name: "valid minimal args",
			args: map[string]interface{}{
				"field": "owners",
				"value": "test",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.searchByField(context.Background(), tt.args)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestServer_SearchByField_CaseSensitivity(t *testing.T) {
	server, _ := createTestServer()

	tests := []struct {
		name          string
		value         string
		caseSensitive bool
		expectedCount int
	}{
		{
			name:          "case insensitive search",
			value:         "TEAM-MONITORING@REDHAT.COM",
			caseSensitive: false,
			expectedCount: 2,
		},
		{
			name:          "case sensitive search - no match",
			value:         "TEAM-MONITORING@REDHAT.COM",
			caseSensitive: true,
			expectedCount: 0,
		},
		{
			name:          "case sensitive search - exact match",
			value:         "team-monitoring@redhat.com",
			caseSensitive: true,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"field":         "owners",
				"value":         tt.value,
				"version":       "4.21",
				"maxResults":    float64(10),
				"caseSensitive": tt.caseSensitive,
			}

			result, err := server.searchByField(context.Background(), args)
			if err != nil {
				t.Fatalf("searchByField() error = %v", err)
			}

			var searchResult shared.FieldSearchResult
			err = shared.ParseJSONResult(result, &searchResult)
			if err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			if searchResult.TotalFound != tt.expectedCount {
				t.Errorf("Expected %d matches, got %d", tt.expectedCount, searchResult.TotalFound)
			}
		})
	}
}

func TestServer_SearchByField_MaxResults(t *testing.T) {
	server, _ := createTestServer()

	args := map[string]interface{}{
		"field":      "labels.io.k8s.description",
		"value":      "server", // Should match oauth-server and potentially others
		"version":    "4.21",
		"maxResults": float64(1), // Limit to 1 result
	}

	result, err := server.searchByField(context.Background(), args)
	if err != nil {
		t.Fatalf("searchByField() error = %v", err)
	}

	var searchResult shared.FieldSearchResult
	err = shared.ParseJSONResult(result, &searchResult)
	if err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(searchResult.Matches) > 1 {
		t.Errorf("Expected at most 1 match due to maxResults, got %d", len(searchResult.Matches))
	}
}

func TestServer_SearchByField_Cache(t *testing.T) {
	server, _ := createTestServer()

	args := map[string]interface{}{
		"field":   "owners",
		"value":   "team-monitoring@redhat.com",
		"version": "4.21",
	}

	// First call - should populate cache
	result1, err := server.searchByField(context.Background(), args)
	if err != nil {
		t.Fatalf("First searchByField() error = %v", err)
	}

	// Second call - should use cache
	result2, err := server.searchByField(context.Background(), args)
	if err != nil {
		t.Fatalf("Second searchByField() error = %v", err)
	}

	// Results should be identical
	var searchResult1, searchResult2 shared.FieldSearchResult
	err = shared.ParseJSONResult(result1, &searchResult1)
	if err != nil {
		t.Fatalf("Failed to parse first result: %v", err)
	}

	err = shared.ParseJSONResult(result2, &searchResult2)
	if err != nil {
		t.Fatalf("Failed to parse second result: %v", err)
	}

	if searchResult1.TotalFound != searchResult2.TotalFound {
		t.Errorf("Cache results differ: %d vs %d", searchResult1.TotalFound, searchResult2.TotalFound)
	}
}
