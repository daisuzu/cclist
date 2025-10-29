package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/daisuzu/cclist/internal/scanner"
	"github.com/daisuzu/cclist/pkg/models"
)

// respondJSON sends a JSON response to the client.
func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// handleListRepositories lists all registered repositories.
func (s *Server) handleListRepositories(w http.ResponseWriter, r *http.Request) {
	repos := s.config.ListRepositories()
	appConfig := s.config.Get()

	// Get root path (absolute path)
	// Note: Scanner uses os.Root internally for security
	rootPath := appConfig.RootPath

	// Get all active sessions indexed by directory path
	sessions := s.sessionManager.ListSessions()
	sessionsByPath := make(map[string]*models.Session)
	for _, session := range sessions {
		if session.IsActive {
			sessionsByPath[session.Directory] = session
		}
	}

	// Enrich repositories with current state
	enrichedRepos := make([]map[string]any, 0, len(repos))

	for _, repo := range repos {
		// Scan repository
		dir, err := s.scanner.ScanRepository(repo.Path)
		if err != nil {
			slog.Error("failed to scan repository", "path", repo.Path, "error", err)
			continue
		}

		// Check for active session for main repository
		var activeSession *models.Session
		if session, ok := sessionsByPath[repo.Path]; ok {
			activeSession = session
		}

		repoData := map[string]any{
			"path":                repo.Path,
			"fullPath":            filepath.Join(rootPath, repo.Path),
			"autoDetectWorktrees": repo.AutoDetectWorktrees,
			"hasClaudeHistory":    dir.HasClaudeHistory,
			"activeSession":       activeSession,
			"gitBranch":           dir.GitBranch,
			"lastAccessed":        dir.LastAccessed,
		}

		// Get worktrees if auto-detect is enabled
		if repo.AutoDetectWorktrees {
			worktrees, err := s.getWorktreeScanner().ListWorktrees(repo.Path)
			if err == nil {
				// Enrich each worktree with session information
				enrichedWorktrees := make([]map[string]any, 0, len(worktrees))
				for _, wt := range worktrees {
					// Calculate relative path for worktree
					wtRelPath := wt.Path
					if filepath.IsAbs(wt.Path) {
						rel, err := filepath.Rel(rootPath, wt.Path)
						slog.Debug("calculating relative path", "root_path", rootPath, "worktree_path", wt.Path, "relative", rel, "error", err)
						if err == nil {
							wtRelPath = rel
						}
					}

					// Check for active session for this worktree
					var wtSession *models.Session
					if session, ok := sessionsByPath[wtRelPath]; ok {
						wtSession = session
					}

					// Debug logging
					slog.Debug("worktree info", "path", wt.Path, "branch", wt.Branch, "relative_path", wtRelPath, "has_session", wtSession != nil)

					enrichedWt := map[string]any{
						"branch":        wt.Branch,
						"path":          wt.Path,
						"isMain":        wt.IsMain,
						"activeSession": wtSession,
					}
					enrichedWorktrees = append(enrichedWorktrees, enrichedWt)
				}
				repoData["worktrees"] = enrichedWorktrees
			}
		}

		enrichedRepos = append(enrichedRepos, repoData)
	}

	respondJSON(w, map[string]any{
		"repositories": enrichedRepos,
	})
}

