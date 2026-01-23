package git

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/lgarciaaco/claude-build-analyzer/pkg/shared"
)

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Manager handles git repository operations for multiple OpenShift versions
type Manager struct {
	repoURL       string
	baseLocalPath string
	versionsPath  string
	branches      []string
	repositories  map[string]*git.Repository
	mu            sync.RWMutex
}

// NewManager creates a new git repository manager
func NewManager(repoURL, baseLocalPath string, branches []string) *Manager {
	return &Manager{
		repoURL:       repoURL,
		baseLocalPath: baseLocalPath,
		versionsPath:  filepath.Join(baseLocalPath, "versions"),
		branches:      branches,
		repositories:  make(map[string]*git.Repository),
	}
}

// Initialize sets up all repository clones for different OpenShift versions
func (m *Manager) Initialize() error {
	log.Println("Initializing git repository manager...")

	// Create base directories
	if err := os.MkdirAll(m.versionsPath, 0755); err != nil {
		return fmt.Errorf("failed to create versions directory: %w", err)
	}

	// Clone or update repositories for each branch
	var wg sync.WaitGroup
	errors := make(chan error, len(m.branches))

	for _, branch := range m.branches {
		wg.Add(1)
		go func(b string) {
			defer wg.Done()
			if err := m.setupBranch(b); err != nil {
				errors <- fmt.Errorf("failed to setup branch %s: %w", b, err)
			}
		}(branch)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var setupErrors []string
	for err := range errors {
		setupErrors = append(setupErrors, err.Error())
		log.Printf("Setup error: %v", err)
	}

	if len(setupErrors) > 0 {
		log.Printf("Some branches failed to setup, continuing with available ones...")
	}

	log.Printf("Git repository manager initialized with %d branches", len(m.repositories))
	return nil
}

// setupBranch clones or opens a repository for a specific branch
func (m *Manager) setupBranch(branch string) error {
	version := m.extractVersion(branch)
	repoPath := filepath.Join(m.versionsPath, version)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if repository already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		// Repository exists, open it
		repo, err := git.PlainOpen(repoPath)
		if err != nil {
			return fmt.Errorf("failed to open existing repository at %s: %w", repoPath, err)
		}

		// Try to fetch latest changes
		if err := m.fetchRepository(repo); err != nil {
			log.Printf("Warning: failed to fetch updates for %s: %v", branch, err)
		}

		// Checkout the correct branch
		if err := m.checkoutBranch(repo, branch); err != nil {
			log.Printf("Warning: failed to checkout branch %s: %v", branch, err)
		}

		m.repositories[branch] = repo
		log.Printf("Opened existing repository for branch %s at %s", branch, repoPath)
		return nil
	}

	// Repository doesn't exist, clone it
	log.Printf("Cloning repository for branch %s to %s", branch, repoPath)

	cloneOptions := &git.CloneOptions{
		URL:           m.repoURL,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
		SingleBranch:  true,
		Depth:         1, // Shallow clone for faster setup
	}

	repo, err := git.PlainClone(repoPath, false, cloneOptions)
	if err != nil {
		// If single branch clone fails, try full clone and checkout
		log.Printf("Single branch clone failed for %s, trying full clone...", branch)

		fullCloneOptions := &git.CloneOptions{
			URL: m.repoURL,
		}

		repo, err = git.PlainClone(repoPath, false, fullCloneOptions)
		if err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		// Checkout the specific branch
		if err := m.checkoutBranch(repo, branch); err != nil {
			return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
		}
	}

	m.repositories[branch] = repo
	log.Printf("Successfully cloned repository for branch %s", branch)
	return nil
}

// fetchRepository fetches latest changes from remote
func (m *Manager) fetchRepository(repo *git.Repository) error {
	return repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
	})
}

// checkoutBranch checks out a specific branch
func (m *Manager) checkoutBranch(repo *git.Repository, branch string) error {
	workTree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Try to checkout the branch
	err = workTree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
	})
	if err != nil {
		// If local branch doesn't exist, create it from remote
		err = workTree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
			Create: true,
		})
	}

	return err
}

