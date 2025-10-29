package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/daisuzu/cclist/internal/config"
	"github.com/daisuzu/cclist/internal/logger"
	"github.com/daisuzu/cclist/internal/scanner"
	"github.com/daisuzu/cclist/internal/server"
)

const version = "0.1.0"

func main() {
	// Initialize slog
	logger.Init()

	if len(os.Args) < 2 {
		// Default: start server
		runServer()
		return
	}

	command := os.Args[1]

	// If first argument is a flag, default to server command
	if command == "--port" {
		runServer()
		return
	}

	switch command {
	case "serve", "server":
		runServer()
	case "discover":
		runDiscover()
	case "add":
		runAdd()
	case "remove":
		runRemove()
	case "list":
		runList()
	case "version", "--version", "-v":
		fmt.Printf("cclist version %s\n", version)
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

// fatalError logs an error and exits the program with status code 1.
// This should ONLY be used in the main package for unrecoverable errors
// during initialization or startup. Never use this in library code (internal/).
func fatalError(msg string, err error) {
	slog.Error(msg, "error", err)
	os.Exit(1)
}

// getPortOverride checks for port override from command line or environment variable.
// Priority:
//  1. --port flag
//  2. CCLIST_PORT env
//  3. config file
func getPortOverride() (int, bool) {
	// Check command line flags
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--port" && i+1 < len(os.Args) {
			port, err := strconv.Atoi(os.Args[i+1])
			if err == nil && port > 0 && port <= 65535 {
				return port, true
			}
			slog.Warn("invalid port number", "value", os.Args[i+1])
		}
	}

	// Check environment variable
	if portStr := os.Getenv("CCLIST_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err == nil && port > 0 && port <= 65535 {
			return port, true
		}
		slog.Warn("invalid CCLIST_PORT", "value", portStr)
	}

	return 0, false
}

func runServer() {
	// Load config
	cfg, err := config.NewManager()
	if err != nil {
		fatalError("failed to create config manager", err)
	}

	appConfig, err := cfg.Load()
	if err != nil {
		fatalError("failed to load config", err)
	}

	// Apply port override if specified
	if port, ok := getPortOverride(); ok {
		appConfig.Port = port
		slog.Debug("port override applied", "port", port)
	}

	// Create server
	srv := server.NewServer(cfg)

	// Handle graceful shutdown with context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatalError("server failed", err)
		}
	}()

	// User-friendly output
	fmt.Printf("cclist v%s started on http://127.0.0.1:%d\n", version, appConfig.Port)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Printf("To shutdown: curl -X POST http://127.0.0.1:%d/api/shutdown -H 'X-Shutdown-Token: %s'\n",
		appConfig.Port, srv.GetShutdownToken())

	// Wait for shutdown signal (either OS signal or API shutdown)
	select {
	case <-ctx.Done():
		// Shutdown triggered by OS signal (Ctrl+C, SIGTERM)
		slog.Debug("received OS shutdown signal")
	case <-srv.ShutdownChan():
		// Shutdown triggered by API
		slog.Debug("received API shutdown signal")
	}

	// Restore default signal behavior
	stop()

	// Shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		fatalError("server shutdown failed", err)
	}

	fmt.Println("Server stopped")
}

func runDiscover() {
	cfg, err := config.NewManager()
	if err != nil {
		fatalError("failed to create config manager", err)
	}

	appConfig, err := cfg.Load()
	if err != nil {
		fatalError("failed to load config", err)
	}

	s := scanner.NewScanner(appConfig.RootPath)

	fmt.Printf("Scanning %s (Root directory)...\n", appConfig.RootPath)

	discovered, err := s.DiscoverRepositories()
	if err != nil {
		fatalError("failed to discover repositories", err)
	}

	if len(discovered) == 0 {
		fmt.Println("No repositories with ClaudeCode history found.")
		return
	}

	fmt.Printf("Found %d repositories with ClaudeCode history:\n", len(discovered))
	for _, repo := range discovered {
		fmt.Printf("  - %s\n", repo)
	}

	// Ask user if they want to add them
	fmt.Print("\nAdd these repositories to config? [Y/n]: ")
	var response string
	_, _ = fmt.Scanln(&response) // Ignore error; empty input is acceptable

	if response == "" || response == "y" || response == "Y" {
		added := 0
		for _, repo := range discovered {
			err := cfg.AddRepository(repo, true)
			if err != nil {
				fmt.Printf("  ⚠ Failed to add %s: %v\n", repo, err)
			} else {
				added++
			}
		}
		fmt.Printf("✓ Added %d repositories to %s\n", added, cfg.GetConfigPath())
	} else {
		fmt.Println("Skipped adding repositories.")
	}
}

