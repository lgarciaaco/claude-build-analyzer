package shared

import (
	"testing"
)

func TestYAMLParser_ParseComponentConfig(t *testing.T) {
	parser := NewYAMLParser()

	testYAML := `
name: ironic
enabled: true
arches:
  - x86_64
  - aarch64
content:
  source:
    git:
      url: git@github.com:openshift-priv/ironic-image.git
      branch:
        target: release-{MAJOR}.{MINOR}
from:
  builder:
    - rhel-9-golang
  member: openshift-enterprise-base-rhel9
konflux:
  network_mode: open
  cachi2:
    lockfile:
      rpms:
        - python3-requests
labels:
  component: ironic
`

	config, err := parser.ParseComponentConfig([]byte(testYAML), "ironic.yml")
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if config.Name != "ironic" {
		t.Errorf("Expected name 'ironic', got '%s'", config.Name)
	}

	if config.Enabled == nil || !*config.Enabled {
		t.Error("Expected enabled to be true")
	}

	if len(config.Arches) != 2 {
		t.Errorf("Expected 2 arches, got %d", len(config.Arches))
	}

	if config.Content == nil || config.Content.Source == nil || config.Content.Source.Git == nil {
		t.Error("Expected git configuration")
	}

	if config.Konflux == nil || config.Konflux.NetworkMode != "open" {
		t.Error("Expected network_mode 'open'")
	}

	if config.Konflux.Cachi2 == nil || config.Konflux.Cachi2.Lockfile == nil {
		t.Error("Expected cachi2 lockfile configuration")
	}

	if len(config.Konflux.Cachi2.Lockfile.RPMs) != 1 {
		t.Errorf("Expected 1 RPM, got %d", len(config.Konflux.Cachi2.Lockfile.RPMs))
	}
}

func TestYAMLParser_ParseGroupConfig(t *testing.T) {
	parser := NewYAMLParser()

	testYAML := `
konflux:
  network_mode: hermetic
arches:
  - x86_64
  - aarch64
  - ppc64le
  - s390x
`

	config, err := parser.ParseGroupConfig([]byte(testYAML))
	if err != nil {
		t.Fatalf("Failed to parse group YAML: %v", err)
	}

	if config.DefaultNetworkMode != "hermetic" {
		t.Errorf("Expected default network mode 'hermetic', got '%s'", config.DefaultNetworkMode)
	}

	if len(config.Arches) != 4 {
		t.Errorf("Expected 4 arches, got %d", len(config.Arches))
	}
}

func TestYAMLParser_ParseComponentConfig_NameFromFilename(t *testing.T) {
	parser := NewYAMLParser()

	testYAML := `
enabled: true
`

	config, err := parser.ParseComponentConfig([]byte(testYAML), "cluster-etcd-operator.yml")
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if config.Name != "cluster-etcd-operator" {
		t.Errorf("Expected name from filename 'cluster-etcd-operator', got '%s'", config.Name)
	}
}

