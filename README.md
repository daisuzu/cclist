# cclist

ClaudeCode sessions and git worktrees manager with Web UI

## Overview

**cclist** is a CLI tool and web interface for managing ClaudeCode sessions across multiple repositories and git worktrees. It provides a centralized dashboard to:

- Start, resume, and terminate ClaudeCode sessions
- View terminal output in real-time via WebSocket
- Manage git worktrees (list, create, delete)
- Launch regular shell terminals
- Auto-discover repositories with ClaudeCode history

## Installation

```bash
go install github.com/daisuzu/cclist@latest
```

## Quick Start

```bash
# Auto-discover repositories with ClaudeCode history
cclist discover

# Start web server (default port: 12012)
cclist

# Open browser
open http://127.0.0.1:12012
```

## CLI Commands

### Server

```bash
# Start web server (default command)
cclist
cclist serve

# Start on custom port
cclist --port 8080
CCLIST_PORT=8080 cclist
```

### Repository Management

```bash
# Auto-discover repositories with ClaudeCode history
cclist discover

# Manually add repository
cclist add github.com/user/repo

# Add multiple repositories
cclist add github.com/user/repo1 github.com/user/repo2

# Remove repository
cclist remove github.com/user/repo

# List registered repositories
cclist list
cclist list --verbose
```

### Other Commands

```bash
cclist version    # Show version
cclist help       # Show help
```

## Configuration

Configuration file location (priority order):
1. `./.cclist/config.json` (project-specific)
2. `~/.cclist/config.json` (global)

### Port Configuration

Priority order:
1. `--port` flag
2. `CCLIST_PORT` environment variable
3. config.json setting
4. Default (12012)

### Example Configuration

```json
{
  "rootPath": ".",
  "port": 12012,
  "repositories": [
    {
      "path": "github.com/user/repo",
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

### Configuration Options

**repositories**: List of registered repositories
- `path`: Repository path (relative to rootPath)
- `autoDetectWorktrees`: Automatically detect git worktrees (default: true)

**worktree**: Git worktree settings
- `pathPattern`: Path pattern for new worktrees (default: `../{repo}-{branch}`)

**terminal**: Terminal settings
- `shell`: Shell command (default: `/bin/zsh` or `/bin/bash`)
- `rows`: Terminal rows (default: 30)
- `cols`: Terminal columns (default: 120)

**ui**: Web UI settings
- `theme`: UI theme (`dark` or `light`)
- `refreshInterval`: Auto-refresh interval (e.g., `5s`, `10s`)

## Web UI

Access the web interface at `http://127.0.0.1:12012`

### Features

**Repository List**
- View all registered repositories and worktrees
- Filter by repository name or branch
- Sort by last update, repository name, or branch name
- Start new ClaudeCode sessions

**Repository Detail**
- View repository information
- List git worktrees
- Create new worktrees from local or remote branches
- Delete worktrees
- Launch shell terminals

**Session Terminal**
- Real-time terminal output via WebSocket
- Send prompts to ClaudeCode
- Resume existing sessions
- Terminate sessions

**Settings**
- View and edit configuration
- Add/remove repositories
- Discover repositories automatically

## Server Shutdown

Press `Ctrl+C` to stop the server gracefully.

Alternatively, you can use the shutdown endpoint with the token displayed at startup:

```bash
curl -X POST http://127.0.0.1:12012/api/shutdown \
     -H 'X-Shutdown-Token: <token-from-startup>'
```

## Troubleshooting

### Port Already in Use

If port 12012 is already in use:

```bash
# Use a different port
cclist --port 8080

# Or kill the existing process
lsof -ti:12012 | xargs kill -9
```

### Session Start Failure

If ClaudeCode sessions fail to start:

1. Check if `claude` command is in your PATH:
   ```bash
   which claude
   ```

2. Verify the repository path is within the root directory (where cclist was started)

3. Check server logs for error messages

### WebSocket Connection Issues

If the terminal fails to connect:

1. Check browser console for errors (F12 → Console)
2. Verify the server is running
3. Check firewall settings

### Debug Mode

Enable debug logging for detailed output:

```bash
CCLIST_DEBUG=1 cclist
```

### Configuration Issues

Check which config file is being used:

```bash
cclist help
# Shows: Config file: ./.cclist/config.json or ~/.cclist/config.json
```

## Requirements

- Go
- Git
- Claude Code

## Contributing

For development documentation, see:
- [Design Documentation](docs/design.md)
- [Development Guide](docs/development.md)

## License

MIT License - see [LICENSE](LICENSE) file for details
