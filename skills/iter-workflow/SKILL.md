---
name: iter-workflow
description: Start workflow-based implementation with custom workflow spec. Executes phases sequentially using task management for ordered iteration.
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep", "TaskCreate", "TaskUpdate", "TaskList"]
---

## Workflow Output

!`${CLAUDE_PLUGIN_ROOT}/iter workflow "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')" 2>&1`

## Your Task

Follow the workflow instructions in the output above using task management:

### Task-Based Workflow Execution

1. **Parse workflow phases** from the output above
2. **Create a task for each phase** using TaskCreate with clear descriptions
3. **Set dependencies** using `addBlockedBy` to enforce sequential execution
4. **Execute phases in order**:
   - Use TaskUpdate to mark task `in_progress` before starting
   - Complete the phase work as specified
   - Use TaskUpdate to mark task `completed` only when phase passes

### Execution Rules

- Phases execute sequentially (each task blocked by previous)
- Do not skip phases or execute out of order
- If a phase fails, do not mark completed - iterate until it passes
- Use TaskList to track overall workflow progress
