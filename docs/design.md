# cclist Design Document

## Overview

**cclist** (ClaudeCode List) is a CLI tool for listing and managing ClaudeCode sessions. It detects repositories under the startup directory and provides the following operations via a web interface:

- Start, resume, and terminate ClaudeCode sessions
- Real-time display of terminal output (via WebSocket)
- Git worktree management (list, create, delete)
- Launch regular shell terminals

## Design Philosophy

### Registration-based Scanning

Walking through all directories under the root is not practical, so we adopt a **registration-based** approach:

- Users explicitly register repositories they want to manage
- Only registered repositories are scanned (fast startup)
- Worktrees are automatically detected from parent repositories via `git worktree list`
- Registration and removal are possible from both CLI and Web UI

**Benefits:**
- Fast startup (tens of milliseconds)
- No scanning of unnecessary directories
- Privacy protection (prevents access to unintended directories)
- Display only repositories users want to manage

**Registration methods:**
```bash
# Manual registration
cclist add github.com/user/repo

# Auto-detect and register
cclist discover
```

## Architecture

### Overall Structure

```
┌─────────────────────────────────────────────────────────────┐
│                         Browser (UI)                         │
│  - Display repository list                                   │
│  - Session management (start/resume/terminate)               │
│  - Terminal connection via WebSocket                         │
└────────────────┬────────────────────────────────────────────┘
                 │ HTTP/WebSocket
┌────────────────┴────────────────────────────────────────────┐
│                      HTTP Server                             │
│  - REST API endpoints                                        │
│  - WebSocket handler (GoTTY protocol)                        │
│  - Static file serving (embed.FS)                            │
└────┬───────────────┬──────────────┬────────────────────────┘
     │               │              │
┌────┴────┐  ┌──────┴──────┐  ┌────┴────────────┐
│ Scanner │  │   Session   │  │     Config      │
│         │  │   Manager   │  │     Manager     │
└────┬────┘  └──────┬──────┘  └─────────────────┘
     │               │
┌────┴───────────────┴─────────────────────────────────┐
│                    os.Root                            │
│  - Access restriction to startup directory           │
│  - Path traversal prevention                         │
└───────────────────────────────────────────────────────┘
```

### Components

#### 1. Entry Point

`main.go`

Main program entry point. Provides the following commands:

- `serve` / default: Start web server
- `discover`: Auto-detect repositories with `.claude/` directory under Root
- `add <path>`: Add repository to configuration
- `remove <path>`: Remove repository from configuration
- `list [--verbose]`: Display list of registered repositories
- `version`: Display version
- `help`: Display help

Port specification priority:
1. `--port` flag
2. `CCLIST_PORT` environment variable
3. config.json settings
4. Default value (12012)

**CLI usage examples:**

```bash
# Start server
$ cclist
Starting cclist server on http://127.0.0.1:12012
Shutdown token: a1b2c3d4e5f6...
Press Ctrl+C to stop, or use shutdown endpoint

# Auto-detect repositories
$ cclist discover
Scanning ./ (Root directory)...
Found 3 repositories with ClaudeCode history:
  - github.com/user/repo
  - github.com/user/another-repo
  - github.com/org/project

Add these repositories to config? [Y/n]: y
✓ Added 3 repositories to ./.cclist/config.json

# Register repository
$ cclist add github.com/user/repo
✓ Added repository: github.com/user/repo
  Worktree auto-detection: enabled

# Remove repository
$ cclist remove github.com/user/repo
Remove repository 'github.com/user/repo'? [y/N]: y
✓ Removed repository: github.com/user/repo

# List repositories
$ cclist list
Registered repositories (3):
  github.com/user/repo
  github.com/user/another-repo
  github.com/org/project

# Verbose display
$ cclist list --verbose
github.com/user/repo
  Path: ./github.com/user/repo (Root relative)
  Worktrees: 2 (main, feature-a)
  ClaudeCode History: Yes
  Last Active: 3m ago
```

#### 2. Configuration Management

`internal/config`

**Responsibilities:**
- Read and write configuration files
- Register and remove repositories
- Apply default values

**Configuration file priority:**
1. `./.cclist/config.json` (current directory) - Project-specific settings
2. `~/.cclist/config.json` (home directory) - Global settings

**Configuration file example:**

```json
{
  "rootPath": ".",
  "port": 12012,
  "repositories": [
    {
      "path": "github.com/user/repo",
      "autoDetectWorktrees": true
    },
    {
      "path": "github.com/user/another-repo",
      "autoDetectWorktrees": true
    }
  ],
  "worktree": {
    "pathPattern": "../{repo}-{branch}"
  },
  "terminal": {
    "shell": "/bin/zsh",
    "rows": 30,
    "cols": 120
  },
  "ui": {
    "theme": "dark",
    "refreshInterval": "5s"
  }
}
```

