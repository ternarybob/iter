---
description: Search indexed code (semantic/keyword search)
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`"${ITER_BIN:-$HOME/.local/bin/iter}" search "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')"`
