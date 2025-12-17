package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daisuzu/cclist/pkg/models"
)

const (
	defaultConfigDir      = ".cclist"
	defaultConfigFile     = "config.json"
	sessionOutputsFile    = "session-outputs.json"
	defaultPort           = 12012
	defaultShell          = "/bin/sh"
	defaultRows           = 30
	defaultCols           = 120
	defaultTheme          = "dark"
	defaultRefresh        = "5s"
	defaultPathPattern    = "../{repo}-{branch}"
	defaultTerminalLayout = "auto"
)

// Manager handles configuration loading and saving.
type Manager struct {
	configPath string
	config     *models.Config
}

// NewManager creates a new configuration manager
// Priority: 1. ./.cclist/config.json (current directory)
//  2. ~/.cclist/config.json (home directory)
func NewManager() (*Manager, error) {
	// Check current directory first
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	localConfigPath := filepath.Join(pwd, defaultConfigDir, defaultConfigFile)
	if _, err := os.Stat(localConfigPath); err == nil {
		// Local config exists, use it
		return &Manager{
			configPath: localConfigPath,
		}, nil
	}

	// Fall back to home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	globalConfigPath := filepath.Join(homeDir, defaultConfigDir, defaultConfigFile)
	return &Manager{
		configPath: globalConfigPath,
	}, nil
}

// Load loads the configuration from file, or creates default if not exists.
func (m *Manager) Load() (*models.Config, error) {
	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Create default config
		config := m.createDefaultConfig()
		m.config = config
		return config, nil
	}

	// Read config file
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config models.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply default values for empty fields
	m.applyDefaults(&config)

	m.config = &config
	return &config, nil
}

// Save saves the configuration to file.
func (m *Manager) Save(config *models.Config) error {
	// Ensure config directory exists
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	m.config = config
	return nil
}

// Get returns the current configuration.
func (m *Manager) Get() *models.Config {
	return m.config
}

// GetConfigPath returns the path to the config file.
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// createDefaultConfig creates a default configuration.
func (m *Manager) createDefaultConfig() *models.Config {
	// Detect default shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = defaultShell
	}

	return &models.Config{
		RootPath:     ".", // Current directory
		Port:         defaultPort,
		Repositories: []models.Repository{},
		Worktree: models.Worktree{
			PathPattern: defaultPathPattern,
		},
		Terminal: models.Terminal{
			Shell: shell,
			Rows:  defaultRows,
			Cols:  defaultCols,
		},
		UI: models.UI{
			Theme:           defaultTheme,
			RefreshInterval: defaultRefresh,
			TerminalLayout:  defaultTerminalLayout,
		},
	}
}

// applyDefaults applies default values to empty fields in the config.
func (m *Manager) applyDefaults(config *models.Config) {
	if config.RootPath == "" {
		config.RootPath = "."
	}
	if config.Port == 0 {
		config.Port = defaultPort
	}
	if config.Repositories == nil {
		config.Repositories = []models.Repository{}
	}
	if config.Worktree.PathPattern == "" {
		config.Worktree.PathPattern = defaultPathPattern
	}
	if config.Terminal.Shell == "" {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = defaultShell
		}
		config.Terminal.Shell = shell
	}
	if config.Terminal.Rows == 0 {
		config.Terminal.Rows = defaultRows
	}
	if config.Terminal.Cols == 0 {
		config.Terminal.Cols = defaultCols
	}
	if config.UI.Theme == "" {
		config.UI.Theme = defaultTheme
	}
	if config.UI.RefreshInterval == "" {
		config.UI.RefreshInterval = defaultRefresh
	}
	if config.UI.TerminalLayout == "" {
		config.UI.TerminalLayout = defaultTerminalLayout
	}
}

// AddRepository adds a repository to the configuration.
func (m *Manager) AddRepository(path string, autoDetectWorktrees bool) error {
	if m.config == nil {
		return fmt.Errorf("config not loaded")
	}

	// Check if repository already exists
	for _, repo := range m.config.Repositories {
		if repo.Path == path {
			return fmt.Errorf("repository already exists: %s", path)
		}
	}

	// Add repository
	m.config.Repositories = append(m.config.Repositories, models.Repository{
		Path:                path,
		AutoDetectWorktrees: autoDetectWorktrees,
	})

	// Save config
	return m.Save(m.config)
}

// RemoveRepository removes a repository from the configuration.
func (m *Manager) RemoveRepository(path string) error {
	if m.config == nil {
		return fmt.Errorf("config not loaded")
	}

	// Find and remove repository
	found := false
	newRepos := make([]models.Repository, 0, len(m.config.Repositories))
	for _, repo := range m.config.Repositories {
		if repo.Path != path {
			newRepos = append(newRepos, repo)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("repository not found: %s", path)
	}

	m.config.Repositories = newRepos

	// Save config
	return m.Save(m.config)
}

// ListRepositories returns all registered repositories.
func (m *Manager) ListRepositories() []models.Repository {
	if m.config == nil {
		return nil
	}

	return m.config.Repositories
}

// sessionOutputsPath returns the path to session-outputs.json.
func (m *Manager) sessionOutputsPath() string {
	return filepath.Join(filepath.Dir(m.configPath), sessionOutputsFile)
}

// LoadSessionOutputs loads the session outputs cache from file.
func (m *Manager) LoadSessionOutputs() (map[string]*models.SessionOutputCache, error) {
	path := m.sessionOutputsPath()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]*models.SessionOutputCache), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read session outputs file: %w", err)
	}

	var cache map[string]*models.SessionOutputCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse session outputs file: %w", err)
	}

	return cache, nil
}

// SaveSessionOutputs saves the session outputs cache to file.
func (m *Manager) SaveSessionOutputs(cache map[string]*models.SessionOutputCache) error {
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session outputs: %w", err)
	}

	path := m.sessionOutputsPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session outputs file: %w", err)
	}

	return nil
}
