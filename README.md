# Iter

**Iter** is a Claude Code plugin for adversarial iterative implementation. It provides a structured loop that iterates until requirements and tests pass, with semantic code indexing for intelligent code discovery.

## Features

- **Self-contained binary**: All prompts and logic embedded in Go binary
- **Adversarial validation**: Default stance is REJECT - find problems
- **Structured iteration**: Architect → Worker → Validator loop
- **Semantic code index**: DAG-based dependency tracking and search
- **Auto-activation**: `/iter` shortcut and `CLAUDE.md` auto-installed on session start
- **Exit blocking**: Session continues until complete or max iterations

## Installation

### Prerequisites

- Go 1.23+
- Claude Code CLI

### Environment Variables (Optional)

```bash
# For LLM-powered commit summaries (optional)
export GEMINI_API_KEY=your-api-key-here
```

The `GEMINI_API_KEY` enables LLM-generated commit summaries in the `history` command and MCP server. Without it, commit messages are used as summaries instead.

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
| `/iter -t:<file> [tests...]` | Test-driven iteration with auto-fix (max 10x) |
| `/iter -w:<file> <description>` | Start workflow from markdown file |
| `/iter -r` | Rebuild the code index |
| `/iter -v` | Show version |

### Code Discovery Commands

| Command | Description |
|---------|-------------|
| `/iter search "<query>"` | Semantic code search |
| `/iter deps "<symbol>"` | Show what symbol depends on and what depends on it |
| `/iter impact "<file>"` | Show what's affected by file changes |
| `/iter history [N]` | Show commit history with summaries (default: 10) |

### Session Management

| Command | Description |
|---------|-------------|
| `iter status` | Show current session status |
| `iter pass` | Record validation pass |
| `iter reject "<reason>"` | Record validation rejection |
| `iter next` | Move to next step |
| `iter complete` | Finalize session (merges worktree) |
| `iter reset` | Abort session |

### Index Management

| Command | Description |
|---------|-------------|
| `iter index` | Show index status |
| `iter index build` | Build/rebuild the full code index |
| `iter index clear` | Clear and rebuild the index |
| `iter index daemon` | Start background daemon (auto-detaches) |
| `iter index daemon status` | Check if daemon is running |
| `iter index daemon stop` | Stop the daemon gracefully |

## MCP Server

Iter includes an MCP (Model Context Protocol) server that exposes the code index to Claude Code as native tools.

### Starting the MCP Server

The MCP server runs on stdio and is automatically configured via `.mcp.json`:

```bash
# Manual start (for testing)
iter mcp-server
```

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `search` | Semantic code search with filters |
| `deps` | Get dependencies of a symbol |
| `dependents` | Get dependents of a symbol |
| `impact` | Change impact analysis for a file |
| `history` | Commit history with summaries |
| `stats` | Index statistics |
| `reindex` | Trigger full reindex |

### MCP Configuration

The `.mcp.json` file in the project root registers the MCP server:

```json
{
  "mcpServers": {
    "iter-index": {
      "command": "${CLAUDE_PLUGIN_ROOT}/iter",
      "args": ["mcp-server"],
      "env": {
        "GEMINI_API_KEY": "${GEMINI_API_KEY}"
      }
    }
  }
}
```

### Verifying MCP Tools

In Claude Code, use `/mcp` to verify the iter-index tools are available.

## Auto-Activation

When iter is installed, the following happens automatically on session start:

1. **Index daemon starts** in background for semantic code search
2. **CLAUDE.md generated** at repo root with iter default process
3. **/iter shortcut installed** in `~/.claude/skills/iter/`

No manual setup required - just install the plugin and use `/iter`.

## Index-First Code Discovery

**ALWAYS use the semantic index before grep or file search:**

```bash
iter search "<query>"     # Semantic code search (understands code structure)
iter deps "<symbol>"      # Show dependencies and dependents
iter impact "<file>"      # Show change impact analysis
iter history [N]          # Show commit history with summaries
```

The semantic index provides more accurate results than grep by understanding:

- **Function call graphs** - who calls whom
- **Import/export relationships** - module dependencies
- **Type implementations** - interface implementers
- **Change propagation paths** - impact analysis
- **Commit lineage** - LLM-generated summaries of changes

### DAG (Dependency Graph)

The code index builds a directed acyclic graph tracking:

- **Calls**: Function/method call relationships
- **Imports**: Package/module imports
- **Types**: Type definitions and implementations
- **References**: Symbol usage across files

### Lineage Tracking

Commit history is analyzed and summarized:

- Recent commits with LLM-generated summaries
- Change context for understanding code evolution
- Impact of historical changes on current code

## Modes

### Run Mode (Default)

Structured implementation with ARCHITECT → WORKER → VALIDATOR loop.

```bash
/iter "add health check endpoint"
```

### Test Mode

Test-driven iteration with auto-fix (max 10 iterations).

```bash
/iter -t:tests/docker/plugin_test.go TestPluginInstallation
```

### Workflow Mode

Custom workflow specifications for specialized iterative processes.

```bash
/iter -w:workflow.md include docker logs in results
```

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

All code-modifying modes (`/iter`, `/iter -w:`, `/iter -t:`) use git worktree isolation:

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

The `/iter -t:` command runs tests with automated fix iteration (max 10 attempts):

```bash
# Run specific test with auto-fix
/iter -t:tests/docker/plugin_test.go TestPluginInstallation

# Run multiple tests
/iter -t:tests/docker/iter_command_test.go TestIterRunCommandLine TestIterRunInteractive

# Run all tests in file
/iter -t:tests/docker/plugin_test.go
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
/iter -t:tests/docker/plugin_test.go TestPluginInstallation

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

Docker tests verify `/iter` works in Claude. **Requires API key.**

```bash
# Setup: Copy .env.example and add your API key
cp tests/docker/.env.example tests/docker/.env
# Edit tests/docker/.env and set ANTHROPIC_API_KEY=sk-ant-...

# Run Docker integration tests
go test ./tests/docker/... -v
```

**Docker Tests:**
1. `TestDockerPluginInstallation` - Full installation and `/iter` test
2. `TestIterRunCommandLine` - Tests `claude -p '/iter -v'`
3. `TestIterRunInteractive` - Tests `/iter -v` in interactive session

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
├── .mcp.json                    # MCP server registration
├── config/
│   ├── plugin.json              # Plugin manifest template
│   └── marketplace.json         # Marketplace manifest template
├── skills/
│   └── iter/SKILL.md            # Unified iter skill (all modes)
├── index/                       # Code indexing package
│   ├── index.go                 # Core indexer
│   ├── search.go                # Search functionality
│   ├── dag.go                   # Dependency graph
│   ├── dag_parser.go            # AST parsing for DAG
│   ├── lineage.go               # Commit history tracking
│   ├── llm.go                   # LLM integration for summaries
│   ├── mcp_server.go            # MCP server implementation
│   ├── watcher.go               # File change watcher
│   ├── parser.go                # Symbol extraction
│   └── types.go                 # Type definitions
├── prompts/                     # Embedded prompt templates
│   ├── system.md                # System rules
│   ├── architect.md             # Architect role
│   ├── worker.md                # Worker role
│   └── validator.md             # Validator role
├── templates/
│   └── CLAUDE.md.tmpl           # CLAUDE.md template
├── hooks/hooks.json             # Stop/SessionStart/PreToolUse hooks
├── embed.go                     # Go embed for prompts/templates
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