// extractVersion extracts version number from branch name (e.g., openshift-4.21 -> 4.21)
func (m *Manager) extractVersion(branch string) string {
	re := regexp.MustCompile(`openshift-(\d+\.\d+)`)
	matches := re.FindStringSubmatch(branch)
	if len(matches) > 1 {
		return matches[1]
	}
	// Fallback: clean the branch name
	return strings.ReplaceAll(strings.ReplaceAll(branch, "openshift-", ""), "/", "-")
}

// GetFileContent reads file content from a specific branch
func (m *Manager) GetFileContent(branch, filePath string) ([]byte, error) {
	m.mu.RLock()
	repo, exists := m.repositories[branch]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repository for branch %s not found", branch)
	}

	// Get the current commit
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	// Get the file tree
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit tree: %w", err)
	}

	// Get the file
	file, err := tree.File(filePath)
	if err != nil {
		return nil, fmt.Errorf("file %s not found in branch %s: %w", filePath, branch, err)
	}

	// Read file content
	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return []byte(content), nil
}

// ListFiles lists files in a directory for a specific branch
func (m *Manager) ListFiles(branch, dirPath string) ([]string, error) {
	m.mu.RLock()
	repo, exists := m.repositories[branch]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repository for branch %s not found", branch)
	}

	// Get the current commit
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	// Get the file tree
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit tree: %w", err)
	}

	var files []string

	// If dirPath is empty, list root directory
	if dirPath == "" {
		tree.Files().ForEach(func(f *object.File) error {
			if !strings.Contains(f.Name, "/") {
				files = append(files, f.Name)
			}
			return nil
		})
		return files, nil
	}

	// Get subdirectory
	subTree, err := tree.Tree(dirPath)
	if err != nil {
		// Directory might not exist, return empty list
		return files, nil
	}

	// List files in subdirectory
	subTree.Files().ForEach(func(f *object.File) error {
		// Remove directory prefix and only include direct files
		name := strings.TrimPrefix(f.Name, dirPath+"/")
		if !strings.Contains(name, "/") {
			files = append(files, name)
		}
		return nil
	})

	return files, nil
}

// FileExists checks if a file exists in a specific branch
func (m *Manager) FileExists(branch, filePath string) bool {
	_, err := m.GetFileContent(branch, filePath)
	return err == nil
}

// UpdateAllBranches fetches latest changes for all branches
func (m *Manager) UpdateAllBranches() error {
	log.Println("Updating all branches...")

	// Get repository count and create slice of branch-repo pairs to avoid copying
	m.mu.RLock()
	repoCount := len(m.repositories)
	repoPairs := make([]struct{
		branch string
		repo   *git.Repository
	}, 0, repoCount)
	
	for branch, repo := range m.repositories {
		repoPairs = append(repoPairs, struct{
			branch string
			repo   *git.Repository
		}{branch, repo})
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	errors := make(chan error, repoCount)

	for _, pair := range repoPairs {
		wg.Add(1)
		go func(b string, r *git.Repository) {
			defer wg.Done()
			if err := m.fetchRepository(r); err != nil {
				errors <- fmt.Errorf("failed to update branch %s: %w", b, err)
			} else {
				log.Printf("Updated branch %s", b)
			}
		}(pair.branch, pair.repo)
	}

	wg.Wait()
	close(errors)

	var updateErrors []string
	for err := range errors {
		updateErrors = append(updateErrors, err.Error())
		log.Printf("Update error: %v", err)
	}

	if len(updateErrors) > 0 {
		return fmt.Errorf("some branches failed to update: %v", updateErrors)
	}

	log.Println("All branches updated successfully")
	return nil
}

// GetAvailableBranches returns the list of available branches
func (m *Manager) GetAvailableBranches() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var branches []string
	for branch := range m.repositories {
		branches = append(branches, branch)
	}
	return branches
}

// GetRepositoryPath returns the local path for a specific branch
func (m *Manager) GetRepositoryPath(branch string) string {
	version := m.extractVersion(branch)
	return filepath.Join(m.versionsPath, version)
}