func TestYAMLParser_ParseComponentConfig_InvalidYAML(t *testing.T) {
	parser := NewYAMLParser()

	invalidYAML := `
name: ironic
enabled: [invalid yaml structure
`

	_, err := parser.ParseComponentConfig([]byte(invalidYAML), "ironic.yml")
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestSafeStringValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string value", "hello", "hello"},
		{"nil value", nil, ""},
		{"int value", 42, "42"},
		{"bool value", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeStringValue(tt.input)
			if result != tt.expected {
				t.Errorf("SafeStringValue(%v) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSafeBoolValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string TRUE", "TRUE", true},
		{"string false", "false", false},
		{"nil value", nil, false},
		{"int value", 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeBoolValue(tt.input)
			if result != tt.expected {
				t.Errorf("SafeBoolValue(%v) = %t, expected %t", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSafeSliceValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{"string slice", []interface{}{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"mixed slice", []interface{}{"a", 42, "b"}, []string{"a", "42", "b"}},
		{"nil value", nil, nil},
		{"non-slice", "string", nil},
		{"empty slice", []interface{}{}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeSliceValue(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("SafeSliceValue(%v) length = %d, expected %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("SafeSliceValue(%v)[%d] = %s, expected %s", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

// Tests for new field search functionality

func TestYAMLParser_ParseFullYAML(t *testing.T) {
	parser := NewYAMLParser()

	testYAML := `
name: test-component
owners:
  - team-monitoring@redhat.com
  - another-team@redhat.com
labels:
  io.k8s.description: "Test component for monitoring"
  license: "Apache 2.0"
content:
  source:
    git:
      url: git@github.com:example/repo.git
nested:
  level1:
    level2:
      value: "deep value"
`

	data, err := parser.ParseFullYAML([]byte(testYAML))
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Verify top-level fields
	if name, ok := data["name"].(string); !ok || name != "test-component" {
		t.Errorf("Expected name 'test-component', got %v", data["name"])
	}

	// Verify owners array
	if owners, ok := data["owners"].([]interface{}); !ok {
		t.Errorf("Expected owners to be array, got %T", data["owners"])
	} else if len(owners) != 2 {
		t.Errorf("Expected 2 owners, got %d", len(owners))
	}

	// Verify nested structure
	if content, ok := data["content"].(map[string]interface{}); !ok {
		t.Errorf("Expected content to be map, got %T", data["content"])
	} else if source, ok := content["source"].(map[string]interface{}); !ok {
		t.Errorf("Expected source to be map, got %T", content["source"])
	} else if git, ok := source["git"].(map[string]interface{}); !ok {
		t.Errorf("Expected git to be map, got %T", source["git"])
	} else if url, ok := git["url"].(string); !ok || url != "git@github.com:example/repo.git" {
		t.Errorf("Expected URL 'git@github.com:example/repo.git', got %v", git["url"])
	}
}

func TestYAMLParser_SearchFieldPath(t *testing.T) {
	parser := NewYAMLParser()

	data := map[string]interface{}{
		"name": "test-component",
		"owners": []interface{}{
			"team-monitoring@redhat.com",
			"another-team@redhat.com",
		},
		"labels": map[string]interface{}{
			"io.k8s.description": "Test component",
			"license":            "Apache 2.0",
		},
		"content": map[string]interface{}{
			"source": map[string]interface{}{
				"git": map[string]interface{}{
					"url": "git@github.com:example/repo.git",
				},
			},
		},
		"nested": map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"value": "deep value",
				},
			},
		},
	}

	tests := []struct {
		name        string
		fieldPath   string
		expectFound bool
		expectValue interface{}
	}{
		{
			name:        "simple field",
			fieldPath:   "name",
			expectFound: true,
			expectValue: "test-component",
		},
		{
			name:        "array field",
			fieldPath:   "owners",
			expectFound: true,
			expectValue: []interface{}{"team-monitoring@redhat.com", "another-team@redhat.com"},
		},
		{
			name:        "nested field - labels.license",
			fieldPath:   "labels.license",
			expectFound: true,
			expectValue: "Apache 2.0",
		},
		{
			name:        "deep nested field",
			fieldPath:   "content.source.git.url",
			expectFound: true,
			expectValue: "git@github.com:example/repo.git",
		},
		{
			name:        "very deep nested field",
			fieldPath:   "nested.level1.level2.value",
			expectFound: true,
			expectValue: "deep value",
		},
		{
			name:        "non-existent field",
			fieldPath:   "nonexistent",
			expectFound: false,
			expectValue: nil,
		},
		{
			name:        "non-existent nested field",
			fieldPath:   "labels.nonexistent",
			expectFound: false,
			expectValue: nil,
		},
		{
			name:        "path through non-map",
			fieldPath:   "name.invalid.path",
			expectFound: false,
			expectValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := parser.SearchFieldPath(data, tt.fieldPath)
			if found != tt.expectFound {
				t.Errorf("SearchFieldPath(%s) found = %t, expected %t", tt.fieldPath, found, tt.expectFound)
			}
			if tt.expectFound && !deepEqual(value, tt.expectValue) {
				t.Errorf("SearchFieldPath(%s) value = %v, expected %v", tt.fieldPath, value, tt.expectValue)
			}
		})
	}
}

func TestYAMLParser_MatchFieldValue(t *testing.T) {
	parser := NewYAMLParser()

	tests := []struct {
		name          string
		fieldValue    interface{}
		searchValue   string
		caseSensitive bool
		expectMatch   bool
		expectType    string
	}{
		{
			name:          "exact string match",
			fieldValue:    "team-monitoring@redhat.com",
			searchValue:   "team-monitoring@redhat.com",
			caseSensitive: true,
			expectMatch:   true,
			expectType:    "exact",
		},
		{
			name:          "partial string match",
			fieldValue:    "team-monitoring@redhat.com",
			searchValue:   "monitoring",
			caseSensitive: true,
			expectMatch:   true,
			expectType:    "partial",
		},
		{
			name:          "case insensitive match",
			fieldValue:    "Team-Monitoring@RedHat.com",
			searchValue:   "team-monitoring@redhat.com",
			caseSensitive: false,
			expectMatch:   true,
			expectType:    "exact",
		},
		{
			name:          "case sensitive no match",
			fieldValue:    "Team-Monitoring@RedHat.com",
			searchValue:   "team-monitoring@redhat.com",
			caseSensitive: true,
			expectMatch:   false,
			expectType:    "",
		},
		{
			name: "array contains exact match",
			fieldValue: []interface{}{
				"team-monitoring@redhat.com",
				"another-team@redhat.com",
			},
			searchValue:   "team-monitoring@redhat.com",
			caseSensitive: true,
			expectMatch:   true,
			expectType:    "array_contains",
		},
		{
			name: "array contains partial match",
			fieldValue: []interface{}{
				"team-monitoring@redhat.com",
				"another-team@redhat.com",
			},
			searchValue:   "monitoring",
			caseSensitive: true,
			expectMatch:   true,
			expectType:    "array_contains",
		},
		{
			name: "string array contains match",
			fieldValue: []string{
				"team-monitoring@redhat.com",
				"another-team@redhat.com",
			},
			searchValue:   "monitoring",
			caseSensitive: true,
			expectMatch:   true,
			expectType:    "array_contains",
		},
		{
			name:          "no match in array",
			fieldValue:    []interface{}{"other@redhat.com", "different@redhat.com"},
			searchValue:   "monitoring",
			caseSensitive: true,
			expectMatch:   false,
			expectType:    "",
		},
		{
			name:          "integer to string match",
			fieldValue:    42,
			searchValue:   "42",
			caseSensitive: false,
			expectMatch:   true,
			expectType:    "partial",
		},
		{
			name:          "boolean to string match",
			fieldValue:    true,
			searchValue:   "true",
			caseSensitive: false,
			expectMatch:   true,
			expectType:    "partial",
		},
		{
			name:          "nil value no match",
			fieldValue:    nil,
			searchValue:   "anything",
			caseSensitive: false,
			expectMatch:   false,
			expectType:    "",
		},
		{
			name:          "empty string no match",
			fieldValue:    "",
			searchValue:   "something",
			caseSensitive: false,
			expectMatch:   false,
			expectType:    "",
		},
		{
			name:          "empty string exact match",
			fieldValue:    "",
			searchValue:   "",
			caseSensitive: false,
			expectMatch:   true,
			expectType:    "exact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchType, found := parser.MatchFieldValue(tt.fieldValue, tt.searchValue, tt.caseSensitive)
			if found != tt.expectMatch {
				t.Errorf("MatchFieldValue() found = %t, expected %t", found, tt.expectMatch)
			}
			if tt.expectMatch && matchType != tt.expectType {
				t.Errorf("MatchFieldValue() type = %s, expected %s", matchType, tt.expectType)
			}
		})
	}
}

func TestYAMLParser_SearchFieldPath_ArrayNotation(t *testing.T) {
	parser := NewYAMLParser()

	data := map[string]interface{}{
		"owners": []interface{}{
			"team-monitoring@redhat.com",
			"another-team@redhat.com",
		},
		"maintainers": []string{
			"user1@redhat.com",
			"user2@redhat.com",
		},
	}

	tests := []struct {
		name        string
		fieldPath   string
		expectFound bool
	}{
		{
			name:        "array with wildcard notation",
			fieldPath:   "owners[*]",
			expectFound: true,
		},
		{
			name:        "array with index notation",
			fieldPath:   "owners[0]",
			expectFound: true,
		},
		{
			name:        "non-existent array notation",
			fieldPath:   "nonexistent[*]",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found := parser.SearchFieldPath(data, tt.fieldPath)
			if found != tt.expectFound {
				t.Errorf("SearchFieldPath(%s) found = %t, expected %t", tt.fieldPath, found, tt.expectFound)
			}
		})
	}
}

func TestYAMLParser_RealWorldYAML(t *testing.T) {
	parser := NewYAMLParser()

	// Example of real OCP component YAML with owners
	realWorldYAML := `
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
`

	data, err := parser.ParseFullYAML([]byte(realWorldYAML))
	if err != nil {
		t.Fatalf("Failed to parse real world YAML: %v", err)
	}

	// Test searching for owners field
	owners, found := parser.SearchFieldPath(data, "owners")
	if !found {
		t.Error("Expected to find owners field")
	}

	// Test matching team-monitoring email
	matchType, matched := parser.MatchFieldValue(owners, "team-monitoring@redhat.com", false)
	if !matched {
		t.Error("Expected to match team-monitoring@redhat.com in owners")
	}
	if matchType != "array_contains" {
		t.Errorf("Expected match type 'array_contains', got '%s'", matchType)
	}

	// Test partial matching
	matchType, matched = parser.MatchFieldValue(owners, "monitoring", false)
	if !matched {
		t.Error("Expected to match 'monitoring' in owners")
	}
	if matchType != "array_contains" {
		t.Errorf("Expected match type 'array_contains', got '%s'", matchType)
	}

	// Test nested label search
	description, found := parser.SearchFieldPath(data, "labels.io.k8s.description")
	if !found {
		t.Error("Expected to find nested label description")
	}

	matchType, matched = parser.MatchFieldValue(description, "monitoring", false)
	if !matched {
		t.Error("Expected to match 'monitoring' in description")
	}
	if matchType != "partial" {
		t.Errorf("Expected match type 'partial', got '%s'", matchType)
	}
}

// Helper function for deep equality comparison
func deepEqual(a, b interface{}) bool {
	switch va := a.(type) {
	case []interface{}:
		if vb, ok := b.([]interface{}); ok {
			if len(va) != len(vb) {
				return false
			}
			for i := range va {
				if !deepEqual(va[i], vb[i]) {
					return false
				}
			}
			return true
		}
		return false
	case string:
		if vb, ok := b.(string); ok {
			return va == vb
		}
		return false
	default:
		return a == b
	}
}
