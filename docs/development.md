# cclist Development Guide

This document is a practical guide for developing the cclist project.

## Table of Contents

- [Requirements](#requirements)
- [Development Workflow](#development-workflow)
- [Test Environment](#test-environment)
- [Coding Conventions](#coding-conventions)
- [Troubleshooting](#troubleshooting)

## Requirements

- Go (see `go.mod` for version)
- Git
- `claude` command (ClaudeCode CLI)
- chrome-devtools MCP (for functional testing)

## Development Workflow

### Available Make Commands

```bash
# Display available commands
make help

# Format code
make fmt

# Compile check
make vet

# Run all checks
make check

# Start development server in testdata sandbox
make dev

# Stop development server
make stop
```

### Basic Development Flow

1. **Edit code**
   ```bash
   vim internal/server/handlers.go
   ```

2. **Format**
   ```bash
   make fmt
   ```

3. **Run all checks**
   ```bash
   make check
   ```

4. **Functional testing**

   Ask Claude:
   ```
   Start the development server with make dev and verify the Web UI functionality using chrome-devtools
   ```

   Claude will:
   - Start server with `make dev` (inside testdata sandbox)
   - Operate browser with chrome-devtools MCP
   - Verify UI elements and interactions based on features described in [design.md](design.md)
   - Report results

   **Key verification items:**
   - Repository list display and filtering
   - Repository discovery and registration
   - Session start, resume, and termination
   - Terminal display and input/output
   - Worktree management (list, create, delete)
   - Shell terminal launch

   See [Web Interface Design](design.md#web-interface-design) for the complete feature list.

## Test Environment

Development is performed inside the **testdata** sandbox. `make dev` automatically sets up a clean environment.

**testdata structure:**
```
testdata/
├── README.md                # This file
├── .gitignore               # Git ignore settings
├── reset.sh                 # Reset script (cleanup + setup, auto-executed by make dev)
├── .cclist/                 # Config directory
│   └── config.json          # Empty config ({})
├── aaa/                     # Main repo with worktree
│   ├── .claude/
│   ├── .git/
│   └── README.md
├── aaa-feature/             # Worktree (feature branch)
│   ├── .claude/
│   ├── .git                 # Git worktree link
│   └── FEATURE.md
├── bbb/                     # Repo without worktree
│   ├── .claude/
│   ├── .git/
│   └── README.md
└── ccc/                     # Repo without .claude/
    ├── .git/
    └── README.md
```

**Test repositories:**
1. **aaa/** - Repository with worktree (discoverable)
2. **bbb/** - Repository without worktree (discoverable)
3. **ccc/** - Repository without `.claude/` (NOT discoverable)

## Coding Conventions

### File Naming

- Snake case: `directory.go`, `worktree.go`
- Clear responsibility: 1 file = 1 responsibility

### Package Structure

```go
// Internal package (internal/)
package scanner

// Public package (pkg/)
package models
```

### Import Order

goimports automatically formats:

```go
import (
    // Standard library
    "context"
    "fmt"
    "log/slog"

    // External packages
    "github.com/gorilla/websocket"

    // Local packages
    "github.com/daisuzu/cclist/internal/config"
    "github.com/daisuzu/cclist/pkg/models"
)
```

### Error Handling

```go
// ❌ Bad: No context
return nil, err

// ✅ Good: With context
return nil, fmt.Errorf("failed to scan repository %s: %w", repoPath, err)
```

**Important:** Do not use `os.Exit()` in library code (internal/). Return errors instead.

### Logging

This project uses `log/slog`.

#### Log Levels

```go
// Debug: Debug information during development (enabled with CCLIST_DEBUG=1)
slog.Debug("resized PTY", "columns", cols, "rows", rows, "session_id", sessionID)

// Info: Normal operational information
slog.Info("started session", "session_id", sessionID, "repository", repoPath)

// Warn: Warnings (processing continues but attention needed)
slog.Warn("failed to set initial PTY size", "error", err)

// Error: Errors (processing failed but can continue)
slog.Error("failed to scan repository", "path", repoPath, "error", err)
```

#### Structured Logging Format

```go
// ✅ Good: Structured with key-value pairs
slog.Info("started session",
    "session_id", sessionID,
    "repository", repoPath,
    "pid", cmd.Process.Pid)

// ❌ Bad: String concatenation
slog.Info(fmt.Sprintf("Started session %s for %s", sessionID, repoPath))
```

### Path Operations

```go
// ❌ Bad: Direct use of absolute paths
fullPath := "/Users/user/go/src/" + repoPath

// ✅ Good: Root-based path joining
fullPath := filepath.Join(s.root.Name(), repoPath)
```

### Comments

```go
// Public functions must have documentation comments
// Scanner handles directory scanning and ClaudeCode detection
type Scanner struct {
    root *os.Root
}

// NewScanner creates a new scanner instance
func NewScanner(rootPath string) *Scanner {
    // ...
}
```

## Troubleshooting

### Common Issues

#### 1. Port Conflict

```bash
# Error: listen tcp 127.0.0.1:12012: bind: address already in use

# Solution 1: Start on a different port
cclist --port 8080

# Solution 2: Stop existing process
lsof -ti:12012 | xargs kill -9
```

#### 2. Config File Location

```bash
# Check config path
cclist help

# Example output:
# Config file: ./.cclist/config.json or ~/.cclist/config.json
```

#### 3. Session Start Failure

**Symptom:** Error occurs when clicking session start button

**Cause and solution:**
- Verify `claude` command exists in PATH
  ```bash
  which claude
  ```
- Verify directory is under Root
  ```bash
  pwd  # Directory when server started
  ```

#### 4. WebSocket Connection Failure

**Symptom:** "WebSocket connection failed" displayed in terminal

**Cause and solution:**
- Check browser console for errors
- Check server logs (use `CCLIST_DEBUG=1` for details)
- Check firewall settings

#### 5. PTY Size Issues

**Symptom:** Terminal display is corrupted

**Cause and solution:**
- Wait for resize message to be sent after WebSocket connection
- Resize browser window to readjust

#### 6. goimports Not Found

```bash
# Install
go install golang.org/x/tools/cmd/goimports@latest

# Check path
which goimports
```

#### 7. staticcheck Not Found

```bash
# Install
go install honnef.co/go/tools/cmd/staticcheck@latest

# Check path
which staticcheck
```

### Debug Mode

```bash
# Enable debug logging
CCLIST_DEBUG=1 make dev

# Detailed logs will be output
# Example:
# DEBUG resized PTY columns=120 rows=30 session_id=abc123
# DEBUG sent initial prompt session_id=abc123
```

### Log Inspection

All server logs are output to standard output:

```bash
# Save logs to file
make dev 2>&1 | tee server.log

# Filter in real-time
make dev 2>&1 | grep ERROR
```

## Tips

### Development Efficiency

#### 1. API Testing with curl

```bash
# Repository list
curl http://localhost:12012/api/repositories | jq

# Get configuration
curl http://localhost:12012/api/config | jq

# Start session
curl -X POST http://localhost:12012/api/sessions \
  -H 'Content-Type: application/json' \
  -d '{"repositoryPath": "aaa", "prompt": "hello"}'
```

## Related Documents

- [design.md](design.md) - cclist Design Document
