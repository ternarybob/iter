# Iter

**Iter** is a Claude Code plugin for adversarial iterative implementation. It provides a structured loop that iterates until requirements and tests pass.

## Features

- **Self-contained binary**: All prompts and logic embedded in Go binary
- **Adversarial validation**: Default stance is REJECT - find problems
- **Structured iteration**: Architect → Worker → Validator loop
- **Exit blocking**: Session continues until complete or max iterations

## Installation

### Prerequisites

- Go 1.22+
- Claude Code CLI

### Build

```bash
./scripts/build.sh
```

### Install

```bash
# Add local marketplace (once)
claude plugin marketplace add /path/to/iter/bin

# Install plugin (choose scope)
claude plugin install iter@iter-local                  # user scope (default)
claude plugin install iter@iter-local --scope project  # project scope (shared via git)
claude plugin install iter@iter-local --scope local    # local scope (gitignored)
```

**Scopes:**
| Scope | Location | Use case |
|-------|----------|----------|
| `user` | `~/.claude/settings.json` | Personal, all projects (default) |
| `project` | `.claude/settings.json` | Team-shared via version control |
| `local` | `.claude/settings.local.json` | Project-specific, gitignored |

### Update

Plugin auto-updates on Claude restart when the version changes. To manually update:

```bash
claude plugin marketplace update iter-local
claude plugin update iter@iter-local
```

### Uninstall

```bash
# Uninstall plugin
claude plugin uninstall iter@iter-local

# Remove marketplace (optional)
claude plugin marketplace remove iter-local
```

## Commands

| Command | Description |
|---------|-------------|
| `/iter "<task>"` | Start iterative implementation |
| `/iter-workflow "<spec>"` | Start workflow-based implementation |
| `/iter-index` | Manage the code index (status, build, clear, watch) |
| `/iter-search "<query>"` | Search indexed code (semantic/keyword search) |

## How It Works

```
/iter "Add health check endpoint"
         │
         ▼
┌─────────────────┐
│    ARCHITECT    │  Analyze codebase, create step documents
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     WORKER      │  Implement step exactly as specified
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌────────┐
│    VALIDATOR    │────►│ REJECT │──► Back to WORKER
└────────┬────────┘     └────────┘
         │
         ▼ (PASS)
   Next step or COMPLETE
```

The Validator assumes ALL implementations are wrong until proven correct:

**Auto-reject**: Build fails, tests fail, requirements not traced, dead code remains
**Pass**: ALL checks verified, build passes, tests pass, cleanup complete

## Session Artifacts

Created in `.iter/workdir/`:

| File | Purpose |
|------|---------|
| `requirements.md` | Extracted requirements (R1, R2...) |
| `step_N.md` | Step specifications |
| `step_N_impl.md` | Implementation notes |
| `summary.md` | Completion summary |

## Project Structure

```
iter/
├── config/
│   ├── plugin.json              # Plugin manifest
│   └── marketplace.json         # Marketplace manifest
├── commands/                    # Command definitions
│   ├── iter.md
│   ├── iter-workflow.md
│   ├── iter-index.md
│   └── iter-search.md
├── hooks/hooks.json             # Stop hook
├── cmd/iter/main.go             # Binary source (all logic here)
├── scripts/build.sh             # Build script
└── bin/                         # Build output (plugin root)
    ├── .claude-plugin/
    │   └── marketplace.json     # Marketplace manifest
    ├── plugin.json              # Plugin manifest
    ├── iter                     # Compiled binary
    ├── commands/                # Command definitions
    └── hooks/                   # Hook definitions
```

## License

MIT
