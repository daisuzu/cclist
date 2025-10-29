package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty/v2"

	"github.com/daisuzu/cclist/pkg/models"
)

// Manager manages ClaudeCode sessions and shell terminals.
type Manager struct {
	root           *os.Root
	sessions       map[string]*sessionData
	shellTerminals map[string]*shellTerminalData
	outputs        map[string][]models.SessionOutput
	mu             sync.RWMutex
}

type sessionData struct {
	session *models.Session
	cmd     *exec.Cmd
	pty     *os.File
}

type shellTerminalData struct {
	id        string
	directory string
	cmd       *exec.Cmd
	pty       *os.File
	startedAt time.Time
	isActive  bool
}

// NewManager creates a new session manager.
func NewManager(rootPath string) *Manager {
	// Open the root directory with os.Root for secure path operations
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		panic(fmt.Sprintf("failed to open root directory: %v", err))
	}
	return &Manager{
		root:           root,
		sessions:       make(map[string]*sessionData),
		shellTerminals: make(map[string]*shellTerminalData),
		outputs:        make(map[string][]models.SessionOutput),
	}
}

// cmdDir returns the absolute directory path for use with exec.Cmd.Dir.
// It validates that repoPath exists within the root boundary.
func (m *Manager) cmdDir(repoPath string) (string, error) {
	// Verify path exists within root boundary using os.Root
	if _, err := m.root.Stat(repoPath); err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// Path is validated, safe to construct absolute path
	return filepath.Join(m.root.Name(), repoPath), nil
}

// generateSessionID generates a unique session ID using crypto/rand.
func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// StartSession starts a new ClaudeCode session in the specified repository.
func (m *Manager) StartSession(repoPath, prompt string, args []string) (*models.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate session ID
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	dir, err := m.cmdDir(repoPath)
	if err != nil {
		return nil, err
	}

	// Build command arguments (don't include prompt at startup)
	cmdArgs := []string{}
	cmdArgs = append(cmdArgs, args...)

	// Create command
	cmd := exec.Command("claude", cmdArgs...)
	cmd.Dir = dir

	// Set environment variables for proper terminal behavior
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	// Start command with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start claude with pty: %w", err)
	}

	// Set initial PTY size to match typical terminal dimensions
	// This will be updated when WebSocket client connects with actual dimensions
	initialSize := pty.Winsize{
		Rows: 30,
		Cols: 120,
	}
	if err := pty.Setsize(ptmx, &initialSize); err != nil {
		slog.Warn("failed to set initial PTY size", "error", err)
	} else {
		slog.Debug("set initial PTY size", "cols", initialSize.Cols, "rows", initialSize.Rows, "session_id", sessionID)
	}

	// Create session
	session := &models.Session{
		ID:        sessionID,
		Directory: repoPath,
		StartedAt: time.Now(),
		IsActive:  true,
		ProcessID: cmd.Process.Pid,
	}

	// Store session data
	m.sessions[sessionID] = &sessionData{
		session: session,
		cmd:     cmd,
		pty:     ptmx,
	}
	m.outputs[sessionID] = []models.SessionOutput{}

	// Note: PTY output is handled by WebSocket terminal
	// No captureOutputPTY goroutine to avoid concurrent reads from PTY

	// Send initial prompt if provided (after a longer delay to let PTY size be set by WebSocket)
	if prompt != "" {
		go func() {
			time.Sleep(1500 * time.Millisecond) // Wait for WebSocket to connect and set PTY size
			_, err := ptmx.Write([]byte(prompt + "\n"))
			if err != nil {
				slog.Warn("failed to send initial prompt", "error", err)
			} else {
				slog.Debug("sent initial prompt", "session_id", sessionID)
			}
		}()
	}

	// Monitor process completion
	go m.monitorProcess(sessionID)

	slog.Info("started session", "session_id", sessionID, "repository", repoPath, "pid", cmd.Process.Pid)

	return session, nil
}

// ResumeSession resumes a previous ClaudeCode session using claude --resume.
func (m *Manager) ResumeSession(repoPath string) (*models.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate session ID
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	dir, err := m.cmdDir(repoPath)
	if err != nil {
		return nil, err
	}

	// Create command with --resume flag (without session ID to use interactive selection)
	cmd := exec.Command("claude", "--resume")
	cmd.Dir = dir

	// Set environment variables for proper terminal behavior
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	// Start command with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start claude --resume with pty: %w", err)
	}

	// Set initial PTY size
	initialSize := pty.Winsize{
		Rows: 30,
		Cols: 120,
	}
	if err := pty.Setsize(ptmx, &initialSize); err != nil {
		slog.Warn("failed to set initial PTY size", "error", err)
	} else {
		slog.Debug("set initial PTY size", "cols", initialSize.Cols, "rows", initialSize.Rows, "session_id", sessionID)
	}

	// Create session
	session := &models.Session{
		ID:        sessionID,
		Directory: repoPath,
		StartedAt: time.Now(),
		IsActive:  true,
		ProcessID: cmd.Process.Pid,
	}

	// Store session data
	m.sessions[sessionID] = &sessionData{
		session: session,
		cmd:     cmd,
		pty:     ptmx,
	}
	m.outputs[sessionID] = []models.SessionOutput{}

	// Monitor process completion
	go m.monitorProcess(sessionID)

	slog.Info("resumed session", "session_id", sessionID, "repository", repoPath, "pid", cmd.Process.Pid)

	return session, nil
}

