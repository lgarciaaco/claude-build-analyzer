package metadataserver

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lgarciaaco/claude-build-analyzer/internal/git"
	"github.com/lgarciaaco/claude-build-analyzer/pkg/shared"
)

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// cacheEntry represents a cached metadata entry with expiration
type cacheEntry struct {
	data      []byte
	timestamp time.Time
}

// metadataCache provides LRU cache for component metadata with TTL expiration
type metadataCache struct {
	cache    map[string]*cacheEntry
	keyOrder []string
	maxSize  int
	ttl      time.Duration
	mu       sync.RWMutex
}

// newMetadataCache creates a new metadata cache
func newMetadataCache(maxSize int, ttl time.Duration) *metadataCache {
	return &metadataCache{
		cache:    make(map[string]*cacheEntry),
		keyOrder: make([]string, 0, maxSize),
		maxSize:  maxSize,
		ttl:      ttl,
	}
}

// get retrieves data from cache if not expired
func (mc *metadataCache) get(key string) ([]byte, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	entry, exists := mc.cache[key]
	if !exists {
		return nil, false
	}
	
	// Check TTL expiration
	if time.Since(entry.timestamp) > mc.ttl {
		return nil, false
	}
	
	return entry.data, true
}

// put stores data in cache with LRU eviction
func (mc *metadataCache) put(key string, data []byte) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Update existing entry
	if entry, exists := mc.cache[key]; exists {
		entry.data = data
		entry.timestamp = time.Now()
		return
	}
	
	// Add new entry
	mc.cache[key] = &cacheEntry{
		data:      data,
		timestamp: time.Now(),
	}
	mc.keyOrder = append(mc.keyOrder, key)
	
	// Evict oldest if over capacity
	if len(mc.cache) > mc.maxSize {
		oldestKey := mc.keyOrder[0]
		delete(mc.cache, oldestKey)
		mc.keyOrder = mc.keyOrder[1:]
	}
}

// Server implements the OCP Metadata MCP server
type Server struct {
	*shared.BaseMCPServer
	gitManager shared.GitManager
	yamlParser *shared.YAMLParser
	cache      *metadataCache
	ctx        context.Context
}

// NewServer creates a new metadata server
func NewServer(ctx context.Context) (*Server, error) {

	// Initialize Git manager for metadata repositories
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	dataPath := filepath.Join(homeDir, ".claude-build-analyzer", "ocp-build-data")
	branches := []string{
		"openshift-4.21",
		"openshift-4.20",
		"openshift-4.19",
		"openshift-4.18",
		"openshift-4.17",
		"openshift-4.16",
		"openshift-4.15",
		"openshift-4.14",
		"openshift-4.13",
		"openshift-4.12",
	}

	gitManager := git.NewManager(
		"https://github.com/openshift-eng/ocp-build-data.git",
		dataPath,
		branches,
	)

	server := &Server{
		gitManager: gitManager,
		yamlParser: shared.NewYAMLParser(),
		cache:      newMetadataCache(1000, 15*time.Minute), // Cache 1000 items for 15 minutes
		ctx:        ctx,
	}

	// Create base server with tool list function
	server.BaseMCPServer = shared.NewBaseMCPServer("ocp-metadata-server", "1.0.0", ctx, server.GetToolList)

	// Register tools
	server.RegisterTool("get_component_metadata", server.getComponentMetadata)
	server.RegisterTool("search_components", server.searchComponents)
	server.RegisterTool("search_by_field", server.searchByField)

	return server, nil
}

// Initialize initializes the metadata server
func (s *Server) Initialize() error {
	log.Println("Initializing OCP Metadata Server...")

	// Initialize git repositories
	if err := s.gitManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize git manager: %w", err)
	}

	log.Println("OCP Metadata Server initialized successfully")
	return nil
}

