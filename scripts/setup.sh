#!/bin/bash
# Setup iter commands for current project
# Run this after installing the iter plugin to get /iter instead of /iter:iter

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ITER_BIN="$PLUGIN_DIR/bin/iter"

# Check binary exists
if [ ! -f "$ITER_BIN" ]; then
    echo "Error: iter binary not found at $ITER_BIN"
    echo "Run ./scripts/build.sh first"
    exit 1
fi

# Create symlink in ~/.local/bin
mkdir -p ~/.local/bin
ln -sf "$ITER_BIN" ~/.local/bin/iter
echo "✓ Symlinked iter to ~/.local/bin/iter"

# Ensure ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    echo ""
    echo "Add ~/.local/bin to your PATH by adding this to ~/.bashrc or ~/.zshrc:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
    echo ""
fi

# Create project commands (optional - for current directory)
if [ "$1" = "--project" ]; then
    mkdir -p .claude/commands

    cat > .claude/commands/iter.md << 'EOF'
---
description: Adversarial iterative implementation. Usage: /iter run <task> | /iter -v | /iter status
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`"${ITER_BIN:-$HOME/.local/bin/iter}" $(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')`
EOF

    cat > .claude/commands/iter-workflow.md << 'EOF'
---
description: Start workflow-based implementation with custom workflow spec
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`"${ITER_BIN:-$HOME/.local/bin/iter}" workflow "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')`
EOF

    cat > .claude/commands/iter-index.md << 'EOF'
---
description: Manage the code index (status, build, clear, watch)
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`"${ITER_BIN:-$HOME/.local/bin/iter}" index $(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')`
EOF

    cat > .claude/commands/iter-search.md << 'EOF'
---
description: Search indexed code (semantic/keyword search)
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---
!`"${ITER_BIN:-$HOME/.local/bin/iter}" search "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')"`
EOF

    echo "✓ Created project commands in .claude/commands/"
    echo "  /iter, /iter-workflow, /iter-index, /iter-search"
fi

echo ""
echo "Setup complete. Restart Claude Code to use /iter commands."
