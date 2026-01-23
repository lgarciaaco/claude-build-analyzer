package shared

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLParser provides flexible YAML parsing utilities
type YAMLParser struct{}

// NewYAMLParser creates a new YAML parser
func NewYAMLParser() *YAMLParser {
	return &YAMLParser{}
}

// ParseComponentConfig parses a component YAML file into ComponentConfig
// Uses flexible parsing to handle various YAML structures
func (p *YAMLParser) ParseComponentConfig(data []byte, filename string) (*ComponentConfig, error) {
	// First parse into a flexible map
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	config := &ComponentConfig{}

	// Extract name from filename if not in YAML
	if name, exists := raw["name"]; exists {
		if nameStr, ok := name.(string); ok {
			config.Name = nameStr
		}
	} else {
		// Extract from filename: cluster-etcd-operator.yml -> cluster-etcd-operator
		parts := strings.Split(filename, ".")
		if len(parts) > 0 {
			config.Name = parts[0]
		}
	}

	// Parse enabled field
	if enabled, exists := raw["enabled"]; exists {
		if enabledBool, ok := enabled.(bool); ok {
			config.Enabled = &enabledBool
		}
	}

	// Parse arches
	if arches, exists := raw["arches"]; exists {
		if archList, ok := arches.([]interface{}); ok {
			for _, arch := range archList {
				if archStr, ok := arch.(string); ok {
					config.Arches = append(config.Arches, archStr)
				}
			}
		}
	}

	// Parse content section
	if content, exists := raw["content"]; exists {
		if contentMap, ok := content.(map[string]interface{}); ok {
			config.Content = p.parseContentConfig(contentMap)
		}
	}

	// Parse from section
	if from, exists := raw["from"]; exists {
		if fromMap, ok := from.(map[string]interface{}); ok {
			config.From = p.parseFromConfig(fromMap)
		}
	}

	// Parse konflux section
	if konflux, exists := raw["konflux"]; exists {
		if konfluxMap, ok := konflux.(map[string]interface{}); ok {
			config.Konflux = p.parseKonfluxConfig(konfluxMap)
		}
	}

	// Parse labels
	if labels, exists := raw["labels"]; exists {
		if labelsMap, ok := labels.(map[string]interface{}); ok {
			config.Labels = make(map[string]string)
			for k, v := range labelsMap {
				if vStr, ok := v.(string); ok {
					config.Labels[k] = vStr
				}
			}
		}
	}

	return config, nil
}

// parseContentConfig parses the content section
func (p *YAMLParser) parseContentConfig(contentMap map[string]interface{}) *ContentConfig {
	config := &ContentConfig{}

	if source, exists := contentMap["source"]; exists {
		if sourceMap, ok := source.(map[string]interface{}); ok {
			config.Source = p.parseSourceConfig(sourceMap)
		}
	}

	return config
}

// parseSourceConfig parses the source section
func (p *YAMLParser) parseSourceConfig(sourceMap map[string]interface{}) *SourceConfig {
	config := &SourceConfig{}

	if git, exists := sourceMap["git"]; exists {
		if gitMap, ok := git.(map[string]interface{}); ok {
			config.Git = p.parseGitConfig(gitMap)
		}
	}

	return config
}

// parseGitConfig parses the git section
func (p *YAMLParser) parseGitConfig(gitMap map[string]interface{}) *GitConfig {
	config := &GitConfig{}

	if url, exists := gitMap["url"]; exists {
		if urlStr, ok := url.(string); ok {
			config.URL = urlStr
		}
	}

	if branch, exists := gitMap["branch"]; exists {
		if branchMap, ok := branch.(map[string]interface{}); ok {
			config.Branch = p.parseBranchConfig(branchMap)
		}
	}

	return config
}

// parseBranchConfig parses the branch section
func (p *YAMLParser) parseBranchConfig(branchMap map[string]interface{}) *BranchConfig {
	config := &BranchConfig{}

	if target, exists := branchMap["target"]; exists {
		if targetStr, ok := target.(string); ok {
			config.Target = targetStr
		}
	}

	return config
}

// parseFromConfig parses the from section
func (p *YAMLParser) parseFromConfig(fromMap map[string]interface{}) *FromConfig {
	config := &FromConfig{}

	if builder, exists := fromMap["builder"]; exists {
		if builderList, ok := builder.([]interface{}); ok {
			for _, b := range builderList {
				if bStr, ok := b.(string); ok {
					config.Builder = append(config.Builder, bStr)
				}
			}
		}
	}

	if member, exists := fromMap["member"]; exists {
		if memberStr, ok := member.(string); ok {
			config.Member = memberStr
		}
	}

	return config
}

// parseKonfluxConfig parses the konflux section
func (p *YAMLParser) parseKonfluxConfig(konfluxMap map[string]interface{}) *KonfluxConfig {
	config := &KonfluxConfig{}

	if networkMode, exists := konfluxMap["network_mode"]; exists {
		if networkModeStr, ok := networkMode.(string); ok {
			config.NetworkMode = networkModeStr
		}
	}

	if cachi2, exists := konfluxMap["cachi2"]; exists {
		if cachi2Map, ok := cachi2.(map[string]interface{}); ok {
			config.Cachi2 = p.parseCachi2Config(cachi2Map)
		}
	}

	return config
}

