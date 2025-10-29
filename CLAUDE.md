# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**cclist** is a CLI tool and web UI for managing ClaudeCode sessions across multiple repositories and git worktrees.

Full architecture and design: [docs/design.md](docs/design.md)
Development guide: [docs/development.md](docs/development.md)

## Essential Commands

```bash
make dev   # Start dev server in testdata/ sandbox
make stop  # Stop dev server (curl with debug-token-12345)
make fmt   # Format with goimports
make vet   # Compile check
make check # Run all checks
```

Always test with `make dev` - it auto-resets testdata/ sandbox with `reset.sh`.

## Architecture

- **main.go** - CLI (serve, discover, add, remove, list)
- **internal/config** - Config (`.cclist/config.json` or `~/.cclist/config.json`)
- **internal/scanner** - Repo/worktree scanning with `os.Root` security
- **internal/session** - ClaudeCode session + PTY via `creack/pty/v2`
- **internal/server** - HTTP/WebSocket (GoTTY protocol)

See [Architecture](docs/design.md#architecture) for details.

## Critical Conventions

**Error handling**: Always wrap errors with context using `fmt.Errorf("context: %w", err)`. Never use `os.Exit()` in `internal/` packages.

**Path operations**: Always use `os.Root` or `filepath.Join(s.root.Name(), path)` - never construct absolute paths directly.

**Logging**: Use `log/slog` with structured key-value pairs. Enable debug: `CCLIST_DEBUG=1`.

**Imports**: goimports auto-formats (stdlib → external → github.com/daisuzu/cclist/...).

See [Coding Conventions](docs/development.md#coding-conventions) for details.

## Key Design Points

- **Registration-based scanning**: Users explicitly register repos (not full dir walk). See [Design Philosophy](docs/design.md#design-philosophy).
- **Worktree strategy**: Parallel dirs `../repo-{branch}` for tool compatibility. See [Worktree Path Design](docs/design.md#worktree-path-design).
- **WebSocket**: GoTTY protocol - type `'1'`+Base64 for I/O, type `'3'`+JSON for resize. See [WebSocket Communication](docs/design.md#websocket-communication).

## Common Tasks

**Add API endpoint**: Add handler in `internal/server/*_handlers.go`, register in `setupRoutes()`.

**Modify session logic**: Edit `internal/session/manager.go` (StartSession/ResumeSession/monitorProcess).

**Debug**: `CCLIST_DEBUG=1 make dev` or `curl http://localhost:12012/api/repositories | jq`

See [Troubleshooting](docs/development.md#troubleshooting) for troubleshooting.