**Configuration structure (`models.Config`):**
```go
type Config struct {
    RootPath     string       // Root directory (pwd at startup)
    Port         int          // Server port (default: 12012)
    Repositories []Repository // Registered repository list
    Worktree     Worktree     // worktree settings
    Terminal     Terminal     // Terminal settings
    UI           UI           // UI settings
}
```

#### 3. Directory Scanner

`internal/scanner`

**Directory Scanner (`scanner.Scanner`):**
- Safe filesystem access using `os.Root`
- Detection of `.claude/` directories
- Retrieval of Git branch information
- Detection of ClaudeCode session information

**Worktree Scanner (`scanner.WorktreeScanner`):**
- List Git worktrees
- Retrieve branch information (local/remote)
- Create and delete worktrees

**Main methods:**
```go
// Scan repository
ScanRepository(repoPath string) (*models.Directory, error)

// Auto-detect repositories under Root
DiscoverRepositories() ([]string, error)

// List worktrees
ListWorktrees(repoPath string) ([]*models.WorktreeInfo, error)

// List branches
ListBranches(repoPath string) (*models.BranchInfo, error)

// Create worktree
CreateWorktree(repoPath, branch, baseBranch string,
               createBranch, fromRemote bool,
               customPath string) (string, error)
```

### Worktree Path Design

Default adopts **parallel directory approach**:

```
~/projects/github.com/user/
  ├── repo/              # main (parent repository)
  ├── repo-feature-a/    # worktree
  └── repo-feature-b/    # worktree
```

**Reasons for adoption:**
1. **Docker compatibility**: `Dockerfile` often assumes parent directory
2. **direnv compatibility**: Can place independent `.envrc` per worktree
3. **CI/CD compatibility**: GitHub Actions and others assume repository root
4. **Tool affinity**: Makefile, VSCode settings, etc. work independently
5. **go.mod**: Module-based approach, so no package path issues

**Option: `.worktree/` subdirectory approach also available**

```
~/projects/github.com/user/repo/
  ├── .git/
  ├── .worktree/
  │   ├── feature-a/    # worktree
  │   └── feature-b/    # worktree
  └── main.go           # main (parent repository)
```

**Implementation details** (internal/scanner/worktree.go:172-184):
- If `customPath` is a relative path, it's interpreted as relative to the repository directory
- If absolute path, use as-is
- When `customPath` is not specified, use default `../{repo}-{branch}` pattern

#### 4. Session Management

`internal/session`

**Responsibilities:**
- ClaudeCode session lifecycle management
- Shell terminal management
- PTY (pseudo-terminal) management
- Process monitoring

**Main functions:**

1. **ClaudeCode session:**
   - `StartSession()`: Start new session (`claude` command execution)
   - `ResumeSession()`: Resume session (`claude --resume`)
   - `SendPrompt()`: Send prompt
   - `TerminateSession()`: Terminate session

2. **Shell terminal:**
   - `StartShellTerminal()`: Launch regular shell terminal
   - `TerminateShellTerminal()`: Terminate terminal

3. **PTY management:**
   - `GetPTY()`: Get session's PTY file (for WebSocket connection)
   - Set PTY size (default: 30 rows × 120 columns)
   - Process monitoring (asynchronous monitoring via goroutine)

**Session data structure:**
```go
type Session struct {
    ID          string    // Session ID (generated with crypto/rand)
    Directory   string    // Working directory
    IsActive    bool      // Active state
    StartedAt   time.Time // Start time
    LastCommand time.Time // Last command time
    OutputPath  string    // Output log path
    ProcessID   int       // Process ID
    LastOutput  string    // Last output line
    ExitCode    *int      // Exit code
}
```

#### 5. HTTP Server

`internal/server`

**Server (`server.Server`):**
- HTTP server management
- Routing configuration
- Graceful shutdown support
- WebSocket connection management

**Main components:**
```go
type Server struct {
    config            *config.Manager
    scanner           *scanner.Scanner
    worktreeScanner   *scanner.WorktreeScanner
    sessionManager    *session.Manager
    httpServer        *http.Server
    shutdownToken     string  // Shutdown authentication token
    activeConnections map[string]*websocket.Conn
    cancelFuncs       map[string]context.CancelFunc
    connectionsMu     sync.RWMutex
}
```

**Static files:**
- Embedded with `//go:embed ui/*`
- Served using `embed.FS`

#### 6. REST API Endpoints

