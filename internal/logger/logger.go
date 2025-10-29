package logger

import (
	"log/slog"
	"os"
)

// Init initializes the default slog logger with appropriate configuration.
// The log level is controlled by the CCLIST_DEBUG environment variable:
//   - CCLIST_DEBUG=1: Debug level (verbose)
//   - otherwise: Info level (default)
//
// Log level usage guidelines:
//   - Debug: Development debugging (internal state, detailed flow)
//   - Info:  Normal operations (server start, session events, config changes)
//   - Warn:  Warnings (processing continues but attention needed)
//   - Error: Errors (operation failed but can continue)
//
// For fatal errors that require program termination, use the fatalError()
// helper in main.go (available only in main package, not in library code).
func Init() {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: getLogLevel(),
	})
	slog.SetDefault(slog.New(handler))
}

func getLogLevel() slog.Level {
	if os.Getenv("CCLIST_DEBUG") != "" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
