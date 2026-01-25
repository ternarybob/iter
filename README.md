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

## Testing

### Run All Tests

```bash
# Run all tests including Docker integration (requires Docker)
go test ./cmd/iter/... -v

# Run only unit tests (faster, no Docker required)
go test ./cmd/iter/... -v -short
```

### Test Coverage

**Unit Tests:**
- SKILL.md files have required `name` and `description` fields
- marketplace.json has correct structure with `skills` array
- plugin.json has required fields and no conflicting `skills` field
- Binary commands function correctly
- All expected skills exist

**Docker Integration Test** (`TestDockerIntegration`):
1. Builds Docker image with fresh Claude Code CLI
2. Adds the local marketplace
3. Installs the iter plugin
4. Validates settings and cache structure
5. Checks SKILL.md format for `name` field
6. Verifies marketplace.json has `skills` field
7. Tests the iter binary executes correctly
8. Checks plugin loading in Claude debug mode

### Run Docker Test Standalone

```bash
# Offline test (simulates skill execution)
go run ./test/docker/runner.go

# Full integration test with Claude API
ANTHROPIC_API_KEY=sk-... go run ./test/docker/runner.go
```

### Test Results

Test results are saved to `test/results/{timestamp}-{test-name}/`:
- `test-output.log` - Full test output
- `result.txt` - Pass/fail status and summary

### Full Integration Test

To test `/iter:run` in Claude with a live API:

```bash
# Via Go test
ANTHROPIC_API_KEY=sk-... go test ./cmd/iter/... -v -run TestDockerIntegration

# Via standalone runner
ANTHROPIC_API_KEY=sk-... go run ./test/docker/runner.go

# Via Docker directly
docker run -e ANTHROPIC_API_KEY=sk-... iter-plugin-test
```

### Manual Testing

After installation, verify the plugin works:

```bash
# Check plugin is installed
claude plugin list

# In Claude Code, type /iter and verify it appears in suggestions
# Run the command
/iter "test task"
```

## Project Structure

```
iter/
├── config/
│   ├── plugin.json              # Plugin manifest template
│   └── marketplace.json         # Marketplace manifest template
├── skills/                      # Skill definitions (source)
│   ├── iter/SKILL.md
│   ├── iter-workflow/SKILL.md
│   ├── iter-index/SKILL.md
│   └── iter-search/SKILL.md
├── hooks/hooks.json             # Stop hook
├── cmd/iter/main.go             # Binary source (all logic here)
├── scripts/build.sh             # Build script
└── bin/                         # Build output (plugin root)
    ├── .claude-plugin/
    │   ├── plugin.json          # Plugin manifest
    │   └── marketplace.json     # Marketplace manifest
    ├── iter                     # Compiled binary
    ├── skills/                  # Skill definitions
    └── hooks/                   # Hook definitions
```

## License

MIT
