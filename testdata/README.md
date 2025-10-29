# testdata - Development Sandbox

This directory is a development and testing sandbox for cclist.

## Setup

```bash
# From project root
make dev
```

`make dev` automatically runs `reset.sh` to build a clean environment.

## Test Repositories

1. **aaa/** - Repository with worktree (discoverable)
2. **bbb/** - Repository without worktree (discoverable)
3. **ccc/** - Repository without `.claude/` (NOT discoverable)

### Directory Structure

```
testdata/
├── README.md           # This file
├── .gitignore          # Git ignore settings
├── reset.sh            # Reset script (cleanup + setup)
├── .cclist/            # Config directory
│   └── config.json     # Empty config ({})
├── aaa/                # Main repo with worktree
│   ├── .claude/
│   ├── .git/
│   └── README.md
├── aaa-feature/        # Worktree (feature branch)
│   ├── .claude/
│   ├── .git            # Git worktree link
│   └── FEATURE.md
├── bbb/                # Repo without worktree
│   ├── .claude/
│   ├── .git/
│   └── README.md
└── ccc/                # Repo without .claude/
    ├── .git/
    └── README.md
```
