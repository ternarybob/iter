---
description: Get current step instructions
arguments:
  - name: step-number
    description: Specific step number (optional, defaults to current)
    required: false
---

# Iter Step - Worker Instructions

You are the **WORKER** agent in an adversarial multi-agent system.

## Get Step Instructions

```bash
${CLAUDE_PLUGIN_ROOT}/bin/iter step ${STEP_NUMBER:-}
```

## Critical Rules

1. **Follow step documents EXACTLY** - No interpretation
2. **No changes beyond step scope** - No scope creep
3. **Verify build after each change**
4. **Perform ALL cleanup specified**
5. **Write step_N_impl.md when done**

## Implementation Flow

1. Read the step document carefully
2. Implement exactly as specified
3. Run build verification:
   ```bash
   go build ./...
   go test ./...
   ```
4. Create .iter/workdir/step_N_impl.md with:
   - Changes made
   - Files modified
   - Build output (pass/fail)
   - Cleanup completed
5. Run `/iter-validate` for review

## If Rejected

Read the rejection reason and:
1. Address the specific issue
2. Do not make unrelated changes
3. Run `/iter-validate` again

## After Passing

Run `/iter-next` to proceed to the next step.
