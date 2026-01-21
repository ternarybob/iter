---
name: iter-index
description: Manage the code index (status, build, clear, watch)
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`${CLAUDE_PLUGIN_ROOT}/iter index $(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')`
