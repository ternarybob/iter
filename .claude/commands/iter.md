---
description: Adversarial iterative implementation. Usage: /iter run <task> | /iter -v | /iter status
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`"${ITER_BIN:-$HOME/.local/bin/iter}" $(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')`
