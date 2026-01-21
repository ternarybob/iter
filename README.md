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

### Install (Persistent)

The build creates a local marketplace. Install with:

```bash
# Add the local marketplace (one-time)
claude plugin marketplace add /path/to/iter/bin

# Install the plugin
claude plugin install iter@iter-local
```

### Plugin Management

```bash
# Update after rebuilding
claude plugin update iter@iter-local

# Uninstall
claude plugin uninstall iter@iter-local

# Disable/enable without uninstalling
claude plugin disable iter@iter-local
claude plugin enable iter@iter-local
```

### Development Mode

For development without installing:

```bash
claude --plugin-dir /path/to/iter/bin/plugins/iter
```

## Commands

| Command | Description |
|---------|-------------|
| `/iter "<task>"` | Start iterative implementation |
| `/iter-workflow "<spec>"` | Start workflow-based implementation |

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
├── .claude-plugin/plugin.json   # Plugin manifest (source)
├── commands/                    # Command stubs (source)
│   ├── iter.md
│   └── iter-workflow.md
├── hooks/hooks.json             # Stop hook (source)
├── cmd/iter/main.go             # Binary source (all logic here)
├── scripts/build.sh             # Build script
└── bin/                         # Build output (marketplace format)
    ├── .claude-plugin/
    │   └── marketplace.json     # Marketplace manifest
    └── plugins/iter/            # Plugin package
        ├── .claude-plugin/plugin.json
        ├── commands/
        ├── hooks/
        └── iter                 # Compiled binary
```

## License

MIT