**Repository management:**
- `GET /api/repositories` - Get repository list
- `POST /api/repositories` - Register repository
- `DELETE /api/repositories/{path...}` - Remove repository
- `POST /api/repositories/discover` - Auto-detect repositories

**Directory management:**
- `GET /api/directories/{path...}` - Get directory details

**Configuration management:**
- `GET /api/config` - Get configuration
- `PUT /api/config` - Update configuration

**Session management:**
- `POST /api/sessions` - Start session
- `POST /api/sessions/resume` - Resume session
- `DELETE /api/sessions/{id}` - Terminate session

**Worktree management:**
- `GET /api/worktrees/{path...}` - Get worktree list
- `POST /api/worktrees/{path...}` - Create worktree
- `DELETE /api/worktrees/{path...}` - Delete worktree
- `GET /api/branches/{path...}` - Get branch list

**Terminal management:**
- `POST /api/terminal` - Create terminal
- `DELETE /api/terminal/{id}` - Delete terminal

**WebSocket:**
- `GET /ws/terminal/{id}` - Terminal WebSocket connection

**System:**
- `POST /api/shutdown` - Server shutdown (token authentication)

#### 7. WebSocket Communication

`internal/server/websocket.go`

**Protocol:**
Uses GoTTY-compatible protocol:

- Message types:
  - `'1'`: Input/output data
  - `'3'`: Terminal resize

- Data format:
  - Message type (1 byte) + Base64-encoded payload

**Communication flow:**

1. **PTY → WebSocket (output):**
   ```
   PTY read → Base64 encode → '1' + encoded → WebSocket send
   ```

2. **WebSocket → PTY (input):**
   ```
   WebSocket receive → Determine message type → Base64 decode → PTY write
   ```

3. **Resize:**
   ```json
   {
     "columns": 120,
     "rows": 30
   }
   ```

**Connection management:**
- One WebSocket connection allowed per session ID
- Existing connections automatically disconnected if present
- Graceful cancellation handling via context
- Bidirectional communication synchronization via sync.WaitGroup

#### 8. Data Models

`pkg/models`

**Main type definitions:**

```go
// Directory information
type Directory struct {
    Path             string
    RelativePath     string
    HasClaudeHistory bool
    ActiveSession    *Session
    IsWorktree       bool
    ParentPath       string
    Children         []*Directory
    GitBranch        string
    LastAccessed     time.Time
    FileCount        int
}

// Session information
type Session struct {
    ID          string
    Directory   string
    IsActive    bool
    StartedAt   time.Time
    LastCommand time.Time
    OutputPath  string
    ProcessID   int
    LastOutput  string
    ExitCode    *int
}

// Worktree information
type WorktreeInfo struct {
    Branch string
    Path   string
    IsMain bool
}

// Branch information
type BranchInfo struct {
    Local     []string
    Remote    []string
    Worktrees []string
}
```

## Security Design

### 1. Access Control via os.Root

**Role of os.Root:**
```go
root, err := os.OpenRoot(rootPath)
```

- Restricts access to under startup directory (pwd)
- Prevents parent directory access via path traversal (`../`)
- All file operations executed via Root

**Usage locations:**
- `scanner.Scanner` - Directory scanning
- `scanner.WorktreeScanner` - Worktree operations
- `session.Manager` - Session management

### 2. Shutdown Token Authentication

**Token generation:**
```go
// Cryptographically secure random generation using crypto/rand
func generateShutdownToken() (string, error) {
    b := make([]byte, 16)
    rand.Read(b)
    return hex.EncodeToString(b), nil
}
```

**Usage:**
```bash
curl -X POST http://127.0.0.1:12012/api/shutdown \
     -H 'X-Shutdown-Token: <token>'
```

Token is displayed at server startup and can be overridden with `CCLIST_SHUTDOWN_TOKEN` environment variable.

### 3. WebSocket Connection Management

- Origin check (only same-origin allowed)
- Connection limit per session ID (1 session = 1 connection)
- Graceful disconnection handling (context + WaitGroup)

## Implementation Features

### 1. Standard Library First

Use standard library whenever possible:
- `net/http` - HTTP server
- `log/slog` - Structured logging
- `encoding/json` - JSON processing
- `embed` - Static file embedding

External dependencies:
- `github.com/gorilla/websocket` - WebSocket communication
- `github.com/creack/pty/v2` - PTY management

### 2. Structured Logging

Structured logging via `log/slog`:

```go
slog.Info("started session",
    "session_id", sessionID,
    "repository", repoPath,
    "pid", cmd.Process.Pid)
```

Log level control:
- Enable debug logs with `CCLIST_DEBUG=1`
- Levels: Debug, Info, Warn, Error

### 3. Error Handling

```go
// Error with context
return nil, fmt.Errorf("failed to scan repository %s: %w", repoPath, err)
```

