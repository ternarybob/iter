# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Build Commands

```bash
./scripts/build.sh                # Build plugin from source
go build -o bin/iter ./cmd/iter   # Build binary only
golangci-lint run                 # Lint
```

## Testing

### Unit Tests

```bash
go test ./cmd/iter/... -v         # Run unit tests
```

Unit tests verify binary functionality without Docker or API access.

### Docker Integration Tests

```bash
# Requires ANTHROPIC_API_KEY in environment or tests/docker/.env
ANTHROPIC_API_KEY=sk-... go test ./tests/docker/... -v -timeout 15m

# Run specific test
go test ./tests/docker/... -run TestPluginInstallation -v
```

Docker tests are **independent** and can run in any order or individually:

- **TestPluginInstallation** - Full plugin installation and /iter:run test
- **TestIterRunCommandLine** - Tests `claude -p '/iter:run -v'`
- **TestIterRunInteractive** - Tests `/iter:run -v` in interactive session
- **TestPluginSkillAutoprompt** - Tests skill discoverability
- **TestIterDirectoryCreation** - Tests .iter directory creation
- **TestIterDirectoryRecreation** - Tests .iter directory recreation

Each test sets up its own Docker environment and builds the image if needed (reuses if exists).

**Results** for TestPluginInstallation are saved to `tests/results/{timestamp}-plugin-installation/`:
- `test-output.log` - Full test output
- `result.txt` - Pass/fail status with any missing checks

## Architecture Overview

Iter is a **Claude Code plugin** that implements adversarial iterative implementation.

```
.claude-plugin/plugin.json  →  Plugin manifest
commands/iter.md            →  /iter command (minimal stub)
commands/iter-workflow.md   →  /iter-workflow command (minimal stub)
hooks/hooks.json            →  Stop hook for session control
cmd/iter/main.go            →  Binary with embedded prompts and state management
bin/                        →  Build output (self-contained plugin)
.iter/                      →  Session state (created at runtime)
```

**Key design**: All prompts and logic are embedded in the Go binary. Command markdown files are minimal stubs that invoke the binary.

### Workflow

```
ARCHITECT  →  Analyze requirements, create step_N.md documents
    ↓
WORKER     →  Implement step exactly as specified
    ↓
VALIDATOR  →  Review with adversarial stance (DEFAULT: REJECT)
    ↓
(reject)   →  Back to WORKER with rejection reasons
(pass)     →  Next step or COMPLETE
```

## Execution Rules

When Iter is running:

- **CORRECTNESS over SPEED** - never rush
- **Requirements are LAW** - no interpretation or deviation
- **EXISTING PATTERNS ARE LAW** - match codebase style exactly
- **CLEANUP IS MANDATORY** - remove dead/redundant code
- **BUILD MUST PASS** - verify after each change
- **Validator DEFAULT: REJECT** - find problems, don't confirm success

## Skills

```bash
/iter "<task>"                          # Start iterative implementation
/iter-workflow "<spec>"                 # Start workflow-based implementation
/iter-test <test-file> [tests...]      # Run tests with auto-fix (max 3 iterations)
/iter-index                            # Manage code index
/iter-search "<query>"                 # Search indexed code
```

### Test-Driven Iteration

Use `/iter-test` to run Go tests with automated fix iteration:

```bash
# Run specific test
/iter-test tests/docker/plugin_test.go TestPluginInstallation

# Run multiple tests
/iter-test tests/docker/iter_command_test.go TestIterRunCommandLine TestIterRunInteractive

# Run all tests in file
/iter-test tests/docker/plugin_test.go
```

**Workflow:**
1. Run test and capture output
2. If fail: Analyze error → Fix issue → Retry (max 3x)
3. Document all iterations
4. Save results to `tests/results/{timestamp}-{test-name}/`

**Session state:** `.iter/workdir/test-{slug}-{timestamp}/`

## Binary Commands

```bash
iter run "<task>"           # Start iterative implementation
iter workflow "<spec>"      # Start workflow-based implementation
iter status                 # Show session status
iter step [N]               # Show step instructions
iter pass                   # Record validation pass
iter reject "<reason>"      # Record validation rejection
iter next                   # Move to next step
iter complete               # Mark session complete
iter reset                  # Reset session
iter hook-stop              # Stop hook handler (JSON)
```
