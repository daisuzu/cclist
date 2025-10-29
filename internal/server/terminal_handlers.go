package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/daisuzu/cclist/pkg/models"
)

// handleCreateTerminal creates a new shell terminal session.
func (s *Server) handleCreateTerminal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RepositoryPath string `json:"repositoryPath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepositoryPath == "" {
		http.Error(w, "Repository path is required", http.StatusBadRequest)
		return
	}

	// Get shell from config
	appConfig := s.config.Get()
	shell := appConfig.Terminal.Shell
	if shell == "" {
		shell = "/bin/bash"
	}

	// Start shell terminal
	terminalID, err := s.sessionManager.StartShellTerminal(req.RepositoryPath, shell)
	if err != nil {
		slog.Error("failed to start shell terminal", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Shell terminal started successfully",
		Data: map[string]any{
			"id":        terminalID,
			"directory": req.RepositoryPath,
		},
	}

	respondJSON(w, response)
}

// handleDeleteTerminal terminates a shell terminal session.
func (s *Server) handleDeleteTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("id")
	if terminalID == "" {
		http.Error(w, "Terminal ID is required", http.StatusBadRequest)
		return
	}

	if err := s.sessionManager.TerminateShellTerminal(terminalID); err != nil {
		slog.Error("failed to terminate shell terminal", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Shell terminal terminated successfully",
	}

	respondJSON(w, response)
}
