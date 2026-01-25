---
name: iter
description: Start iterative implementation until requirements/tests pass. Use -v to show version. Manages workflow phases (ARCHITECT, WORKER, VALIDATOR) with task tracking for ordered execution.
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep", "TaskCreate", "TaskUpdate", "TaskList"]
---

## Iter Output

!`if [ "$ARGUMENTS" = "-v" ] || [ "$ARGUMENTS" = "--version" ]; then ${CLAUDE_PLUGIN_ROOT}/iter version 2>&1; else ${CLAUDE_PLUGIN_ROOT}/iter run "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')" 2>&1; fi`

## Your Task

If the output above shows version information, report the iter version.

Otherwise, follow the iter workflow using task management:

### Task-Based Iteration

1. **Create tasks** for each step in the workflow using TaskCreate
2. **Set dependencies** using `addBlockedBy` to ensure tasks execute in order
3. **Update task status** to `in_progress` before starting work
4. **Mark completed** only when the step passes validation

### Workflow Phases

Execute the phase indicated in the output above:

- **ARCHITECT**: Analyze requirements, create step documents. Create a task for each step with dependencies.
- **WORKER**: Implement the current step exactly as specified. Mark task in_progress, then completed on success.
- **VALIDATOR**: Review with adversarial stance (DEFAULT: REJECT). Only mark task completed if validation passes.

### Iteration Rules

- Tasks must complete in order (use blockedBy dependencies)
- Failed validation returns to WORKER (do not mark completed)
- Only proceed to next step when current task is completed
- Use TaskList to check progress and find next available task
