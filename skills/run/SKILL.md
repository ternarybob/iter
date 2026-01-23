---
description: Start iterative implementation until requirements/tests pass
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`${CLAUDE_PLUGIN_ROOT}/iter run "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')"`
