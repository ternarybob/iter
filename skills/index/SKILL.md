---
name: index
description: Manage the code index for semantic search. Commands: status, build, clear, watch. Use before /iter:search for code navigation.
allowed-tools: ["Bash", "Read"]
---

## Index Output

!`${CLAUDE_PLUGIN_ROOT}/iter index $(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g') 2>&1`

## Your Task

Report the index operation result from the output above:

- **status**: Show current index state and statistics
- **build**: Report build progress and completion
- **clear**: Confirm index was cleared
- **watch**: Report watch mode status

Suggest next steps if appropriate (e.g., "run `/iter:search <query>` to search the index").
