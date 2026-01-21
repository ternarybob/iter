# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Build Commands

```bash
./scripts/build.sh                # Build plugin from source
go build -o bin/iter ./cmd/iter   # Build binary only
go test ./cmd/iter/...            # Run tests
golangci-lint run                 # Lint
```

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
