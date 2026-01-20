# Iter

**Iter** (Latin: "journey, path") is a **Claude Code plugin** that implements an adversarial multi-agent DevOps loop.

It provides a rigorous Architect → Worker → Validator feedback cycle for complex development tasks, with the Validator taking an adversarial stance (default: REJECT) to ensure correctness.

## Features

- **Claude Code Plugin**: Integrates directly with Claude Code via hooks and slash commands
- **Adversarial Validation**: Multi-agent architecture with hostile review (default REJECT)
- **Go-Powered CLI**: State management and prompt generation via compiled Go binary
- **Correctness over Speed**: Requirements are law, validation is mandatory
- **Self-Referential Loop**: Stop hook creates continuous improvement cycle

## Quick Start

```bash
# Clone the repository
git clone https://github.com/ternarybob/iter.git
cd iter

# Build the plugin binary
./scripts/build.sh

# Use with Claude Code
claude --plugin-dir /path/to/iter

# Start an adversarial loop
/iter-loop "Add a health check endpoint to the API"
```

## Prerequisites

- **Go 1.22+** - [Download Go](https://go.dev/dl/)

## Plugin Commands

| Command | Description |
|---------|-------------|
| `/iter-loop "<task>"` | Start an adversarial multi-agent loop |
| `/iter-analyze` | Run architect analysis |
| `/iter-validate` | Run validator review (adversarial) |
| `/iter-step [N]` | Get step instructions |
| `/iter-status` | Show session status |
| `/iter-next` | Move to next step |
| `/iter-complete` | Mark session complete |
| `/iter-reset` | Reset session |

## Multi-Agent Architecture

```
┌─────────────┐
│  ARCHITECT  │ ─── Analyzes requirements, creates step documents
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   WORKER    │ ─── Implements step exactly (you, via Claude Code)
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────┐
│  VALIDATOR  │ ──► │ REJECT  │ ──► Back to WORKER (fix issues)
└──────┬──────┘     └─────────┘
       │
       ▼ (PASS)
┌─────────────┐
│  Next Step  │ ──► Repeat until all steps done
└─────────────┘
```

| Agent | Role | Stance |
|-------|------|--------|
| **ARCHITECT** | Analyze requirements, create step docs | Thorough, comprehensive |
| **WORKER** | Implement steps exactly as specified | Follow spec precisely |
| **VALIDATOR** | Review implementation against requirements | **HOSTILE - default REJECT** |

## Adversarial Validation

The Validator assumes ALL implementations are incorrect until proven otherwise:

**Auto-reject conditions:**
- Build fails
- Tests fail
- Requirements not traceable to code
- Dead code left behind
- Missing cleanup as specified

**Pass conditions (ALL must be true):**
- ALL requirements verified with code references
- Build passes
- Tests pass
- No dead code
- Cleanup verified

## Workdir Artifacts

Each execution creates artifacts in `.iter/workdir/`:

| File | Purpose |
|------|---------|
| `requirements.md` | Extracted requirements |
| `architect-analysis.md` | Patterns, decisions |
| `step_N.md` | Step specifications |
| `step_N_impl.md` | Implementation notes |
| `step_N_valid.md` | Validation results |
| `summary.md` | **MANDATORY** completion summary |

## Project Structure

```
github.com/ternarybob/iter/
├── .claude-plugin/
│   └── plugin.json         # Plugin manifest
├── commands/               # Slash commands
│   ├── iter-loop.md
│   ├── iter-analyze.md
│   ├── iter-validate.md
│   └── ...
├── hooks/
│   └── hooks.json          # Stop hook configuration
├── plugin-skills/
│   └── adversarial-devops.md
├── cmd/iter/
│   └── main.go             # CLI binary source
├── bin/
│   └── iter                # Compiled binary
└── scripts/
    └── build.sh            # Build script
```

## Development

### Building

```bash
# Build the binary
./scripts/build.sh

# Or build manually
go build -o bin/iter ./cmd/iter
```

### Testing the Plugin

```bash
# Use with Claude Code
claude --plugin-dir /path/to/iter
```

## License

MIT License - see [LICENSE](LICENSE)
