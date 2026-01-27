package server

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/daisuzu/cclist/internal/config"
	"github.com/daisuzu/cclist/internal/scanner"
	"github.com/daisuzu/cclist/internal/session"
)

//go:embed ui/*
var uiFiles embed.FS

// generateShutdownToken generates a random token for shutdown authentication.
func generateShutdownToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Server represents the HTTP server.
type Server struct {
	config              *config.Manager
	scanner             *scanner.Scanner
	worktreeScanner     atomic.Value // stores *scanner.WorktreeScanner
	sessionManager      *session.Manager
	httpServer          *http.Server
	shutdownToken       string
	activeConnections   map[string]*websocket.Conn
	cancelFuncs         map[string]context.CancelFunc
	connectionsMu       sync.RWMutex
	shutdownCh          chan struct{} // Channel to signal shutdown
	closeShutdownChFunc func()        // Function to close shutdownCh exactly once
}

// NewServer creates a new HTTP server instance.
func NewServer(cfg *config.Manager) *Server {
	appConfig := cfg.Get()

	// Get shutdown token from environment variable or generate new one
	shutdownToken := os.Getenv("CCLIST_SHUTDOWN_TOKEN")
	if shutdownToken == "" {
		var err error
		shutdownToken, err = generateShutdownToken()
		if err != nil {
			// This is a critical error during initialization
			// In a production system, this should be handled by returning an error
			// For now, we panic since the server cannot operate without a token
			panic(fmt.Sprintf("failed to generate shutdown token: %v", err))
		}
	}

	shutdownCh := make(chan struct{})
	sessionMgr := session.NewManager(appConfig.RootPath)

	// Load session outputs cache from file
	if cache, err := cfg.LoadSessionOutputs(); err != nil {
		slog.Warn("failed to load session outputs cache", "error", err)
	} else {
		sessionMgr.SetOutputCache(cache)
	}

	s := &Server{
		config:              cfg,
		scanner:             scanner.NewScanner(appConfig.RootPath),
		sessionManager:      sessionMgr,
		shutdownToken:       shutdownToken,
		activeConnections:   make(map[string]*websocket.Conn),
		cancelFuncs:         make(map[string]context.CancelFunc),
		shutdownCh:          shutdownCh,
		closeShutdownChFunc: sync.OnceFunc(func() { close(shutdownCh) }),
	}
	s.worktreeScanner.Store(scanner.NewWorktreeScanner(appConfig.RootPath, appConfig.Worktree.PathPattern))

	// Create HTTP server
	mux := http.NewServeMux()
	s.setupRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", appConfig.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// GetShutdownToken returns the server's shutdown token.
func (s *Server) GetShutdownToken() string {
	return s.shutdownToken
}

// getWorktreeScanner returns the current worktree scanner atomically.
func (s *Server) getWorktreeScanner() *scanner.WorktreeScanner {
	return s.worktreeScanner.Load().(*scanner.WorktreeScanner)
}

// setWorktreeScanner sets a new worktree scanner atomically.
func (s *Server) setWorktreeScanner(ws *scanner.WorktreeScanner) {
	s.worktreeScanner.Store(ws)
}

// getShell returns the shell to use for session/terminal.
func (s *Server) getShell() string {
	if shell := s.config.Get().Terminal.Shell; shell != "" {
		return shell
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash"
}

// setupRoutes sets up HTTP routes.
func (s *Server) setupRoutes(mux *http.ServeMux) {
	// Serve static files
	uiFS, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		// This is a critical error during initialization
		panic(fmt.Sprintf("failed to create UI filesystem: %v", err))
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(uiFS))))

	// HTML routes (SPA - all return index.html)
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/repo/", s.handleIndex)
	mux.HandleFunc("/settings", s.handleIndex)

	// API routes
	// Repository management
	mux.HandleFunc("GET /api/repositories", s.handleListRepositories)
	mux.HandleFunc("POST /api/repositories", s.handleCreateRepository)
	mux.HandleFunc("DELETE /api/repositories/{path...}", s.handleDeleteRepository)
	mux.HandleFunc("POST /api/repositories/discover", s.handleDiscover)

	// Directory management
	mux.HandleFunc("GET /api/directories/{path...}", s.handleDirectoryDetail)

	// Config management
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handleUpdateConfig)

	// Session management
	mux.HandleFunc("POST /api/sessions", s.handleStartSession)
	mux.HandleFunc("POST /api/sessions/resume", s.handleResumeSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleTerminateSession)

	// Worktree management
	mux.HandleFunc("GET /api/worktrees/{path...}", s.handleListWorktrees)
	mux.HandleFunc("POST /api/worktrees/{path...}", s.handleCreateWorktree)
	mux.HandleFunc("DELETE /api/worktrees/{path...}", s.handleDeleteWorktree)
	mux.HandleFunc("GET /api/branches/{path...}", s.handleListBranches)

	// Terminal management (shell terminals)
	mux.HandleFunc("POST /api/terminal", s.handleCreateTerminal)
	mux.HandleFunc("DELETE /api/terminal/{id}", s.handleDeleteTerminal)

	// WebSocket for terminal (shared for both ClaudeCode and shell terminals)
	mux.HandleFunc("GET /ws/terminal/{id}", s.handleTerminalWebSocket)

	// Shutdown
	mux.HandleFunc("POST /api/shutdown", s.handleShutdown)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	appConfig := s.config.Get()
	slog.Debug("starting HTTP server",
		"address", fmt.Sprintf("127.0.0.1:%d", appConfig.Port))
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down server")

	// Save session outputs cache
	if err := s.saveSessionOutputs(); err != nil {
		slog.Warn("failed to save session outputs cache", "error", err)
	}

	// Close worktree scanner to release file descriptor
	if ws := s.getWorktreeScanner(); ws != nil {
		if err := ws.Close(); err != nil {
			slog.Warn("failed to close worktree scanner during shutdown", "error", err)
		}
	}

	return s.httpServer.Shutdown(ctx)
}

func (s *Server) saveSessionOutputs() error {
	cache := s.sessionManager.GetAllOutputCache()
	return s.config.SaveSessionOutputs(cache)
}

// handleIndex serves the index.html for all page routes.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read index.html from embedded files
	data, err := uiFiles.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "Failed to load page", http.StatusInternalServerError)
		slog.Error("failed to read index.html", "error", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

// handleShutdown handles graceful shutdown requests.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	// Verify shutdown token
	token := r.Header.Get("X-Shutdown-Token")
	if token == "" || token != s.shutdownToken {
		slog.Warn("shutdown requested with invalid token")
		http.Error(w, "Invalid shutdown token", http.StatusUnauthorized)
		return
	}

	slog.Info("shutdown requested via API")

	respondJSON(w, map[string]any{
		"success": true,
		"message": "Server shutting down",
	})

	// Signal shutdown to main goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.closeShutdownChFunc()
	}()
}

// ShutdownChan returns the channel that signals API-based shutdown.
func (s *Server) ShutdownChan() <-chan struct{} {
	return s.shutdownCh
}
