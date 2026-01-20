---
description: Mark the iter session as complete
---

# Iter Complete

Mark the current iter session as complete:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/iter complete
```

This will:
1. Set the session as completed
2. Generate .iter/workdir/summary.md with:
   - Task description
   - Total iterations
   - Steps completed
   - All verdicts
3. Allow the session to exit normally

Only run this after ALL steps have passed validation.
