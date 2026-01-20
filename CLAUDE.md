# Iter - Claude Code Plugin

Iter is a **Claude Code plugin** that implements an adversarial multi-agent DevOps loop.
It runs within Claude Code using hooks and slash commands, with a Go binary for state management.

## Quick Start

```bash
# Build the CLI binary
go build -o bin/iter ./cmd/iter

# Install the plugin (from plugin directory)
claude --plugin-dir /path/to/iter

# Start an iter loop
/iter-loop "Add a health check endpoint to the API"
```

## Build Commands

```bash
go build -o bin/iter ./cmd/iter   # Build the CLI
go build ./...                     # Build all packages
go test ./...                      # Run tests
go test -race ./...                # Race detection
golangci-lint run                  # Lint
```

## Project Structure

```
github.com/ternarybob/iter/
├── .claude-plugin/
│   └── plugin.json         # Plugin manifest
├── commands/               # Slash commands
│   ├── iter-loop.md        # Start adversarial loop
│   ├── iter-analyze.md     # Run architect analysis
│   ├── iter-validate.md    # Run validator review
│   ├── iter-step.md        # Get step instructions
│   ├── iter-status.md      # Show session status
│   ├── iter-next.md        # Move to next step
│   ├── iter-complete.md    # Mark complete
│   └── iter-reset.md       # Reset session
├── hooks/
│   └── hooks.json          # Stop hook configuration
├── plugin-skills/
│   └── adversarial-devops.md  # Skill definition
├── cmd/
│   └── iter/
│       └── main.go         # CLI entry point
├── bin/
│   └── iter                # Compiled binary (build output)
├── pkg/                    # Go packages (library code)
│   ├── sdk/                # Public SDK interfaces
│   ├── agent/              # Core agent implementation
│   ├── orchestra/          # Multi-agent orchestration
│   ├── llm/                # LLM provider abstraction
│   ├── index/              # Codebase indexing
│   ├── config/             # Configuration
│   ├── session/            # Session management
│   └── monitor/            # Live monitoring
├── skills/                 # Go skill implementations
└── internal/               # Private utilities
```

## Plugin Commands

| Command | Description |
|---------|-------------|
| `/iter-loop "<task>"` | Start an adversarial multi-agent loop |
| `/iter-analyze` | Run architect analysis |
| `/iter-validate` | Run validator review |
| `/iter-step [N]` | Get step instructions |
| `/iter-status` | Show session status |
| `/iter-next` | Move to next step |
| `/iter-complete` | Mark session complete |
| `/iter-reset` | Reset session |

## Multi-Agent Architecture

### Architect Agent
- Analyzes requirements thoroughly
- Creates detailed step documents
- Identifies cleanup targets
- Outputs: `requirements.md`, `step_N.md`, `architect-analysis.md`

### Worker Agent (Claude Code)
- Implements steps EXACTLY as specified
- No interpretation or deviation
- Verifies build after each change
- Writes `step_N_impl.md` after implementation

### Validator Agent
- **DEFAULT STANCE: REJECT**
- Must find problems, not confirm success
- Auto-reject on build failure
- Auto-reject on missing requirements traceability
- Writes `step_N_valid.md` with verdict

## Workflow

```
┌─────────────┐
│  ARCHITECT  │ ─── /iter-analyze creates requirements.md, step_N.md
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   WORKER    │ ─── /iter-step implements step exactly
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌─────────┐
│  VALIDATOR  │ ──► │ REJECT  │ ──► Back to WORKER
└──────┬──────┘     └─────────┘
       │
       ▼ (PASS)
┌─────────────┐
│  Next Step  │ ──► /iter-next, repeat until all steps done
└─────────────┘
       │
       ▼
┌─────────────┐
│  COMPLETE   │ ─── /iter-complete writes summary.md
└─────────────┘
```

## Execution Rules

### Absolutes
- CORRECTNESS over SPEED - never rush validation
- Requirements are LAW - no interpretation or deviation
- EXISTING PATTERNS ARE LAW - match codebase style exactly
- CLEANUP IS MANDATORY - remove dead/redundant code
- STEPS ARE MANDATORY - no implementation without step docs
- BUILD VERIFICATION IS MANDATORY - verify after each change

### Exit Detection
- Session completes when `/iter-complete` is run
- Or when max iterations is reached
- Stop hook blocks exit during active session

## CLI Binary Commands

The Go binary (`bin/iter`) provides:

```bash
iter init "<task>" [--max-iterations N]  # Start session
iter check                               # Check completion (for hook)
iter analyze                             # Output architect prompt
iter validate                            # Output validator prompt
iter status                              # Show session status
iter step [N]                            # Get step instructions
iter next                                # Move to next step
iter complete                            # Mark complete
iter reset                               # Reset session
iter reject "<reason>"                   # Record rejection
iter pass                                # Record pass
iter hook-stop                           # Stop hook handler (JSON output)
```

## Session State

State is persisted in `.iter/` directory:
- `.iter/state.json` - Session state
- `.iter/workdir/` - Artifacts (requirements, steps, etc.)

## Hook Configuration

The stop hook (`hooks/hooks.json`) intercepts session exit and:
1. Checks if iter session is active
2. If not complete, blocks exit and injects continuation prompt
3. If complete or max iterations reached, allows exit

## Code Style (for Go packages)

- Use slog for structured logging
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Use functional options for configuration
- No `fmt.Println` (use logger)
- No global mutable state

## Testing

```bash
# Unit tests
go test ./...

# With race detection
go test -race ./...

# Specific package
go test ./cmd/iter/...
```

## Development

### Testing the plugin locally

```bash
# Build the binary
go build -o bin/iter ./cmd/iter

# Test with Claude Code
claude --plugin-dir .
```

### Adding a new command

1. Create `commands/<command-name>.md`
2. Add YAML frontmatter with description and arguments
3. Document the command behavior

### Modifying hook behavior

1. Edit `hooks/hooks.json`
2. Update `cmd/iter/main.go` for new CLI commands
3. Rebuild the binary