// GetToolList returns the list of available tools
func (s *Server) GetToolList() shared.ToolList {
	return shared.ToolList{
		Tools: []shared.Tool{
			{
				Name:        "get_component_metadata",
				Description: "Get component configuration metadata from ocp-build-data for failure correlation analysis",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"componentName": {
							Type:        "string",
							Description: "Component name to look up (e.g., 'ironic', 'oauth-server')",
						},
						"version": {
							Type:        "string",
							Description: "OpenShift version (e.g., '4.21', '4.20'). Defaults to '4.21'",
						},
					},
					Required: []string{"componentName"},
				},
			},
			{
				Name:        "search_components",
				Description: "Search for components by name pattern using fuzzy matching to find exact component names",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"query": {
							Type:        "string",
							Description: "Search query or pattern (e.g., 'ironic', 'etcd', 'oauth')",
						},
						"version": {
							Type:        "string",
							Description: "OpenShift version (e.g., '4.21', '4.20'). Defaults to '4.21'",
						},
						"maxResults": {
							Type:        "number",
							Description: "Maximum number of results to return (default: 10)",
						},
					},
					Required: []string{"query"},
				},
			},
			{
				Name:        "search_by_field",
				Description: "Search components by any YAML field value (e.g., owners, labels, etc.) with dot-notation support",
				InputSchema: shared.InputSchema{
					Type: "object",
					Properties: map[string]shared.Property{
						"field": {
							Type:        "string",
							Description: "Field path using dot notation (e.g., 'owners', 'labels.io.k8s.description', 'content.source.git.url')",
						},
						"value": {
							Type:        "string",
							Description: "Value to search for in the field (supports partial matching)",
						},
						"version": {
							Type:        "string",
							Description: "OpenShift version (e.g., '4.21', '4.20'). Defaults to '4.21'",
						},
						"maxResults": {
							Type:        "number",
							Description: "Maximum number of results to return (default: 50)",
						},
						"caseSensitive": {
							Type:        "boolean",
							Description: "Whether to perform case-sensitive matching (default: false)",
						},
					},
					Required: []string{"field", "value"},
				},
			},
		},
	}
}

// Close closes the server resources
func (s *Server) Close() error {
	// No resources to close for metadata server
	return nil
}

// getComponentMetadata handles the get_component_metadata tool
func (s *Server) getComponentMetadata(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	componentName, ok := args["componentName"].(string)
	if !ok {
		return shared.ToolResult{}, fmt.Errorf("componentName is required")
	}

	version := "4.21"
	if v, ok := args["version"].(string); ok {
		version = v
	}

	// Ensure version has proper prefix
	if !strings.HasPrefix(version, "openshift-") {
		version = "openshift-" + version
	}

	// Create cache key for component metadata
	cacheKey := fmt.Sprintf("%s:%s", version, componentName)
	
	// Check cache first
	if cachedData, found := s.cache.get(cacheKey); found {
		result := map[string]interface{}{
			"component": componentName,
			"version":   version,
			"config":    string(cachedData),
		}
		return shared.FormatJSONResult(result)
	}

	// Get repository for the specified version
	repo, err := s.gitManager.GetRepository(version)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to get repository for version %s: %w", version, err)
	}

	// Try to find the component in images/ directory
	componentPath := fmt.Sprintf("images/%s.yml", componentName)
	data, err := repo.GetFile(componentPath)
	if err != nil {
		// Try alternative naming patterns
		alternatives := []string{
			fmt.Sprintf("images/ose-%s.yml", componentName),
			fmt.Sprintf("images/openshift-%s.yml", componentName),
			fmt.Sprintf("images/openshift-enterprise-%s.yml", componentName),
			fmt.Sprintf("images/cluster-%s.yml", componentName),
		}

		for _, alt := range alternatives {
			if altData, altErr := repo.GetFile(alt); altErr == nil {
				data = altData
				componentPath = alt
				err = nil
				break
			}
		}

		if err != nil {
			return shared.ToolResult{}, fmt.Errorf("component '%s' not found in version %s", componentName, version)
		}
	}

	// Parse the component configuration
	config, err := s.yamlParser.ParseComponentConfig(data, filepath.Base(componentPath))
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to parse component config: %w", err)
	}

	// Get group configuration for defaults
	groupData, err := repo.GetFile("group.yml")
	var groupConfig *shared.GroupConfig
	if err == nil {
		groupConfig, err = s.yamlParser.ParseGroupConfig(groupData)
		if err != nil {
			log.Printf("Warning: failed to parse group config: %v", err)
		}
	}

	// Determine effective network mode
	networkMode := "hermetic" // default
	if config.Konflux != nil && config.Konflux.NetworkMode != "" {
		networkMode = config.Konflux.NetworkMode
	} else if groupConfig != nil && groupConfig.DefaultNetworkMode != "" {
		networkMode = groupConfig.DefaultNetworkMode
	}
	config.NetworkMode = networkMode

	// Add analysis information
	result := map[string]interface{}{
		"component": config,
		"analysis": map[string]interface{}{
			"version":              version,
			"effectiveNetworkMode": networkMode,
			"isHermetic":           networkMode == "hermetic",
			"hasLockfile":          config.Konflux != nil && config.Konflux.Cachi2 != nil && config.Konflux.Cachi2.Lockfile != nil,
			"needsConversion":      networkMode == "open",
		},
	}

	if groupConfig != nil {
		result["groupDefaults"] = groupConfig
	}

	// Cache the raw component data for future requests
	s.cache.put(cacheKey, data)

	return shared.FormatJSONResult(result)
}

