# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
./scripts/install.sh              # Build and install plugin from source
go build -o bin/iter ./cmd/iter   # Build CLI binary only
go test ./cmd/iter/...            # Run tests
golangci-lint run                 # Lint
```

## Architecture Overview

Iter is a **Claude Code plugin** that implements an adversarial multi-agent DevOps loop.

```
.claude-plugin/plugin.json  →  Plugin manifest
commands/*.md               →  Slash commands (/iter-loop, /iter-validate, etc.)
hooks/hooks.json            →  Stop hook that blocks exit during active session
cmd/iter/main.go            →  CLI binary for state management
bin/iter                    →  Compiled binary (build output)
.iter/                      →  Session state (created at runtime)
```

The plugin works by:
- Slash commands invoke the `bin/iter` CLI to manage state and output prompts
- Stop hook calls `iter hook-stop` to block exit until session completes
- State persists in `.iter/state.json`, artifacts in `.iter/workdir/`

### Multi-Agent Workflow

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

These rules apply when Iter is running (via `/iter-loop`):

- **CORRECTNESS over SPEED** - never rush validation
- **Requirements are LAW** - no interpretation or deviation
- **EXISTING PATTERNS ARE LAW** - match codebase style exactly
- **CLEANUP IS MANDATORY** - remove dead/redundant code
- **BUILD VERIFICATION IS MANDATORY** - verify after each change
- **Validator DEFAULT STANCE: REJECT** - find problems, don't confirm success

## Adding Components

### New Plugin Command
1. Create `commands/<name>.md` with YAML frontmatter
2. Reference `bin/iter` CLI for state management

### New CLI Subcommand
1. Add case in `cmd/iter/main.go` switch statement
2. Implement `cmd<Name>(args []string) error` function
