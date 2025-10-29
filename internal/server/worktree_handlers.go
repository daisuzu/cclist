package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/daisuzu/cclist/pkg/models"
)

// handleListWorktrees handles GET /api/worktrees/{path...}.
func (s *Server) handleListWorktrees(w http.ResponseWriter, r *http.Request) {
	repoPath := r.PathValue("path")
	if repoPath == "" {
		http.Error(w, "Repository path required", http.StatusBadRequest)
		return
	}

	worktrees, err := s.getWorktreeScanner().ListWorktrees(repoPath)
	if err != nil {
		slog.Error("failed to list worktrees", "path", repoPath, "error", err)
		http.Error(w, "Failed to list worktrees", http.StatusInternalServerError)
		return
	}

	response := models.APIResponse{
		Success: true,
		Data: map[string]any{
			"worktrees": worktrees,
		},
	}

	respondJSON(w, response)
}

// handleListBranches handles GET /api/branches/{path...}.
func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request) {
	repoPath := r.PathValue("path")
	if repoPath == "" {
		http.Error(w, "Repository path required", http.StatusBadRequest)
		return
	}

	branches, err := s.getWorktreeScanner().ListBranches(repoPath)
	if err != nil {
		slog.Error("failed to list branches", "path", repoPath, "error", err)
		http.Error(w, "Failed to list branches", http.StatusInternalServerError)
		return
	}

	response := models.APIResponse{
		Success: true,
		Data: map[string]any{
			"branches": branches,
		},
	}

	respondJSON(w, response)
}

// handleCreateWorktree handles POST /api/worktrees/{path...}.
func (s *Server) handleCreateWorktree(w http.ResponseWriter, r *http.Request) {
	repoPath := r.PathValue("path")
	if repoPath == "" {
		http.Error(w, "Repository path required", http.StatusBadRequest)
		return
	}

	var req models.CreateWorktreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Branch == "" {
		http.Error(w, "Branch name required", http.StatusBadRequest)
		return
	}

	worktreePath, err := s.getWorktreeScanner().CreateWorktree(
		repoPath,
		req.Branch,
		req.BaseBranch,
		req.CreateBranch,
		req.FromRemote,
		req.CustomPath,
	)
	if err != nil {
		slog.Error("failed to create worktree", "path", repoPath, "error", err)
		response := models.APIResponse{
			Success: false,
			Message: err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("failed to encode JSON response", "error", err)
		}
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Worktree created successfully",
		Data: map[string]any{
			"path":   worktreePath,
			"branch": req.Branch,
		},
	}

	respondJSON(w, response)
}

// handleDeleteWorktree handles DELETE /api/worktrees/{path...}.
func (s *Server) handleDeleteWorktree(w http.ResponseWriter, r *http.Request) {
	repoPath := r.PathValue("path")
	if repoPath == "" {
		http.Error(w, "Repository path required", http.StatusBadRequest)
		return
	}

	var req struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Branch == "" {
		http.Error(w, "Branch name required in request body", http.StatusBadRequest)
		return
	}

	if err := s.getWorktreeScanner().RemoveWorktree(repoPath, req.Branch); err != nil {
		slog.Error("failed to remove worktree", "branch", req.Branch, "path", repoPath, "error", err)
		response := models.APIResponse{
			Success: false,
			Message: err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("failed to encode JSON response", "error", err)
		}
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Worktree removed successfully",
	}

	respondJSON(w, response)
}