// searchComponents handles the search_components tool
func (s *Server) searchComponents(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok {
		return shared.ToolResult{}, fmt.Errorf("query is required")
	}

	version := "4.21"
	if v, ok := args["version"].(string); ok {
		version = v
	}

	maxResults := 10
	if mr, ok := args["maxResults"].(float64); ok {
		maxResults = int(mr)
	}

	// Ensure version has proper prefix
	if !strings.HasPrefix(version, "openshift-") {
		version = "openshift-" + version
	}

	// Get repository for the specified version
	repo, err := s.gitManager.GetRepository(version)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to get repository for version %s: %w", version, err)
	}

	// List all image files
	imageFiles, err := repo.ListFiles("images/*.yml")
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to list image files: %w", err)
	}

	log.Printf("Debug: Found %d image files for version %s", len(imageFiles), version)
	if len(imageFiles) > 0 {
		log.Printf("Debug: First few files: %v", imageFiles[:minInt(5, len(imageFiles))])
	}

	// Score and rank matches
	var matches []shared.ComponentMatch
	queryLower := strings.ToLower(query)

	for _, file := range imageFiles {
		filename := filepath.Base(file)
		componentName := strings.TrimSuffix(filename, ".yml")
		componentLower := strings.ToLower(componentName)

		score := s.scoreMatch(queryLower, componentLower, componentName)
		if score > 0 {
			matches = append(matches, shared.ComponentMatch{
				Name:  componentName,
				Score: score,
			})
		}
	}

	// Sort by score (highest first) and limit results
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].Score < matches[j].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	result := map[string]interface{}{
		"query":      query,
		"version":    version,
		"found":      len(matches),
		"components": matches,
	}

	return shared.FormatJSONResult(result)
}

