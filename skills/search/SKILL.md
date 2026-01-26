---
name: search
description: Search indexed code using semantic or keyword search. Returns relevant code locations with context. Requires index built via /iter:index.
allowed-tools: ["Bash", "Read", "Glob", "Grep"]
---

## Search Results

!`${CLAUDE_PLUGIN_ROOT}/iter search "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')" 2>&1`

## Your Task

Present the search results from the output above:

1. **List relevant matches** with file paths and line numbers
2. **Show context** for each match to help understanding
3. **Suggest navigation** - offer to read specific files for more detail

If no results found, suggest:
- Checking if index is built (`/iter:index status`)
- Trying different search terms
- Using Grep for literal pattern matching
