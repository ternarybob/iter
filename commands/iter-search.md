---
name: iter-search
description: Search indexed code (semantic/keyword search)
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`${CLAUDE_PLUGIN_ROOT}/iter search "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')"`
