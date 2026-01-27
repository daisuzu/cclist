package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/daisuzu/cclist/pkg/models"
)

// handleStartSession starts a new session.
func (s *Server) handleStartSession(w http.ResponseWriter, r *http.Request) {
	var req models.StartSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepositoryPath == "" {
		http.Error(w, "Repository path is required", http.StatusBadRequest)
		return
	}

	session, err := s.sessionManager.StartSession(req.RepositoryPath, req.Prompt, req.Args, s.getShell())
	if err != nil {
		slog.Error("failed to start session", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Session started successfully",
		Data:    session,
	}

	respondJSON(w, response)
}

// handleResumeSession resumes a previous session.
func (s *Server) handleResumeSession(w http.ResponseWriter, r *http.Request) {
	var req models.ResumeSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepositoryPath == "" {
		http.Error(w, "Repository path is required", http.StatusBadRequest)
		return
	}

	session, err := s.sessionManager.ResumeSession(req.RepositoryPath, s.getShell())
	if err != nil {
		slog.Error("failed to resume session", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Session resumed successfully",
		Data:    session,
	}

	respondJSON(w, response)
}

// handleTerminateSession terminates an active session.
func (s *Server) handleTerminateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		http.Error(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	if err := s.sessionManager.TerminateSession(sessionID); err != nil {
		slog.Error("failed to terminate session", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.saveSessionOutputs(); err != nil {
		slog.Warn("failed to save session outputs", "error", err)
	}

	response := models.APIResponse{
		Success: true,
		Message: "Session terminated successfully",
	}

	respondJSON(w, response)
}
