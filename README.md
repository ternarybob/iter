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
| `/iter:workflow "<spec>"` | Start workflow-based implementation |
| `/iter:test <test-file> [tests...]` | Run tests with auto-fix iteration (max 10x) |
| `/iter:index` | Manage the code index (status, build, clear, watch) |
| `/iter:search "<query>"` | Search indexed code (semantic/keyword search) |
| `/iter:install` | Install `/iter` shortcut wrapper |

## Skills

Iter provides six specialized skills for different aspects of iterative development:

### /iter:run - Core Iterative Implementation

The main skill for implementing features, fixing bugs, and refactoring code with high quality standards.

**Purpose**: Execute a structured three-phase workflow (ARCHITECT → WORKER → VALIDATOR) that iterates until all requirements and tests pass.

**When to use**:
- Implementing new features
- Fixing bugs
- Refactoring code
- Any code changes requiring high quality

**Key features**:
- Adversarial validation (default: REJECT)
- Task management for ordered execution
- Git worktree isolation
- Requirements traceability
- Automatic cleanup enforcement

**Example**: `/iter:run "add health check endpoint at /health"`

[→ Full documentation](skills/run/README.md)

### /iter:workflow - Custom Workflow Execution

Execute custom workflow specifications for specialized iterative processes.

**Purpose**: Run domain-specific workflows with custom phases, priorities, and success criteria beyond the standard implementation pattern.

**When to use**:
- Service stabilization and debugging
- Performance optimization
- Multi-iteration system tuning
- Custom iteration logic with specific priority rules

**Key features**:
- Custom workflow specifications (from file or inline)
- Priority-based issue selection
- Multi-phase iteration (ARCHITECT/WORKER/VALIDATOR/COMPLETE)
- Configurable iteration limits and stabilization periods
- Comprehensive documentation per iteration

**Example**: `/iter:workflow docs/stabilize-services.md`

[→ Full documentation](skills/workflow/README.md)

### /iter:test - Test-Driven Iteration

Run Go tests with automated fix iteration until tests pass.

**Purpose**: Execute tests, analyze failures, implement fixes, and retry up to 10 times with intelligent problem-solving.

**When to use**:
- Running flaky or failing tests
- Debugging test failures
- Test-driven development
- Automated test fixing

**Key features**:
- Git worktree isolation - changes merged on success (no push)
- NEVER modifies test files - tests are source of truth
- Advises when test configuration appears incorrect
- Root cause identification and targeted fix implementation
- Max 10 iterations per test run
- Complete documentation of attempts

**Example**: `/iter:test tests/docker/plugin_test.go TestPluginInstallation`

[→ Full documentation](skills/test/README.md)

### /iter:index - Code Index Management

Manage the code index used for semantic and keyword search.

**Purpose**: Build, monitor, and maintain a searchable index of your codebase for fast code navigation.

**When to use**:
- Before using `/iter:search`
- After significant code changes
- To check index freshness
- Setting up search functionality

**Commands**:
- `status` - Show index state and statistics
- `build` - Create or rebuild the index
- `clear` - Remove index data
- `watch` - Auto-update index on file changes

**Example**: `/iter:index build`

[→ Full documentation](skills/index/README.md)

### /iter:search - Code Search

Search your indexed codebase using semantic or keyword search.

**Purpose**: Find relevant code locations with context using natural language queries or specific identifiers.

**When to use**:
- Exploring unfamiliar code
- Finding all instances of a concept
- Understanding system architecture
- Locating functions/classes/patterns

**Key features**:
- Semantic search (understands meaning)
- Keyword search (exact/fuzzy matching)
- Ranked results with context
- File paths and line numbers
- Requires index built via `/iter:index`

**Example**: `/iter:search "user authentication logic"`

[→ Full documentation](skills/search/README.md)

### /iter:install - Shortcut Installation

Create a wrapper that allows using `/iter` instead of `/iter:run`.

**Purpose**: One-time setup to install a convenient shortcut for the main iterative implementation skill.

**When to use**:
- After installing the iter plugin
- When you want shorter command syntax
- First-time setup

**What it does**:
- Creates wrapper skill in `~/.claude/skills/iter/`
- Delegates to `/iter:run` automatically
- Works on Linux, macOS, WSL, and Windows

**Post-installation**: Use `/iter "task"` instead of `/iter:run "task"`

