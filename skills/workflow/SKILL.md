---
name: workflow
description: Start workflow-based implementation with custom workflow spec. Executes phases sequentially using task management for ordered iteration.
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep", "TaskCreate", "TaskUpdate", "TaskList"]
---

## Workflow Initialization

!`
# Check if ARGUMENTS is a file path
if [ -f "$ARGUMENTS" ]; then
  # Read file content
  SPEC=$(cat "$ARGUMENTS")
else
  # Use ARGUMENTS directly as spec content
  SPEC="$ARGUMENTS"
fi

# Pass spec content to iter workflow command
${CLAUDE_PLUGIN_ROOT}/iter workflow "$(printf '%s' "$SPEC" | sed 's/"/\\"/g')" 2>&1
`

## Your Task

The workflow specification has been loaded. You MUST follow the workflow structure documented below using task management.

### Phase 1: ARCHITECT - Workflow Setup and Documentation

**Before starting iterations**, you must:

1. **Parse the workflow specification** from the output above and identify:
   - Working directory requirements
   - Iteration structure (steps, data collection, validation)
   - Configuration parameters (MIN_ITERATIONS, MAX_ITERATIONS, etc.)
   - Success/failure criteria
   - Priority ordering rules

2. **Create the workflow directory structure**:
   ```bash
   REPO_ROOT=$(git rev-parse --show-toplevel)
   WORKDIR="$REPO_ROOT/.claude/workdir/$(date +%Y-%m-%d-%H%M)-<workflow-name>"
   mkdir -p "$WORKDIR"
   ```

3. **Document the architecture** in `$WORKDIR/architecture.md`:
   - Workflow purpose and scope
   - Source code locations
   - Service/system architecture
   - Iteration structure overview
   - Success criteria
   - Priority rules

4. **Create initial workflow state** in `$WORKDIR/workflow-state.md`:
   ```markdown
   # Workflow State

   | Field | Value |
   |-------|-------|
   | Started | [ISO timestamp] |
   | Status | IN_PROGRESS |
   | Current Iteration | 0 |
   | Workdir | [path] |
   ```

5. **Create iteration tasks** using TaskCreate:
   - Create task for each iteration (1 to MAX_ITERATIONS)
   - Set dependencies so iterations execute sequentially
   - Example: Task "Iteration 2" should have `addBlockedBy: ["task-id-of-iteration-1"]`

### Phase 2: WORKER - Execute Iterations

For each iteration task (mark `in_progress` before starting):

#### STEP 1: GET STATUS

Create iteration directory structure:
```bash
ITERATION=[current iteration number]
ITER_DIR="$WORKDIR/iteration-$ITERATION"
mkdir -p "$ITER_DIR/status" "$ITER_DIR/logs" "$ITER_DIR/fixes"
```

Collect all status information as specified in the workflow:
- Read previous iteration notes (if ITERATION > 1)
- Collect system status (docker, services, endpoints, etc.)
- Gather logs and error information
- Run verification/test scripts
- Detect any failure indicators

Save all collected data to `$ITER_DIR/status/` and `$ITER_DIR/logs/`.

#### STEP 2: REVIEW & PLAN

Create `$ITER_DIR/notes.md` with:
- **Current State Analysis**: Review all status files
- **Thinking/Reasoning**: Document your analysis and decision process
- **Selected Issue**: Choose ONE priority issue to fix (use workflow priority rules)
- **Action Plan**: List ordered actions to fix the issue

**CRITICAL**: Follow the workflow's priority ordering rules. Select the highest priority issue.

#### STEP 3: IMPLEMENT

Execute the action plan:
1. For each action, create `$ITER_DIR/fixes/action-N.md` documenting:
   - Before state
   - Command/change executed
   - Result
   - Verification

2. Apply fixes using Edit/Write/Bash tools

3. If configuration changes were made, redeploy/restart as specified in workflow

4. Wait for system stabilization (as specified in workflow config)

#### STEP 4: SUMMARIZE

Create `$ITER_DIR/summary.md` with:
- Issue addressed
- Actions taken
- Files modified
- Verification results
- Outcome (fix successful, stability status)
- Decision: continue or stop?

Update `$WORKDIR/workflow-state.md` with iteration results.

### Phase 3: VALIDATOR - Check Iteration Success

Review the iteration summary against workflow success criteria:

**Continue to next iteration IF**:
- Iteration < MIN_ITERATIONS, OR
- Issues remain and Iteration < MAX_ITERATIONS

**Stop with SUCCESS IF**:
- Iteration >= MIN_ITERATIONS, AND
- All success criteria met (as specified in workflow)

**Stop with INCOMPLETE IF**:
- Iteration >= MAX_ITERATIONS, AND
- Issues still remain

Mark current iteration task as `completed` before proceeding to next iteration.

### Phase 4: COMPLETE - Final Summary

When workflow stops, create `$WORKDIR/summary.md`:
- Workdir path and timestamps
- Final status (SUCCESS/INCOMPLETE/FAILED)
- Metrics (iterations completed, issues fixed/remaining)
- Iteration history table
- Final system status
- Remaining issues (if any)
- Lessons learned

### Task Management Rules

- **Create iteration tasks upfront** with sequential dependencies
- **Mark task `in_progress`** before starting iteration
- **Only mark `completed`** when iteration fully succeeds
- **Use TaskList** to track progress and find next iteration
- **Do NOT skip iterations** - execute in order

### Execution Rules

- **AUTONOMOUS EXECUTION**: Execute all commands directly
- **ONE ISSUE PER ITERATION**: Fix highest priority issue only
- **DOCUMENT THINKING**: Always explain reasoning in notes.md
- **READ PREVIOUS NOTES**: Before each iteration, review previous iteration
- **FOLLOW WORKFLOW PRIORITIES**: Use the priority rules from the workflow spec
- **VERIFY CHANGES**: Always verify fixes work before marking iteration complete
