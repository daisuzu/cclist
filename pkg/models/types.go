package models

import "time"

// Directory represents a scanned directory with ClaudeCode context.
type Directory struct {
	Path             string       `json:"path"`
	RelativePath     string       `json:"relativePath"`
	HasClaudeHistory bool         `json:"hasClaudeHistory"`
	ActiveSession    *Session     `json:"activeSession"`
	IsWorktree       bool         `json:"isWorktree"`
	ParentPath       string       `json:"parentPath"`
	Children         []*Directory `json:"children"`
	GitBranch        string       `json:"gitBranch"`
	LastAccessed     time.Time    `json:"lastAccessed"`
	FileCount        int          `json:"fileCount"`
}

// Session represents a ClaudeCode session.
type Session struct {
	ID          string    `json:"id"`
	Directory   string    `json:"directory"`
	IsActive    bool      `json:"isActive"`
	StartedAt   time.Time `json:"startedAt"`
	LastCommand time.Time `json:"lastCommand"`
	OutputPath  string    `json:"outputPath"`
	ProcessID   int       `json:"processID"`
	LastOutput  string    `json:"lastOutput,omitempty"`
	ExitCode    *int      `json:"exitCode,omitempty"`
}

// SessionOutput represents output from a session.
type SessionOutput struct {
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	IsError   bool      `json:"isError"`
}

// StartSessionRequest is the request to start a new session.
type StartSessionRequest struct {
	RepositoryPath string   `json:"repositoryPath"`
	Prompt         string   `json:"prompt,omitempty"`
	Args           []string `json:"args,omitempty"`
}

// ResumeSessionRequest is the request to resume a previous session.
type ResumeSessionRequest struct {
	RepositoryPath string `json:"repositoryPath"`
}

// SendPromptRequest is the request to send a prompt to an active session.
type SendPromptRequest struct {
	Prompt string `json:"prompt"`
}

// TerminalSession represents a terminal emulation session.
type TerminalSession struct {
	ID        string    `json:"id"`
	Directory string    `json:"directory"`
	Shell     string    `json:"shell"`
	Env       []string  `json:"env"`
	CreatedAt time.Time `json:"createdAt"`
	LastUsed  time.Time `json:"lastUsed"`
}

// DirectoryTree represents the hierarchical structure.
type DirectoryTree struct {
	Root      *Directory              `json:"root"`
	Worktrees map[string][]*Directory `json:"worktrees"` // parent path -> children
	Flat      []*Directory            `json:"flat"`
}

// Config represents the application configuration.
type Config struct {
	RootPath     string       `json:"rootPath"`
	Port         int          `json:"port"`
	Repositories []Repository `json:"repositories"`
	Worktree     Worktree     `json:"worktree"`
	Terminal     Terminal     `json:"terminal"`
	UI           UI           `json:"ui"`
}

// Repository represents a registered repository.
type Repository struct {
	Path                string `json:"path"`
	FullPath            string `json:"fullPath,omitempty"`
	AutoDetectWorktrees bool   `json:"autoDetectWorktrees"`
}

// Worktree represents worktree configuration.
type Worktree struct {
	PathPattern string `json:"pathPattern"` // Default: "../{repo}-{branch}"
}

// Terminal represents terminal configuration.
type Terminal struct {
	Shell string `json:"shell"`
	Rows  int    `json:"rows"`
	Cols  int    `json:"cols"`
}

// UI represents UI configuration.
type UI struct {
	Theme           string `json:"theme"`
	RefreshInterval string `json:"refreshInterval"`
	TerminalLayout  string `json:"terminalLayout"` // "auto", "horizontal", or "vertical"
}

// OutputEntry represents a single output entry from ClaudeCode.
type OutputEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "user_input" or "assistant_output"
	Content   string    `json:"content"`
}

// OutputResponse represents the response for session output API.
type OutputResponse struct {
	Outputs         []OutputEntry `json:"outputs"`
	HasMore         bool          `json:"hasMore"`
	OldestTimestamp time.Time     `json:"oldestTimestamp"`
	LatestTimestamp time.Time     `json:"latestTimestamp"`
}

// SessionOutputCache represents cached latest output for a session.
type SessionOutputCache struct {
	LastOutput string    `json:"lastOutput"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// BranchInfo represents branch information.
type BranchInfo struct {
	Local     []string `json:"local"`
	Remote    []string `json:"remote"`
	Worktrees []string `json:"worktrees"`
}

// WorktreeInfo represents a git worktree.
type WorktreeInfo struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
	IsMain bool   `json:"isMain"`
}

// PromptRequest represents a prompt submission request.
type PromptRequest struct {
	Prompt string `json:"prompt"`
}

// CreateRepositoryRequest represents a repository registration request.
type CreateRepositoryRequest struct {
	Path                string `json:"path"`
	AutoDetectWorktrees bool   `json:"autoDetectWorktrees"`
}

// CreateWorktreeRequest represents a worktree creation request.
type CreateWorktreeRequest struct {
	Branch       string `json:"branch"`
	BaseBranch   string `json:"baseBranch,omitempty"`
	CreateBranch bool   `json:"createBranch"`
	FromRemote   bool   `json:"fromRemote"`
	CustomPath   string `json:"customPath,omitempty"`
}

// CreateTerminalRequest represents a terminal creation request.
type CreateTerminalRequest struct {
	Directory string `json:"directory"`
	Shell     string `json:"shell,omitempty"`
}

// APIResponse represents a generic API response.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