// monitorProcess monitors the process and updates session when it exits.
func (m *Manager) monitorProcess(sessionID string) {
	m.mu.RLock()
	data, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	// Wait for process to complete
	err := data.cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if data, exists := m.sessions[sessionID]; exists {
		data.session.IsActive = false

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode := exitErr.ExitCode()
				data.session.ExitCode = &exitCode
				slog.Info("session exited", "session_id", sessionID, "exit_code", exitCode)
			} else {
				slog.Error("session exited with error", "session_id", sessionID, "error", err)
			}
		} else {
			exitCode := 0
			data.session.ExitCode = &exitCode
			slog.Info("session completed successfully", "session_id", sessionID)
		}
	}
}

// ListSessions returns all sessions.
func (m *Manager) ListSessions() []*models.Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*models.Session, 0, len(m.sessions))
	for _, data := range m.sessions {
		sessions = append(sessions, data.session)
	}

	return sessions
}

// TerminateSession terminates an active session.
func (m *Manager) TerminateSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if !data.session.IsActive {
		return fmt.Errorf("session is not active: %s", sessionID)
	}

	// Close PTY
	if err := data.pty.Close(); err != nil {
		slog.Error("error closing PTY", "session_id", sessionID, "error", err)
	}

	// Kill process
	if err := data.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	data.session.IsActive = false
	slog.Info("terminated session", "session_id", sessionID)

	return nil
}

// GetPTY returns the PTY file for a session (for WebSocket terminal access).
func (m *Manager) GetPTY(sessionID string) (*os.File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if it's a ClaudeCode session
	if data, exists := m.sessions[sessionID]; exists {
		if !data.session.IsActive {
			return nil, fmt.Errorf("session is not active: %s", sessionID)
		}
		return data.pty, nil
	}

	// Check if it's a shell terminal
	if data, exists := m.shellTerminals[sessionID]; exists {
		if !data.isActive {
			return nil, fmt.Errorf("shell terminal is not active: %s", sessionID)
		}
		return data.pty, nil
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// StartShellTerminal starts a new shell terminal in the specified directory.
func (m *Manager) StartShellTerminal(repoPath, shell string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate terminal ID
	terminalID, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate terminal ID: %w", err)
	}

	dir, err := m.cmdDir(repoPath)
	if err != nil {
		return "", err
	}

	// Create shell command (use configured shell or default to bash)
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "-l")
	cmd.Dir = dir

	// Set environment variables for proper terminal behavior
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	// Start command with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start shell with pty: %w", err)
	}

	// Set initial PTY size
	initialSize := pty.Winsize{
		Rows: 30,
		Cols: 120,
	}
	if err := pty.Setsize(ptmx, &initialSize); err != nil {
		slog.Warn("failed to set initial PTY size for shell terminal", "error", err)
	}

	// Store shell terminal data
	m.shellTerminals[terminalID] = &shellTerminalData{
		id:        terminalID,
		directory: repoPath,
		cmd:       cmd,
		pty:       ptmx,
		startedAt: time.Now(),
		isActive:  true,
	}

	// Monitor process completion
	go m.monitorShellTerminal(terminalID)

	slog.Info("started shell terminal", "terminal_id", terminalID, "repository", repoPath, "pid", cmd.Process.Pid)

	return terminalID, nil
}

// monitorShellTerminal monitors the shell terminal process.
func (m *Manager) monitorShellTerminal(terminalID string) {
	m.mu.RLock()
	data, exists := m.shellTerminals[terminalID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	// Wait for process to complete
	err := data.cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if data, exists := m.shellTerminals[terminalID]; exists {
		data.isActive = false

		if err != nil {
			slog.Error("shell terminal exited with error", "terminal_id", terminalID, "error", err)
		} else {
			slog.Info("shell terminal completed successfully", "terminal_id", terminalID)
		}
	}
}

// TerminateShellTerminal terminates a shell terminal.
func (m *Manager) TerminateShellTerminal(terminalID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, exists := m.shellTerminals[terminalID]
	if !exists {
		return fmt.Errorf("shell terminal not found: %s", terminalID)
	}

	if !data.isActive {
		return fmt.Errorf("shell terminal is not active: %s", terminalID)
	}

	// Close PTY
	if err := data.pty.Close(); err != nil {
		slog.Error("error closing PTY for shell terminal", "terminal_id", terminalID, "error", err)
	}

	// Kill process
	if err := data.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill shell terminal process: %w", err)
	}

	data.isActive = false
	slog.Info("terminated shell terminal", "terminal_id", terminalID)

	// Clean up from map
	delete(m.shellTerminals, terminalID)

	return nil
}