// handleCreateRepository creates a new repository registration.
func (s *Server) handleCreateRepository(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "Repository path is required", http.StatusBadRequest)
		return
	}

	// Add repository to config
	if err := s.config.AddRepository(req.Path, req.AutoDetectWorktrees); err != nil {
		slog.Error("failed to add repository", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	appConfig := s.config.Get()
	response := models.APIResponse{
		Success: true,
		Data: map[string]any{
			"path":                req.Path,
			"fullPath":            filepath.Join(appConfig.RootPath, req.Path),
			"autoDetectWorktrees": req.AutoDetectWorktrees,
		},
	}

	respondJSON(w, response)
}

// handleDeleteRepository deletes a repository registration.
func (s *Server) handleDeleteRepository(w http.ResponseWriter, r *http.Request) {
	// Extract path from URL using PathValue
	path := r.PathValue("path")
	if path == "" {
		http.Error(w, "Repository path is required", http.StatusBadRequest)
		return
	}

	// Remove repository from config
	if err := s.config.RemoveRepository(path); err != nil {
		slog.Error("failed to remove repository", "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Repository removed successfully",
	}

	respondJSON(w, response)
}

// handleDiscover handles repository discovery.
func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {

	// Discover repositories
	discovered, err := s.scanner.DiscoverRepositories()
	if err != nil {
		slog.Error("failed to discover repositories", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	appConfig := s.config.Get()

	// Build response with full paths
	discoveredRepos := make([]map[string]any, 0, len(discovered))
	for _, path := range discovered {
		discoveredRepos = append(discoveredRepos, map[string]any{
			"path":             path,
			"fullPath":         filepath.Join(appConfig.RootPath, path),
			"hasClaudeHistory": true,
		})
	}

	response := map[string]any{
		"discovered": discoveredRepos,
		"count":      len(discovered),
	}

	respondJSON(w, response)
}

// handleDirectoryDetail handles single directory detail.
func (s *Server) handleDirectoryDetail(w http.ResponseWriter, r *http.Request) {
	// Extract path from URL using PathValue
	repoPath := r.PathValue("path")
	if repoPath == "" {
		http.Error(w, "Repository path is required", http.StatusBadRequest)
		return
	}

	appConfig := s.config.Get()

	// Scan repository
	dir, err := s.scanner.ScanRepository(repoPath)
	if err != nil {
		slog.Error("failed to scan repository", "path", repoPath, "error", err)
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Check for active sessions managed by cclist
	var activeSession *models.Session
	sessions := s.sessionManager.ListSessions()
	for _, session := range sessions {
		if session.Directory == repoPath && session.IsActive {
			activeSession = session
			break
		}
	}

	response := map[string]any{
		"path":             repoPath,
		"fullPath":         filepath.Join(appConfig.RootPath, repoPath),
		"hasClaudeHistory": dir.HasClaudeHistory,
		"gitBranch":        dir.GitBranch,
		"lastAccessed":     dir.LastAccessed,
		"activeSession":    activeSession,
		"isWorktree":       dir.IsWorktree,
	}

	respondJSON(w, response)
}

// handleGetConfig retrieves the current configuration.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := s.config.Get()

	respondJSON(w, config)
}

// handleUpdateConfig updates the configuration (partial update).
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	// Get current config
	currentConfig := s.config.Get()

	// Parse partial update
	var partialUpdate map[string]any
	if err := json.NewDecoder(r.Body).Decode(&partialUpdate); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Apply partial updates
	worktreePatternChanged := false
	if worktree, ok := partialUpdate["worktree"].(map[string]any); ok {
		if pathPattern, ok := worktree["pathPattern"].(string); ok {
			if currentConfig.Worktree.PathPattern != pathPattern {
				currentConfig.Worktree.PathPattern = pathPattern
				worktreePatternChanged = true
			}
		}
	}

	if terminal, ok := partialUpdate["terminal"].(map[string]any); ok {
		if shell, ok := terminal["shell"].(string); ok {
			currentConfig.Terminal.Shell = shell
		}
		if rows, ok := terminal["rows"].(float64); ok {
			currentConfig.Terminal.Rows = int(rows)
		}
		if cols, ok := terminal["cols"].(float64); ok {
			currentConfig.Terminal.Cols = int(cols)
		}
	}

	if ui, ok := partialUpdate["ui"].(map[string]any); ok {
		if theme, ok := ui["theme"].(string); ok {
			currentConfig.UI.Theme = theme
		}
		if refreshInterval, ok := ui["refreshInterval"].(string); ok {
			currentConfig.UI.RefreshInterval = refreshInterval
		}
		if terminalLayout, ok := ui["terminalLayout"].(string); ok {
			// Validate terminal layout value
			if terminalLayout == "auto" || terminalLayout == "horizontal" || terminalLayout == "vertical" {
				currentConfig.UI.TerminalLayout = terminalLayout
			}
		}
	}

	// Save config
	if err := s.config.Save(currentConfig); err != nil {
		slog.Error("failed to save config", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Recreate worktree scanner if path pattern changed
	if worktreePatternChanged {
		// Close old scanner to release file descriptor
		oldScanner := s.getWorktreeScanner()
		if err := oldScanner.Close(); err != nil {
			slog.Warn("failed to close old worktree scanner", "error", err)
		}

		s.setWorktreeScanner(scanner.NewWorktreeScanner(currentConfig.RootPath, currentConfig.Worktree.PathPattern))
		slog.Debug("worktree scanner recreated with new path pattern", "pattern", currentConfig.Worktree.PathPattern)
	}

	response := models.APIResponse{
		Success: true,
		Message: "Configuration updated successfully",
	}

	respondJSON(w, response)
}
