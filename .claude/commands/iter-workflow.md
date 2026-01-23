---
description: Start workflow-based implementation with custom workflow spec
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`"${ITER_BIN:-$HOME/.local/bin/iter}" workflow "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')"`
