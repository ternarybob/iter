---
name: iter
description: Adversarial iterative implementation. Use -v for version, -t:<file> for test mode, -w:<file> for workflow mode, -r to reindex, or just provide a task description.
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep", "TaskCreate", "TaskUpdate", "TaskList"]
---

## Iter Output

!`${CLAUDE_PLUGIN_ROOT}/iter "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')" 2>&1`

## Your Task

Analyze the output above to determine which mode iter is running in.

### Mode Detection

**VERSION MODE** - If output shows "iter version X.X":
- Report the version and stop

**RUN MODE** - If output shows "# ITERATIVE IMPLEMENTATION":
- Follow the iterative workflow using task management
- Create tasks for each step with dependencies
- Execute: ARCHITECT -> WORKER -> VALIDATOR phases
- Mark tasks in_progress before starting, completed on success

**TEST MODE** - If output shows "# TEST-DRIVEN ITERATION":
- Run the specified tests and iterate to fix failures (max 10 iterations)
- NEVER modify test files - tests are the source of truth
- Only fix implementation code
- Call `iter complete` when tests pass

**WORKFLOW MODE** - If output shows "# WORKFLOW EXECUTION":
- Parse the workflow specification
- Create iteration tasks with sequential dependencies
- Execute each phase, validating success criteria
- Call `iter complete` when workflow is done

**REINDEX MODE** - If output shows "Building code index":
- Report the indexing result

---

## Syntax Reference

```
/iter "<task>"                    # Default: iterative implementation
/iter -t:<file> <description>     # Test mode with file
/iter -w:<file> <description>     # Workflow from markdown file
/iter -r                          # Rebuild code index
/iter -v                          # Show version
```

### Examples

```
/iter "add health check endpoint"
/iter -t:tests/docker/plugin_test.go check installation
/iter -w:workflow.md include docker logs in results
/iter -r
```

---

## RUN Mode Instructions

When in RUN mode, follow the iterative workflow:

### Task-Based Iteration

1. **Create tasks** for each step using TaskCreate
2. **Set dependencies** using `addBlockedBy` to ensure ordered execution
3. **Update status** to `in_progress` before starting work
4. **Mark completed** only when step passes validation

### Workflow Phases

- **ARCHITECT**: Analyze requirements, create step documents (step_N.md)
- **WORKER**: Implement current step exactly as specified
- **VALIDATOR**: Review with adversarial stance (DEFAULT: REJECT)

### Rules

- Tasks must complete in order (use blockedBy dependencies)
- Failed validation returns to WORKER (do not mark completed)
- Use TaskList to check progress and find next task

---

## TEST Mode Instructions

When in TEST mode:

### Critical Rules

1. **NEVER modify test files** - tests are the source of truth
2. **MAX 10 ITERATIONS** - stop after 10 test-fix cycles
3. **FIX ROOT CAUSE** - don't just patch symptoms
4. **VERIFY BUILDS** - run `go build ./...` after changes

### Workflow

1. Run the test command shown in output
2. Analyze failures
3. Fix IMPLEMENTATION code only
4. Re-run tests
5. Iterate until pass or max iterations

### Test Configuration Advisory

If test appears misconfigured (wrong expected values, missing fixtures), output an advisory but DO NOT modify the test.

### Completion

When tests pass: `iter complete`

---

## WORKFLOW Mode Instructions

When in WORKFLOW mode:

### Setup Phase

1. Parse workflow specification from output
2. Create workflow directory structure
3. Document architecture in workdir
4. Create iteration tasks with sequential dependencies

### Execution Phase

For each iteration:
1. **GET STATUS**: Collect system status, logs, errors
2. **REVIEW & PLAN**: Analyze and select ONE priority issue
3. **IMPLEMENT**: Execute fixes, verify changes
4. **SUMMARIZE**: Document results, decide continue/stop

### Rules

- **ONE ISSUE PER ITERATION**: Fix highest priority only
- **DOCUMENT THINKING**: Explain reasoning in notes.md
- **FOLLOW PRIORITIES**: Use workflow priority rules
- **VERIFY CHANGES**: Always verify before marking complete

### Completion

When workflow criteria met: `iter complete`

---

## Session Management

These commands work in any mode:

```bash
iter status      # Show session status
iter pass        # Record validation pass
iter reject "reason"  # Record rejection
iter next        # Move to next step
iter complete    # Finalize session (merges worktree)
iter reset       # Abort session
```
