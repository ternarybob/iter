# Commands Reference

Complete reference for all iter plugin commands, their usage, and interactions.

## Overview

The iter plugin provides three categories of commands:

| Category | Commands | Purpose |
|----------|----------|---------|
| **Iteration** | `/iter`, `/iter-workflow` | Start adversarial implementation sessions |
| **Session Control** | `status`, `step`, `pass`, `reject`, `next`, `complete`, `reset` | Manage active sessions |
| **Index & Search** | `/iter-index`, `/iter-search` | Code indexing and semantic search |
| **Utility** | `/iter-version` | Version information |

## Iteration Commands

### /iter

Starts adversarial iterative implementation with automatic phase cycling.

```bash
/iter "<task description>"
/iter "<task>" --max-iterations 100
/iter "<task>" --no-worktree
```

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--max-iterations N` | 50 | Maximum iteration count before forced exit |
| `--no-worktree` | false | Disable git worktree isolation |

**Behavior:**

1. Creates isolated git worktree at `.iter/worktrees/<branch>/` (unless `--no-worktree`)
2. Creates session state at `.iter/state.json`
3. Starts background index daemon for code search
4. Injects ARCHITECT phase prompt with system rules
5. Blocks session exit via stop hook until completion or max iterations

**Workflow phases:**

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

**Session artifacts created:**

| File | Description |
|------|-------------|
| `requirements.md` | Extracted requirements (R1, R2, ...) |
| `step_N.md` | Step specifications from architect |
| `step_N_impl.md` | Implementation notes from worker |
| `architect-analysis.md` | Architecture and patterns found |
| `summary.md` | Final completion summary |

**Index interaction:**
- Auto-starts index daemon on session start
- Use `/iter-search` during any phase for code context
- Index updates in real-time as you edit files

### /iter-workflow

Alternative workflow-based execution with custom specification.

```bash
/iter-workflow "<workflow spec>"
/iter-workflow "<spec>" --max-iterations 100
/iter-workflow "<spec>" --no-worktree
```

**Options:** Same as `/iter`

**Behavior:**

Unlike `/iter` which uses the fixed ARCHITECT → WORKER → VALIDATOR cycle, `/iter-workflow` accepts arbitrary workflow specifications. The workflow spec defines:
- Custom phases and their order
- Success criteria for each phase
- Transition conditions

**Use cases:**
- Custom validation pipelines
- Non-standard iteration patterns
- Specialized review workflows

## Session Control Commands

These commands are used during an active `/iter` or `/iter-workflow` session.

### iter status

Display current session state.

```bash
iter status
```

**Output includes:**
- Current task description
- Active phase (architect/worker/validator)
- Iteration count
- Current step number and total steps
- Recent validation verdicts
- Elapsed time

### iter step

Display step instructions.

```bash
iter step      # Show current step
iter step 3    # Show step 3 specifically
```

**Output:**
- Step document content from `.iter/workdir/step_N.md`
- Worker role guidance
- Requirements for the step

### iter pass

Record validation pass for current step.

```bash
iter pass
```

**Behavior:**
- Records pass verdict in session state
- Advances to next step if available
- If all steps complete, session can be finished with `iter complete`

**Validator guidelines:**
- Default stance is REJECT
- Only pass when ALL requirements are verified
- Must verify build passes, tests pass, and implementation matches spec

### iter reject

Record validation rejection with reason.

```bash
iter reject "Build fails with type error in handler.go"
iter reject "Missing error handling for edge case"
```

**Behavior:**
- Records rejection reason in session state
- Resets phase to WORKER for re-implementation
- Increments rejection count for the step

**Common rejection reasons:**
- Build fails
- Tests fail
- Requirements not fully implemented
- Code style doesn't match existing patterns
- Dead code or redundant logic present

### iter next

Manually advance to next step.

```bash
iter next
```

**Behavior:**
- Advances step counter
- Resets validation state for new step
- Use when manually managing step progression

### iter complete

Complete the session and merge changes.

```bash
iter complete
```

**Behavior:**
1. Auto-commits any pending changes in worktree
2. Merges worktree branch to original branch
3. Generates `summary.md` with completion report
4. Cleans up worktree and temporary files
5. Allows session exit

**Git operations:**
- Creates merge commit with session summary
- Preserves full commit history from worktree
- Returns to original branch

### iter reset

Reset and abandon the session.

```bash
iter reset
```

**Behavior:**
- Clears all session state
- Checks out original branch
- Removes worktree and branch
- Deletes `.iter/` directory contents

**Warning:** This discards all work in the current session.

## Index Commands

See [Code Indexing](code-indexing.md) for detailed documentation.

### /iter-index

Manage the code index.

```bash
/iter-index              # Show status (auto-builds if empty)
/iter-index build        # Force full rebuild
/iter-index clear        # Clear all indexed data
/iter-index watch        # Start watcher manually (blocking)
/iter-index daemon       # Start background daemon
/iter-index daemon status # Check daemon status
/iter-index daemon stop  # Stop daemon
```

**Session interaction:**
- Daemon starts automatically with `/iter` or `/iter-workflow`
- Watcher updates index in real-time during sessions
- Index persists between sessions

### /iter-search

Search indexed code.

```bash
/iter-search "<query>"
/iter-search "handler" --kind=function
/iter-search "Config" --path=internal/
/iter-search "Parse" --limit=5
```

**Options:**

| Option | Description |
|--------|-------------|
| `--kind=<type>` | Filter by: function, method, type, const |
| `--path=<prefix>` | Filter by file path prefix |
| `--branch=<branch>` | Filter by git branch |
| `--limit=<n>` | Max results (default 10) |

**Search strategy:**
1. Semantic search using vector similarity
2. Falls back to keyword search if no semantic matches
3. Applies filters post-query

**Phase-specific usage:**

| Phase | Use Case | Example |
|-------|----------|---------|
| ARCHITECT | Find related patterns | `/iter-search "authentication handler"` |
| WORKER | Get implementation context | `/iter-search "Config" --kind=type` |
| VALIDATOR | Check for duplicates | `/iter-search "NewHandler" --kind=function` |

## Utility Commands

### /iter-version

Display plugin version.

```bash
/iter-version
```

**Output:**
- Current plugin version (e.g., `2.1.20260122-1245`)
- Version is set at build time via `-ldflags`

## Skills Integration

The iter plugin defines skills that integrate with Claude Code's command system.

### Skill Definitions

From `config/plugin.json`:

| Skill | Command | Description |
|-------|---------|-------------|
| `iter:iter` | `/iter` | Start iterative implementation |
| `iter:iter-workflow` | `/iter-workflow` | Start workflow-based implementation |
| `iter:iter-index` | `/iter-index` | Manage code index |
| `iter:iter-search` | `/iter-search` | Search indexed code |
| `iter:iter-version` | `/iter-version` | Show version info |

### Allowed Tools

All iter commands have access to these Claude Code tools:

```json
["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
```

These tools enable:
- File operations during implementation
- Build and test execution
- Code exploration and modification

### Embedded Roles

The binary embeds role-specific prompts injected during phase transitions:

| Role | Purpose |
|------|---------|
| **SystemRules** | Non-negotiable execution rules |
| **ArchitectRole** | Analysis and step planning guidance |
| **WorkerRole** | Exact implementation guidance |
| **ValidatorRole** | Adversarial validation stance |
| **ValidationRules** | Auto-reject and pass conditions |

## Hook System

### Stop Hook

The stop hook prevents accidental session exit during iteration.

**Configuration** (`hooks/hooks.json`):

```json
{
  "hooks": {
    "Stop": [{
      "matcher": "^/iter",
      "hooks": [{
        "type": "command",
        "command": "${CLAUDE_PLUGIN_ROOT}/iter hook-stop"
      }]
    }]
  }
}
```

**Behavior:**
1. Intercepts session exit attempts during `/iter` commands
2. Reads session state from `.iter/state.json`
3. If not complete and below max iterations:
   - Increments iteration counter
   - Determines next phase
   - Injects phase-specific prompt
   - Blocks exit
4. If complete or at max iterations:
   - Allows exit

### Internal Command

```bash
iter hook-stop
```

This is an internal command called by the stop hook. It outputs JSON:

```json
{
  "continue": false,
  "systemMessage": "<next phase prompt>"
}
```

## State Management

### Session State

Stored at `.iter/state.json`:

| Field | Description |
|-------|-------------|
| `task` | Task description |
| `phase` | Current phase (architect/worker/validator) |
| `iteration` | Current iteration count |
| `step` | Current step number |
| `totalSteps` | Total steps defined |
| `verdicts` | Validation history |
| `worktree` | Git worktree info |
| `startedAt` | Session start time |

### Worktree Isolation

For git repositories (unless `--no-worktree`):

1. **Branch created:** `iter/<task-slug>-<timestamp>`
2. **Worktree location:** `.iter/worktrees/<branch>/`
3. **On complete:** Merged to original branch
4. **On reset:** Deleted with branch

## Command Flow Diagram

```
User: /iter "implement feature X"
         ↓
    [cmdRun]
         ↓
    ├── Create worktree branch
    ├── Initialize state.json
    ├── Start index daemon
    └── Return ARCHITECT prompt
         ↓
    [User works with Claude]
         ↓
    [Stop hook triggers]
         ↓
    [hook-stop]
         ↓
    ├── Read state
    ├── Increment iteration
    ├── Determine next phase
    └── Inject phase prompt
         ↓
    [Cycle continues...]
         ↓
    User: iter complete
         ↓
    [cmdComplete]
         ↓
    ├── Commit changes
    ├── Merge to original
    ├── Generate summary
    └── Cleanup worktree
         ↓
    [Session ends]
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Max iterations reached | Session exits, work preserved in worktree |
| Build fails during validation | Auto-reject, return to WORKER |
| Git operations fail | Error message, session preserved |
| Index daemon crashes | Auto-restarts on next search |

## Best Practices

1. **Let automation work:** Don't manually run `iter next` or `iter pass` - the stop hook manages phase transitions
2. **Use search during phases:** `/iter-search` provides context that improves implementation quality
3. **Trust the validator:** Default REJECT stance catches issues early
4. **Review verdicts:** Check `iter status` to understand rejection reasons
5. **Complete, don't reset:** Finish sessions properly to preserve git history
