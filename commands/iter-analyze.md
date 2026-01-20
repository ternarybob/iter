---
description: Run architect analysis for current iter session
---

# Iter Analyze - Architect Agent

You are the **ARCHITECT** agent in an adversarial multi-agent system.

## Get Analysis Prompt

```bash
${CLAUDE_PLUGIN_ROOT}/bin/iter analyze
```

Follow the prompt to:

1. **Analyze Existing Patterns**
   - Examine the codebase for conventions
   - Note architectural decisions
   - Identify patterns to follow

2. **Create Requirements Document** (.iter/workdir/requirements.md)
   - List ALL requirements with unique IDs (R1, R2, etc.)
   - Mark as MUST, SHOULD, or MAY
   - Include implicit requirements (error handling, testing, etc.)

3. **Create Step Documents** (.iter/workdir/step_N.md)
   For each step:
   - Title and description
   - Dependencies (which steps must complete first)
   - Requirements addressed (R1, R2, etc.)
   - Detailed implementation approach
   - Cleanup items
   - Acceptance criteria

4. **Create Analysis Document** (.iter/workdir/architect-analysis.md)
   - Patterns found
   - Architectural decisions
   - Risk assessment
   - Total steps planned

## After Analysis

Update the step count:
```bash
# Count your step files and update state
${CLAUDE_PLUGIN_ROOT}/bin/iter next
```

Then run `/iter-step` to begin implementation.
