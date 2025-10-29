package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/daisuzu/cclist/pkg/models"
)

// Scanner handles directory scanning and ClaudeCode detection.
type Scanner struct {
	root *os.Root
}

// NewScanner creates a new scanner instance.
func NewScanner(rootPath string) *Scanner {
	// Open the root directory with os.Root for secure path operations
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		// Fallback: if os.Root fails, log the error
		// The caller should handle this case appropriately
		panic(fmt.Sprintf("failed to open root directory: %v", err))
	}
	return &Scanner{
		root: root,
	}
}

// ScanRepository scans a specific repository directory.
func (s *Scanner) ScanRepository(repoPath string) (*models.Directory, error) {
	// Check if directory exists using Root
	info, err := s.root.Stat(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", repoPath)
	}

	// Create directory info
	dir := &models.Directory{
		Path:             repoPath, // Store relative path
		RelativePath:     repoPath,
		HasClaudeHistory: s.hasClaudeHistory(repoPath),
		IsWorktree:       s.isWorktree(repoPath),
		Children:         []*models.Directory{},
		LastAccessed:     info.ModTime(),
	}

	// Detect git branch
	branch, err := s.detectGitBranch(repoPath)
	if err == nil {
		dir.GitBranch = branch
	}

	// Detect active session
	session, err := s.detectSession(repoPath)
	if err == nil && session != nil {
		dir.ActiveSession = session
	}

	return dir, nil
}

// DiscoverRepositories scans Root directory for repositories with .claude/ directory.
func (s *Scanner) DiscoverRepositories() ([]string, error) {
	var discovered []string

	// Walk through root directory using fs.WalkDir with Root's FS()
	if err := fs.WalkDir(s.root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check if this is a .claude directory
		if d.IsDir() && d.Name() == ".claude" {
			// Get parent directory (the repository)
			repoDir := filepath.Dir(path)

			// Normalize path
			if repoDir == "." {
				repoDir = ""
			}

			// Skip if this is a worktree - worktrees will be discovered via their main repo
			if s.isWorktree(repoDir) {
				return fs.SkipDir
			}

			// Add to discovered list
			discovered = append(discovered, repoDir)

			// Skip walking into .claude directory
			return fs.SkipDir
		}

		// Skip hidden directories (except .claude which is handled above)
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			return fs.SkipDir
		}

		// Skip vendor and node_modules directories
		if d.IsDir() && (d.Name() == "vendor" || d.Name() == "node_modules") {
			return fs.SkipDir
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return discovered, nil
}

// HasClaudeHistory checks if a directory has .claude/ directory.
func (s *Scanner) HasClaudeHistory(dir string) bool {
	claudeDir := filepath.Join(dir, ".claude")
	if info, err := s.root.Stat(claudeDir); err == nil && info.IsDir() {
		return true
	}
	return false
}

// hasClaudeHistory is an alias for HasClaudeHistory for internal use.
func (s *Scanner) hasClaudeHistory(dir string) bool {
	return s.HasClaudeHistory(dir)
}

// isWorktree checks if a directory is a git worktree.
// A worktree has a .git file (not directory) that points to the main repo.
func (s *Scanner) isWorktree(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := s.root.Stat(gitPath)
	if err != nil {
		return false
	}
	// If .git is a file (not a directory), it's a worktree
	return !info.IsDir()
}

// detectGitBranch detects the current git branch using git command.
func (s *Scanner) detectGitBranch(dir string) (string, error) {
	// Build full path for exec.Command
	fullPath := filepath.Join(s.root.Name(), dir)

	// Use git command to get current branch
	cmd := exec.Command("git", "-C", fullPath, "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		// Detached HEAD or other state
		return "detached", nil
	}

	return branch, nil
}

// detectSession detects if there's an active ClaudeCode session.
func (s *Scanner) detectSession(dir string) (*models.Session, error) {
	claudeDir := filepath.Join(dir, ".claude")

	// Check if .claude directory exists
	if !s.hasClaudeHistory(dir) {
		return nil, nil
	}

	historyDir := filepath.Join(claudeDir, "history")

	// Check if history directory exists using Root
	if _, err := s.root.Stat(historyDir); os.IsNotExist(err) {
		return nil, nil
	}

	// Find the most recent history file
	var latestFile string
	var latestTime time.Time

	entries, err := fs.ReadDir(s.root.FS(), historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = filepath.Join(historyDir, entry.Name())
		}
	}

	if latestFile == "" {
		return nil, nil
	}

	// Create session info
	session := &models.Session{
		ID:          filepath.Base(latestFile),
		Directory:   dir,
		IsActive:    false, // TODO: Detect if process is running
		StartedAt:   latestTime,
		LastCommand: latestTime,
		OutputPath:  latestFile,
		ProcessID:   0,
	}

	return session, nil
}
