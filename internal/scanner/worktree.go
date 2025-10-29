package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/daisuzu/cclist/pkg/models"
)

// WorktreeScanner handles git worktree operations.
type WorktreeScanner struct {
	root        *os.Root
	pathPattern string
}

// NewWorktreeScanner creates a new worktree scanner.
func NewWorktreeScanner(rootPath, pathPattern string) *WorktreeScanner {
	// Open the root directory with os.Root for secure path operations
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		panic(fmt.Sprintf("failed to open root directory: %v", err))
	}
	return &WorktreeScanner{
		root:        root,
		pathPattern: pathPattern,
	}
}

// Close closes the underlying os.Root to release file descriptor.
func (w *WorktreeScanner) Close() error {
	if w.root != nil {
		return w.root.Close()
	}
	return nil
}

// cmdDir returns the absolute directory path for use with exec.Cmd.Dir.
// It validates that repoPath exists within the root boundary.
func (w *WorktreeScanner) cmdDir(repoPath string) (string, error) {
	// Verify path exists within root boundary using os.Root
	if _, err := w.root.Stat(repoPath); err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// Path is validated, safe to construct absolute path
	return filepath.Join(w.root.Name(), repoPath), nil
}

// ListWorktrees lists all worktrees for a repository.
func (w *WorktreeScanner) ListWorktrees(repoPath string) ([]*models.WorktreeInfo, error) {
	dir, err := w.cmdDir(repoPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = dir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return w.parseWorktreeList(string(output))
}

// parseWorktreeList parses the output of git worktree list --porcelain.
func (w *WorktreeScanner) parseWorktreeList(output string) ([]*models.WorktreeInfo, error) {
	var worktrees []*models.WorktreeInfo
	var current *models.WorktreeInfo

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, current)
				current = nil
			}
			continue
		}

		if after, ok := strings.CutPrefix(line, "worktree "); ok {
			current = &models.WorktreeInfo{
				Path: after,
			}
		} else if strings.HasPrefix(line, "branch ") && current != nil {
			branchRef := strings.TrimPrefix(line, "branch ")
			// Extract branch name from refs/heads/branch-name
			if after, ok := strings.CutPrefix(branchRef, "refs/heads/"); ok {
				current.Branch = after
			}
		} else if strings.HasPrefix(line, "bare") && current != nil {
			// This is the main repository
			current.IsMain = true
		}
	}

	// Add the last worktree if exists
	if current != nil {
		worktrees = append(worktrees, current)
	}

	// Mark the first worktree as main if no bare repo found
	if len(worktrees) > 0 && !worktrees[0].IsMain {
		worktrees[0].IsMain = true
	}

	return worktrees, nil
}

// ListBranches lists all branches (local and remote) for a repository.
func (w *WorktreeScanner) ListBranches(repoPath string) (*models.BranchInfo, error) {
	dir, err := w.cmdDir(repoPath)
	if err != nil {
		return nil, err
	}

	// Get local branches
	localCmd := exec.Command("git", "branch", "--format=%(refname:short)")
	localCmd.Dir = dir
	localOutput, err := localCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list local branches: %w", err)
	}

	// Get remote branches
	remoteCmd := exec.Command("git", "branch", "-r", "--format=%(refname:short)")
	remoteCmd.Dir = dir
	remoteOutput, err := remoteCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %w", err)
	}

	// Get worktree branches
	worktrees, err := w.ListWorktrees(repoPath)
	if err != nil {
		return nil, err
	}

	worktreeBranches := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.Branch != "" {
			worktreeBranches = append(worktreeBranches, wt.Branch)
		}
	}

	return &models.BranchInfo{
		Local:     w.parseLines(string(localOutput)),
		Remote:    w.parseLines(string(remoteOutput)),
		Worktrees: worktreeBranches,
	}, nil
}

// parseLines parses output lines into a slice of strings.
func (w *WorktreeScanner) parseLines(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// CreateWorktree creates a new worktree.
func (w *WorktreeScanner) CreateWorktree(repoPath, branch, baseBranch string, createBranch, fromRemote bool, customPath string) (string, error) {
	dir, err := w.cmdDir(repoPath)
	if err != nil {
		return "", err
	}

	// Determine worktree path
	var worktreePath string
	if customPath != "" {
		if filepath.IsAbs(customPath) {
			worktreePath = customPath
		} else {
			worktreePath = filepath.Join(dir, customPath)
		}
	} else {
		// Use configured path pattern (e.g., "../{repo}-{branch}")
		repoName := filepath.Base(repoPath)
		branchSafe := strings.ReplaceAll(branch, "/", "-")

		// Replace placeholders in pattern
		pattern := w.pathPattern
		pattern = strings.ReplaceAll(pattern, "{repo}", repoName)
		pattern = strings.ReplaceAll(pattern, "{branch}", branchSafe)

		// If pattern is relative, resolve it relative to repo directory
		if !filepath.IsAbs(pattern) {
			worktreePath = filepath.Join(dir, pattern)
		} else {
			worktreePath = pattern
		}
	}

	// Build git worktree add command
	args := []string{"worktree", "add"}

	if createBranch {
		args = append(args, "-b", branch)
	}

	args = append(args, worktreePath)

	if createBranch && baseBranch != "" {
		args = append(args, baseBranch)
	} else if !createBranch {
		if fromRemote {
			args = append(args, branch)
		} else {
			args = append(args, branch)
		}
	}

	// Run git worktree add
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create worktree: %w, output: %s", err, string(output))
	}

	return worktreePath, nil
}

// RemoveWorktree removes a worktree.
func (w *WorktreeScanner) RemoveWorktree(repoPath, branch string) error {
	dir, err := w.cmdDir(repoPath)
	if err != nil {
		return err
	}

	// Get list of worktrees to find the path
	worktrees, err := w.ListWorktrees(repoPath)
	if err != nil {
		return err
	}

	// Find the worktree with matching branch
	var targetPath string
	for _, wt := range worktrees {
		if wt.Branch == branch {
			targetPath = wt.Path
			break
		}
	}

	if targetPath == "" {
		return fmt.Errorf("worktree not found for branch: %s", branch)
	}

	// Run git worktree remove
	cmd := exec.Command("git", "worktree", "remove", targetPath)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %w, output: %s", err, string(output))
	}

	return nil
}