// scoreMatch scores how well a component name matches a query
func (s *Server) scoreMatch(queryLower, componentLower, originalComponent string) int {
	// Exact match
	if queryLower == componentLower {
		return 1000
	}

	// Exact prefix match
	if strings.HasPrefix(componentLower, queryLower) {
		return 800
	}

	// Word boundary match (e.g., "etcd" matches "cluster-etcd-operator")
	if strings.Contains(componentLower, "-"+queryLower+"-") ||
		strings.Contains(componentLower, "-"+queryLower) ||
		strings.HasSuffix(componentLower, "-"+queryLower) {
		return 750
	}

	// Contains match
	if strings.Contains(componentLower, queryLower) {
		return 600
	}

	// Partial word matches
	queryWords := strings.Split(queryLower, "-")
	componentWords := strings.Split(componentLower, "-")

	matchCount := 0
	for _, qw := range queryWords {
		for _, cw := range componentWords {
			if qw == cw {
				matchCount++
				break
			}
		}
	}

	if matchCount > 0 {
		return 400 + (matchCount * 50)
	}

	return 0
}

// searchByField handles the search_by_field tool
func (s *Server) searchByField(ctx context.Context, args map[string]interface{}) (shared.ToolResult, error) {
	field, ok := args["field"].(string)
	if !ok {
		return shared.ToolResult{}, fmt.Errorf("field is required")
	}

	value, ok := args["value"].(string)
	if !ok {
		return shared.ToolResult{}, fmt.Errorf("value is required")
	}

	version := "4.21"
	if v, ok := args["version"].(string); ok {
		version = v
	}

	maxResults := 50
	if mr, ok := args["maxResults"].(float64); ok {
		maxResults = int(mr)
	}

	caseSensitive := false
	if cs, ok := args["caseSensitive"].(bool); ok {
		caseSensitive = cs
	}

	// Ensure version has proper prefix
	if !strings.HasPrefix(version, "openshift-") {
		version = "openshift-" + version
	}

	// Get repository for the specified version
	repo, err := s.gitManager.GetRepository(version)
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to get repository for version %s: %w", version, err)
	}

	// List all image files
	imageFiles, err := repo.ListFiles("images/*.yml")
	if err != nil {
		return shared.ToolResult{}, fmt.Errorf("failed to list image files: %w", err)
	}

	log.Printf("Debug: Searching field '%s' for value '%s' in %d files", field, value, len(imageFiles))

	var allMatches []shared.FieldMatch

	// Search through each file
	for _, file := range imageFiles {
		// Create cache key for raw YAML
		cacheKey := fmt.Sprintf("%s:%s:raw", version, file)
		
		var data []byte
		var found bool
		
		// Check cache first
		if cachedData, cacheFound := s.cache.get(cacheKey); cacheFound {
			data = cachedData
			found = true
		} else {
			// Read file from repository
			data, err = repo.GetFile(file)
			if err != nil {
				log.Printf("Warning: failed to read file %s: %v", file, err)
				continue
			}
			
			// Cache the raw data
			s.cache.put(cacheKey, data)
			found = true
		}

		if !found {
			continue
		}

		// Parse YAML into full structure
		yamlData, err := s.yamlParser.ParseFullYAML(data)
		if err != nil {
			log.Printf("Warning: failed to parse YAML for file %s: %v", file, err)
			continue
		}

		// Search for the field
		fieldValue, fieldExists := s.yamlParser.SearchFieldPath(yamlData, field)
		if !fieldExists {
			continue
		}

		// Check if the value matches
		matchType, valueMatches := s.yamlParser.MatchFieldValue(fieldValue, value, caseSensitive)
		if !valueMatches {
			continue
		}

		// Extract component name from filename
		filename := filepath.Base(file)
		componentName := strings.TrimSuffix(filename, ".yml")

		// Add to matches
		match := shared.FieldMatch{
			ComponentName: componentName,
			FilePath:      file,
			Field:         field,
			Value:         fieldValue,
			MatchType:     matchType,
		}

		allMatches = append(allMatches, match)

		// Limit results
		if len(allMatches) >= maxResults {
			break
		}
	}

	result := shared.FieldSearchResult{
		Field:       field,
		SearchValue: value,
		Version:     version,
		TotalFound:  len(allMatches),
		Matches:     allMatches,
	}

	return shared.FormatJSONResult(result)
}
