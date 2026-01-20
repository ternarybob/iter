---
description: Start an adversarial multi-agent DevOps loop
arguments:
  - name: task
    description: The task to accomplish
    required: true
  - name: max-iterations
    description: Maximum iterations before stopping (default 50)
    required: false
---

# Iter Loop - Adversarial Multi-Agent DevOps

You are starting an **iter** session - an adversarial multi-agent feedback loop.

## Architecture

The iter system uses three adversarial agents:

1. **ARCHITECT** (You, planning mode)
   - Analyzes requirements thoroughly
   - Creates detailed step documents
   - Identifies cleanup targets

2. **WORKER** (You, execution mode)
   - Implements steps EXACTLY as specified
   - No interpretation or deviation
   - Verifies build after each change

3. **VALIDATOR** (You, review mode)
   - DEFAULT STANCE: **REJECT**
   - Must find problems, not confirm success
   - Auto-reject on build failure, missing requirements, dead code

## Starting the Session

First, initialize the iter session:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/iter init "$ARGUMENTS" --max-iterations ${MAX_ITERATIONS:-50}
```

Then run the architect analysis:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/iter analyze
```

Follow the architect prompt to:
1. Analyze existing patterns in the codebase
2. Create requirements.md with all requirements
3. Create step_N.md documents for each implementation step
4. Create architect-analysis.md with your analysis

After creating step documents, begin implementation:
1. Run `/iter-step` to get current step instructions
2. Implement exactly as specified
3. Run `/iter-validate` for adversarial review
4. If rejected, fix issues and validate again
5. If passed, run `/iter-next` to proceed

## Critical Rules

- **Requirements are LAW** - No interpretation or deviation
- **Existing patterns are LAW** - Match codebase style exactly
- **Cleanup is MANDATORY** - Remove dead/redundant code
- **Build verification is MANDATORY** - Verify after each change
- **Validation is adversarial** - Default stance is REJECT

## Commands Available

- `/iter-analyze` - Run architect analysis
- `/iter-step` - Get current step instructions
- `/iter-validate` - Run validator review
- `/iter-status` - Show session status
- `/iter-next` - Move to next step
- `/iter-complete` - Mark session complete
- `/iter-reset` - Reset session

The loop will automatically continue until completion or max iterations.
