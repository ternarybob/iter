---
description: Reset the iter session
---

# Iter Reset

Reset the current iter session and clear all state:

```bash
${CLAUDE_PLUGIN_ROOT}/bin/iter reset
```

**WARNING**: This will delete:
- All session state
- All step documents
- All validation verdicts
- All artifacts

Use this to start fresh or abandon a session.
