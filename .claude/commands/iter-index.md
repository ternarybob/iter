---
description: Manage the code index (status, build, clear, watch)
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`"${ITER_BIN:-$HOME/.local/bin/iter}" index $(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')`
