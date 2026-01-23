package shared

import (
	"time"
)

// GitRepository defines the interface for git repository operations
type GitRepository interface {
	Initialize() error
	GetFile(path string) ([]byte, error)
	ListFiles(pattern string) ([]string, error)
	GetCommitHistory(path string, limit int) ([]Commit, error)
	GetBranch() string
	Update() error
}

// GitManager manages multiple git repositories for different branches
type GitManager interface {
	Initialize() error
	GetRepository(branch string) (GitRepository, error)
	GetAllBranches() []string
	UpdateAll() error
}

// Commit represents a git commit
type Commit struct {
	Hash    string    `json:"hash"`
	Author  string    `json:"author"`
	Email   string    `json:"email"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
	Files   []string  `json:"files,omitempty"`
}

// ComponentConfig represents a parsed component configuration
type ComponentConfig struct {
	Name        string            `json:"name"`
	Enabled     *bool             `json:"enabled,omitempty"`
	NetworkMode string            `json:"network_mode,omitempty"`
	SourceRepo  string            `json:"source_repo,omitempty"`
	Branch      string            `json:"branch,omitempty"`
	Arches      []string          `json:"arches,omitempty"`
	Konflux     *KonfluxConfig    `json:"konflux,omitempty"`
	Content     *ContentConfig    `json:"content,omitempty"`
	From        *FromConfig       `json:"from,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// KonfluxConfig represents Konflux-specific configuration
type KonfluxConfig struct {
	NetworkMode string        `json:"network_mode,omitempty"`
	Cachi2      *Cachi2Config `json:"cachi2,omitempty"`
}

// Cachi2Config represents cachi2 lockfile configuration
type Cachi2Config struct {
	Lockfile *LockfileConfig `json:"lockfile,omitempty"`
}

// LockfileConfig represents lockfile configuration
type LockfileConfig struct {
	RPMs []string `json:"rpms,omitempty"`
}

// ContentConfig represents content configuration
type ContentConfig struct {
	Source *SourceConfig `json:"source,omitempty"`
}

// SourceConfig represents source configuration
type SourceConfig struct {
	Git *GitConfig `json:"git,omitempty"`
}

// GitConfig represents git source configuration
type GitConfig struct {
	URL    string        `json:"url,omitempty"`
	Branch *BranchConfig `json:"branch,omitempty"`
}

// BranchConfig represents branch configuration
type BranchConfig struct {
	Target string `json:"target,omitempty"`
}

// FromConfig represents from configuration
type FromConfig struct {
	Builder []string `json:"builder,omitempty"`
	Member  string   `json:"member,omitempty"`
}

// ComponentMatch represents a component search result
type ComponentMatch struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

// GroupConfig represents group-level configuration
type GroupConfig struct {
	DefaultNetworkMode string   `json:"default_network_mode,omitempty"`
	Arches             []string `json:"arches,omitempty"`
}

// FieldMatch represents a single field match result
type FieldMatch struct {
	ComponentName string      `json:"component_name"`
	FilePath      string      `json:"file_path"`
	Field         string      `json:"field"`
	Value         interface{} `json:"value"`
	MatchType     string      `json:"match_type"` // "exact", "partial", "array_contains"
}

// FieldSearchResult represents the result of a field search operation
type FieldSearchResult struct {
	Field       string       `json:"field"`
	SearchValue string       `json:"search_value"`
	Version     string       `json:"version"`
	TotalFound  int          `json:"total_found"`
	Matches     []FieldMatch `json:"matches"`
}