[→ Full documentation](skills/install/README.md)

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

## Git Worktree Isolation

All code-modifying skills (`/iter:run`, `/iter:workflow`, `/iter:test`) use git worktree isolation:

- **Isolated workspace**: Changes are made in a separate worktree branch
- **Safe iteration**: Original branch is not affected during execution
- **Automatic merge**: On completion, changes are merged back to the original branch
- **No auto-push**: Changes are NOT pushed to remote (user must push manually)

This ensures safe experimentation without affecting the main codebase until work is complete.

**Worktree naming**: `iter/{timestamp}-{slug}` (e.g., `iter/20260127-0834-add-health-check`)

**Workdir naming**: `{timestamp}-{slug}` (e.g., `20260127-0834-add-health-check`)

Use `--no-worktree` flag to disable isolation if needed.

## Session Artifacts

Created in `.iter/workdir/{timestamp}-{slug}/`:

| File | Purpose |
|------|---------|
| `requirements.md` | Extracted requirements (R1, R2...) |
| `step_N.md` | Step specifications |
| `step_N_impl.md` | Implementation notes |
| `summary.md` | Completion summary |

## Test-Driven Iteration

The `/iter:test` command runs tests with automated fix iteration (max 10 attempts):

```bash
# Run specific test with auto-fix
/iter:test tests/docker/plugin_test.go TestPluginInstallation

# Run multiple tests
/iter:test tests/docker/iter_command_test.go TestIterRunCommandLine TestIterRunInteractive

# Run all tests in file
/iter:test tests/docker/plugin_test.go
```

### How It Works

1. **Runs the test** and captures full output
2. **If test fails**:
   - Analyzes error messages and stack traces
   - Reads test source to understand requirements
   - Identifies root cause in **implementation code**
   - Implements targeted fixes (NEVER modifies test files)
   - Advises if test configuration appears incorrect
   - Retries test (max 10 iterations)
3. **Documents everything**:
   - Each iteration's analysis and changes
   - Test outputs and results
   - Test configuration advisories
   - Final summary with all fixes

### Output

Results saved to `tests/results/{timestamp}-{test-name}/`:
- `iteration-N-output.log` - Test output for each run
- `iteration-N-analysis.md` - Failure analysis and fix plan
- `iteration-N-changes.md` - Changes made
- `test-results.md` - Final summary

Session state in `.iter/workdir/{timestamp}-test-{slug}/`

### Example

```
/iter:test tests/docker/plugin_test.go TestPluginInstallation

→ Iteration 1: FAIL
  Error: Docker image build failed
  Fix: Add missing dependency

→ Iteration 2: FAIL
  Error: API key not found
  Fix: Update loadAPIKey helper

→ Iteration 3: PASS
  All tests passed
```

## Testing

### Run Unit Tests

```bash
go test ./cmd/iter/... -v
```

**Unit Tests:**
- SKILL.md files have required `name` and `description` fields
- marketplace.json has correct structure with `skills` array
- plugin.json has required fields and no conflicting `skills` field
- Binary commands function correctly
- All expected skills exist

### Run Docker Integration Tests

Docker tests verify `/iter:run` works in Claude. **Requires API key.**

```bash
# Setup: Copy .env.example and add your API key
cp tests/docker/.env.example tests/docker/.env
# Edit tests/docker/.env and set ANTHROPIC_API_KEY=sk-ant-...

# Run Docker integration tests
go test ./tests/docker/... -v
```

**Docker Tests:**
1. `TestDockerPluginInstallation` - Full installation and `/iter:run` test
2. `TestIterRunCommandLine` - Tests `claude -p '/iter:run -v'`
3. `TestIterRunInteractive` - Tests `/iter:run -v` in interactive session

### Test Results

Test results are saved to `tests/results/{timestamp}-docker/`:
- `test-output.log` - Full test output
- `result.txt` - Pass/fail status and summary

### Run Docker Test Directly

```bash
# Build image
docker build -t iter-plugin-test -f tests/docker/Dockerfile .

# Run with API key
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
│   ├── run/SKILL.md             # Core iterative implementation
│   ├── workflow/SKILL.md        # Custom workflow execution
│   ├── test/SKILL.md            # Test-driven iteration
│   ├── index/SKILL.md           # Code index management
│   ├── search/SKILL.md          # Code search
│   └── install/SKILL.md         # Shortcut installation
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