- Add context information to all errors
- Preserve error chain with `%w`
- Only `main` package exits with `fatalError()`

### 4. Concurrent Processing

**Goroutine usage locations:**

1. **Session management:**
   - Process monitoring (`monitorProcess`)
   - Initial prompt sending (delayed execution)

2. **WebSocket communication:**
   - PTY → WebSocket (read → send)
   - WebSocket → PTY (receive → write)

**Synchronization mechanisms:**
- `sync.RWMutex` - Session map protection
- `sync.WaitGroup` - Goroutine completion waiting
- `context.Context` - Cancellation propagation

## Web Interface Design

### Architecture

**SPA (Single Page Application):**
- No framework (Vanilla JavaScript)
- xterm.js (terminal emulation)
- Client-side routing

### Static Files Structure

```
internal/server/ui/
├── index.html           # SPA entry point
├── css/
│   └── style.css       # Stylesheet
└── js/
    └── app.js          # Frontend logic
```

**Embedded serving:**
```go
//go:embed ui/*
var uiFiles embed.FS
```

### Page Structure and Routing

**1. Home (`/`)**
- Repository list table
- Filter function (repository name, branch name)
- Sort function (update date, repository name, branch name)
- Actions: Start session, show repository details

**2. Repository details (`/repo/{path}`)**
- Display repository information
- Git worktree list
- Worktree operations (create, delete)
- Branch selection (local/remote)
- Launch shell terminal

**3. Session details (`/session/{id}`)**
- Display session information
- Terminal via xterm.js
- Real-time input/output via WebSocket
- Send prompt
- Terminate session

**4. Settings (`/settings`)**
- Display and edit settings
- Add and remove repositories
- Port settings, terminal settings, etc.

**SPA routing implementation:**
```javascript
navigate(path) {
    if (path === '/') {
        this.showRepositoryList();
    } else if (path.startsWith('/repo/')) {
        this.showRepositoryDetail(repoPath);
    } else if (path.startsWith('/session/')) {
        this.showSessionDetail(sessionId);
    } else if (path === '/settings') {
        this.showSettings();
    }
}
```

### Terminal Implementation

**xterm.js integration:**
- Version: 5.5.0 (stable, avoiding DOM renderer bug in 5.6.0-beta)
- Add-on: FitAddon (resize support)

**WebSocket protocol:**
Implements GoTTY-compatible protocol:
```javascript
// Send input: '1' + base64(data)
ws.send('1' + btoa(inputData));

// Send resize: '3' + JSON
ws.send('3' + JSON.stringify({columns: 120, rows: 30}));

// Receive output: '1' + base64(data)
const output = atob(message.substring(1));
term.write(output);
```

**Auto-resize:**
```javascript
const fitAddon = new FitAddon.FitAddon();
term.loadAddon(fitAddon);
fitAddon.fit();
window.addEventListener('resize', () => fitAddon.fit());
```

### UI Components

**CCListApp class:**
```javascript
class CCListApp {
    constructor() {
        this.currentPage = null;
        this.repositories = [];
        this.config = null;
        this.sortColumn = 'updated';
        this.sortDirection = 'desc';
    }

    // Page display
    showRepositoryList()
    showRepositoryDetail(repoPath)
    showSessionDetail(sessionId)
    showSettings()

    // API calls
    loadRepositories()
    loadConfig()
    discoverRepositories()

    // Terminal management
    terminalCleanup()
    shellTerminalCleanup()
}
```

### User Interaction

**Repository list:**
- Real-time filtering by filter input
- Sort by column header click
- Show details by row click
- Start session by button

**Terminal:**
- Keyboard input → WebSocket → PTY
- PTY → WebSocket → Terminal display
- Window resize → PTY size update
- Support for control characters like Ctrl+C

**Worktree management:**
- Branch selection (dropdown)
- New branch creation option
- Base branch specification
- Custom path specification

## Future Extensibility

**Session history:**
- Load history from `.claude/history/`
- Display history in UI

**Advanced worktree management:**
- Improved worktree auto-detection
- Custom path patterns

**Terminal feature enhancements:**
- Tabbed multiple terminals
- Screenshot function

**Integration features:**
- GitHub integration
- CI/CD integration

## Performance Considerations

### Efficient Scanning

- Efficient directory traversal with `fs.WalkDir`
- Skip unnecessary directories (`.git`, `vendor`, `node_modules`)
- Execute git commands only when necessary

### WebSocket Optimization

- Buffer size: 4096 bytes
- Base64 encoding
- Efficient cancellation via context

### Memory Management

- Management via session map
- Auto-cleanup of terminated sessions
- Avoid loading large files
