---
description: Start workflow-based implementation with custom workflow spec
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`${CLAUDE_PLUGIN_ROOT}/iter workflow "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')"`