// GetRepository returns a repository interface for a specific branch
func (m *Manager) GetRepository(branch string) (shared.GitRepository, error) {
	m.mu.RLock()
	repo, exists := m.repositories[branch]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repository for branch %s not found", branch)
	}

	return &repositoryImpl{
		repo:    repo,
		branch:  branch,
		manager: m,
	}, nil
}

// GetAllBranches returns the list of available branches (implements shared.GitManager interface)
func (m *Manager) GetAllBranches() []string {
	return m.GetAvailableBranches()
}

// UpdateAll updates all branches (implements shared.GitManager interface)
func (m *Manager) UpdateAll() error {
	return m.UpdateAllBranches()
}

// Repository interface for git operations
type Repository interface {
	GetFile(path string) ([]byte, error)
	ListFiles(pattern string) ([]string, error)
	GetBranch() string
}

// repositoryImpl implements the Repository interface
type repositoryImpl struct {
	repo    *git.Repository
	branch  string
	manager *Manager
}

// GetFile gets a file from the repository
func (r *repositoryImpl) GetFile(path string) ([]byte, error) {
	return r.manager.GetFileContent(r.branch, path)
}

// ListFiles lists files matching a pattern
func (r *repositoryImpl) ListFiles(pattern string) ([]string, error) {
	// Convert glob pattern to directory path
	// e.g., "images/*.yml" -> "images"
	var dirPath string
	var filePattern string

	if strings.Contains(pattern, "/") {
		parts := strings.Split(pattern, "/")
		if len(parts) > 1 {
			dirPath = strings.Join(parts[:len(parts)-1], "/")
			filePattern = parts[len(parts)-1]
		}
	} else {
		filePattern = pattern
	}

	allFiles, err := r.manager.ListFiles(r.branch, dirPath)
	if err != nil {
		return nil, err
	}

	// Filter files by pattern
	var matchedFiles []string
	for _, file := range allFiles {
		// Match against the file pattern, not the full path pattern
		if matched, _ := filepath.Match(filePattern, file); matched {
			// Return full path relative to repository root
			if dirPath != "" {
				matchedFiles = append(matchedFiles, filepath.Join(dirPath, file))
			} else {
				matchedFiles = append(matchedFiles, file)
			}
		}
	}

	return matchedFiles, nil
}

// GetBranch returns the branch name
func (r *repositoryImpl) GetBranch() string {
	return r.branch
}

// Initialize initializes the repository (already initialized, so no-op)
func (r *repositoryImpl) Initialize() error {
	return nil
}

// Update updates the repository
func (r *repositoryImpl) Update() error {
	return r.manager.fetchRepository(r.repo)
}

// GetCommitHistory returns the commit history for a file
func (r *repositoryImpl) GetCommitHistory(path string, limit int) ([]shared.Commit, error) {
	// Get the current commit
	ref, err := r.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// Get commit iterator
	commitIter, err := r.repo.Log(&git.LogOptions{
		From:     ref.Hash(),
		FileName: &path,
		Order:    git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer commitIter.Close()

	var commits []shared.Commit
	count := 0
	err = commitIter.ForEach(func(c *object.Commit) error {
		if count >= limit {
			return fmt.Errorf("limit reached")
		}
		
		commits = append(commits, shared.Commit{
			Hash:    c.Hash.String(),
			Author:  c.Author.Name,
			Email:   c.Author.Email,
			Date:    c.Author.When,
			Message: c.Message,
		})
		count++
		return nil
	})
	
	if err != nil && err.Error() != "limit reached" {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// Cleanup removes all local repositories (use with caution)
func (m *Manager) Cleanup() error {
	log.Println("Cleaning up git repositories...")

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear the repositories map
	m.repositories = make(map[string]*git.Repository)

	// Remove the versions directory
	if err := os.RemoveAll(m.versionsPath); err != nil {
		return fmt.Errorf("failed to remove versions directory: %w", err)
	}

	log.Println("Git repositories cleanup completed")
	return nil
}