// parseCachi2Config parses the cachi2 section
func (p *YAMLParser) parseCachi2Config(cachi2Map map[string]interface{}) *Cachi2Config {
	config := &Cachi2Config{}

	if lockfile, exists := cachi2Map["lockfile"]; exists {
		if lockfileMap, ok := lockfile.(map[string]interface{}); ok {
			config.Lockfile = p.parseLockfileConfig(lockfileMap)
		}
	}

	return config
}

// parseLockfileConfig parses the lockfile section
func (p *YAMLParser) parseLockfileConfig(lockfileMap map[string]interface{}) *LockfileConfig {
	config := &LockfileConfig{}

	if rpms, exists := lockfileMap["rpms"]; exists {
		if rpmsList, ok := rpms.([]interface{}); ok {
			for _, rpm := range rpmsList {
				if rpmStr, ok := rpm.(string); ok {
					config.RPMs = append(config.RPMs, rpmStr)
				}
			}
		}
	}

	return config
}

// ParseGroupConfig parses a group.yml file
func (p *YAMLParser) ParseGroupConfig(data []byte) (*GroupConfig, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse group YAML: %w", err)
	}

	config := &GroupConfig{}

	// Parse default network mode from konflux section
	if konflux, exists := raw["konflux"]; exists {
		if konfluxMap, ok := konflux.(map[string]interface{}); ok {
			if networkMode, exists := konfluxMap["network_mode"]; exists {
				if networkModeStr, ok := networkMode.(string); ok {
					config.DefaultNetworkMode = networkModeStr
				}
			}
		}
	}

	// Parse arches
	if arches, exists := raw["arches"]; exists {
		if archList, ok := arches.([]interface{}); ok {
			for _, arch := range archList {
				if archStr, ok := arch.(string); ok {
					config.Arches = append(config.Arches, archStr)
				}
			}
		}
	}

	return config, nil
}

// SafeStringValue safely extracts a string value from an interface{}
func SafeStringValue(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", val)
	}
}

// SafeBoolValue safely extracts a bool value from an interface{}
func SafeBoolValue(v interface{}) bool {
	if v == nil {
		return false
	}

	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.ToLower(val) == "true"
	default:
		return false
	}
}

// SafeSliceValue safely extracts a string slice from an interface{}
func SafeSliceValue(v interface{}) []string {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Slice {
		return nil
	}

	var result []string
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i).Interface()
		if str := SafeStringValue(item); str != "" {
			result = append(result, str)
		}
	}

	return result
}

// ParseFullYAML parses YAML data into a map for field searching
func (p *YAMLParser) ParseFullYAML(data []byte) (map[string]interface{}, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return raw, nil
}

// SearchFieldPath searches for a field value using dot notation (e.g., "owners", "labels.io.k8s.description")
func (p *YAMLParser) SearchFieldPath(data map[string]interface{}, fieldPath string) (interface{}, bool) {
	parts := strings.Split(fieldPath, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return nil, false
		}

		// Handle array notation like "owners[*]"
		if strings.Contains(part, "[") {
			arrayName := part[:strings.Index(part, "[")]
			if val, exists := current[arrayName]; exists {
				if i == len(parts)-1 {
					// This is the last part, return the array
					return val, true
				}
				// For nested access in arrays, we'd need more complex logic
				// For now, just return the array if it's the target field
			}
			return nil, false
		}

		if val, exists := current[part]; exists {
			if i == len(parts)-1 {
				// This is the final part
				return val, true
			}
			// Continue traversing
			if nextMap, ok := val.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return nil, false
			}
		} else {
			// If this is not the first part, try constructing the remaining path as a single key
			// This handles cases like "labels.io.k8s.description" where "io.k8s.description" is a single key
			if i > 0 {
				remainingPath := strings.Join(parts[i:], ".")
				if val, exists := current[remainingPath]; exists {
					return val, true
				}
			}
			return nil, false
		}
	}

	return nil, false
}

// MatchFieldValue checks if a field value matches the search criteria
func (p *YAMLParser) MatchFieldValue(fieldValue interface{}, searchValue string, caseSensitive bool) (string, bool) {
	if fieldValue == nil {
		return "", false
	}

	searchLower := strings.ToLower(searchValue)
	if !caseSensitive {
		searchValue = searchLower
	}

	switch v := fieldValue.(type) {
	case string:
		compareValue := v
		if !caseSensitive {
			compareValue = strings.ToLower(v)
		}
		
		// Exact match
		if compareValue == searchValue {
			return "exact", true
		}
		
		// Partial match
		if strings.Contains(compareValue, searchValue) {
			return "partial", true
		}

	case []interface{}:
		// Array contains match
		for _, item := range v {
			if itemStr, ok := item.(string); ok {
				compareValue := itemStr
				if !caseSensitive {
					compareValue = strings.ToLower(itemStr)
				}
				
				if compareValue == searchValue || strings.Contains(compareValue, searchValue) {
					return "array_contains", true
				}
			}
		}

	case []string:
		// String array contains match
		for _, item := range v {
			compareValue := item
			if !caseSensitive {
				compareValue = strings.ToLower(item)
			}
			
			if compareValue == searchValue || strings.Contains(compareValue, searchValue) {
				return "array_contains", true
			}
		}

	default:
		// Convert other types to string and check
		strValue := fmt.Sprintf("%v", v)
		compareValue := strValue
		if !caseSensitive {
			compareValue = strings.ToLower(strValue)
		}
		
		if compareValue == searchValue || strings.Contains(compareValue, searchValue) {
			return "partial", true
		}
	}

	return "", false
}
