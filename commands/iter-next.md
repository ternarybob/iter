---
description: Move to the next step
---

# Iter Next

Move to the next step after passing validation:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/iter next
```

This will:
1. Increment the current step number
2. Reset the validation pass counter
3. Set phase to "worker"

Then run `/iter-step` to get the next step's instructions.