func runAdd() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: cclist add <repository-path> [<repository-path>...]")
		os.Exit(1)
	}

	cfg, err := config.NewManager()
	if err != nil {
		fatalError("failed to create config manager", err)
	}

	if _, err := cfg.Load(); err != nil {
		fatalError("failed to load config", err)
	}

	repos := os.Args[2:]
	added := 0

	for _, repo := range repos {
		err := cfg.AddRepository(repo, true)
		if err != nil {
			fmt.Printf("⚠ Failed to add %s: %v\n", repo, err)
		} else {
			fmt.Printf("✓ Added repository: %s\n", repo)
			added++
		}
	}

	if added > 0 {
		fmt.Printf("✓ Added %d repositories to %s\n", added, cfg.GetConfigPath())
	}
}

func runRemove() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: cclist remove <repository-path>")
		os.Exit(1)
	}

	cfg, err := config.NewManager()
	if err != nil {
		fatalError("failed to create config manager", err)
	}

	if _, err := cfg.Load(); err != nil {
		fatalError("failed to load config", err)
	}

	repo := os.Args[2]

	// Confirmation
	fmt.Printf("Remove repository '%s'? [y/N]: ", repo)
	var response string
	_, _ = fmt.Scanln(&response) // Ignore error; empty input is acceptable

	if response == "y" || response == "Y" {
		err := cfg.RemoveRepository(repo)
		if err != nil {
			fmt.Printf("⚠ Failed to remove repository: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Removed repository: %s\n", repo)
	} else {
		fmt.Println("Cancelled.")
	}
}

func runList() {
	cfg, err := config.NewManager()
	if err != nil {
		fatalError("failed to create config manager", err)
	}

	appConfig, err := cfg.Load()
	if err != nil {
		fatalError("failed to load config", err)
	}

	repos := cfg.ListRepositories()

	if len(repos) == 0 {
		fmt.Println("No repositories registered.")
		fmt.Println("Run 'cclist discover' to find repositories automatically.")
		return
	}

	fmt.Printf("Registered repositories (%d):\n", len(repos))

	// Check if verbose mode
	verbose := len(os.Args) > 2 && (os.Args[2] == "-v" || os.Args[2] == "--verbose")

	if verbose {
		s := scanner.NewScanner(appConfig.RootPath)
		ws := scanner.NewWorktreeScanner(appConfig.RootPath, appConfig.Worktree.PathPattern)

		for _, repo := range repos {
			fmt.Printf("\n%s\n", repo.Path)

			fullPath := filepath.Join(appConfig.RootPath, repo.Path)
			fmt.Printf("  Path: %s (Root relative)\n", fullPath)

			// Get directory info
			dir, err := s.ScanRepository(repo.Path)
			if err == nil {
				fmt.Printf("  Branch: %s\n", dir.GitBranch)
				fmt.Printf("  ClaudeCode History: %v\n", dir.HasClaudeHistory)
				if dir.ActiveSession != nil {
					fmt.Printf("  Active Session: %v\n", dir.ActiveSession.IsActive)
				}
			}

			// Get worktrees
			if repo.AutoDetectWorktrees {
				worktrees, err := ws.ListWorktrees(repo.Path)
				if err == nil && len(worktrees) > 0 {
					fmt.Printf("  Worktrees: %d (", len(worktrees))
					for i, wt := range worktrees {
						if i > 0 {
							fmt.Print(", ")
						}
						fmt.Print(wt.Branch)
					}
					fmt.Println(")")
				}
			}
		}
	} else {
		for _, repo := range repos {
			fmt.Printf("  %s\n", repo.Path)
		}
	}
}

func printHelp() {
	fmt.Printf("cclist v%s - ClaudeCode List\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  cclist [command] [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  (none)           Start the web server (default)")
	fmt.Println("  serve            Start the web server")
	fmt.Println("  discover         Auto-discover repositories with .claude/ directory")
	fmt.Println("  add <path>       Add repository to config")
	fmt.Println("  remove <path>    Remove repository from config")
	fmt.Println("  list             List registered repositories")
	fmt.Println("  list --verbose   List registered repositories with details")
	fmt.Println("  version          Show version information")
	fmt.Println("  help             Show this help message")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --port <port>    Override port number (default: 12012)")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  CCLIST_PORT      Override port number")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  cclist                                    # Start web server")
	fmt.Println("  cclist --port 8080                        # Start on port 8080")
	fmt.Println("  CCLIST_PORT=8080 cclist                   # Start on port 8080 (env)")
	fmt.Println("  cclist discover                           # Find repositories")
	fmt.Println("  cclist add github.com/user/repo           # Add repository")
	fmt.Println("  cclist remove github.com/user/repo        # Remove repository")
	fmt.Println("  cclist list --verbose                     # List with details")
	fmt.Println()

	// Show actual config file path that will be used
	cfg, err := config.NewManager()
	if err == nil {
		fmt.Printf("Config file: %s\n", cfg.GetConfigPath())
	} else {
		fmt.Println("Config file: ./.cclist/config.json or ~/.cclist/config.json")
	}
}
