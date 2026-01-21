#!/bin/bash
# Build iter plugin as a local marketplace for persistent installation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
MARKETPLACE_NAME="iter-local"

echo "Building iter plugin (marketplace format)..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Install Go 1.22+ from https://go.dev/dl/"
    exit 1
fi

# Create marketplace structure:
#   bin/
#   ├── .claude-plugin/marketplace.json
#   └── plugins/iter/
#       ├── .claude-plugin/plugin.json
#       ├── commands/
#       ├── hooks/
#       └── iter (binary)
rm -rf "$PROJECT_DIR/bin"
mkdir -p "$PROJECT_DIR/bin/.claude-plugin"
mkdir -p "$PROJECT_DIR/bin/plugins/iter/.claude-plugin"
mkdir -p "$PROJECT_DIR/bin/plugins/iter/commands"
mkdir -p "$PROJECT_DIR/bin/plugins/iter/hooks"

# Build binary
echo "Compiling binary..."
cd "$PROJECT_DIR"
go mod download
go build -o "$PROJECT_DIR/bin/plugins/iter/iter" "$PROJECT_DIR/cmd/iter"
chmod +x "$PROJECT_DIR/bin/plugins/iter/iter"

# Create marketplace manifest
cat > "$PROJECT_DIR/bin/.claude-plugin/marketplace.json" << 'EOF'
{
  "$schema": "https://anthropic.com/claude-code/marketplace.schema.json",
  "name": "iter-local",
  "description": "Local marketplace for Iter - adversarial iterative implementation plugin",
  "owner": {
    "name": "ternarybob"
  },
  "plugins": [
    {
      "name": "iter",
      "description": "Adversarial iterative implementation - structured loop until requirements/tests pass",
      "version": "2.0.0",
      "author": {
        "name": "ternarybob"
      },
      "source": "./plugins/iter",
      "category": "development",
      "homepage": "https://github.com/ternarybob/iter"
    }
  ]
}
EOF

# Copy plugin manifest
cp "$PROJECT_DIR/.claude-plugin/plugin.json" "$PROJECT_DIR/bin/plugins/iter/.claude-plugin/plugin.json"

# Copy command stubs
cp "$PROJECT_DIR/commands/iter.md" "$PROJECT_DIR/bin/plugins/iter/commands/"
cp "$PROJECT_DIR/commands/iter-workflow.md" "$PROJECT_DIR/bin/plugins/iter/commands/"

# Copy hooks (adjust path: binary is at plugin root)
sed 's|\${CLAUDE_PLUGIN_ROOT}/bin/iter|\${CLAUDE_PLUGIN_ROOT}/iter|g' \
    "$PROJECT_DIR/hooks/hooks.json" > "$PROJECT_DIR/bin/plugins/iter/hooks/hooks.json"

# Verify binary
echo "Verifying build..."
"$PROJECT_DIR/bin/plugins/iter/iter" help > /dev/null 2>&1 || {
    echo "Error: binary verification failed"
    exit 1
}

# Validate marketplace
echo "Validating marketplace..."
if command -v claude &> /dev/null; then
    claude plugin validate "$PROJECT_DIR/bin" 2>/dev/null || true
fi

echo ""
echo "Build complete: $PROJECT_DIR/bin/"
echo ""
echo "Marketplace structure:"
find "$PROJECT_DIR/bin" -type f | sed "s|$PROJECT_DIR/||" | sort
echo ""
echo "Installation:"
echo "  1. Add marketplace:  claude plugin marketplace add $PROJECT_DIR/bin"
echo "  2. Install plugin:   claude plugin install iter@$MARKETPLACE_NAME"
echo ""
echo "Management:"
echo "  - Update:    claude plugin update iter@$MARKETPLACE_NAME"
echo "  - Uninstall: claude plugin uninstall iter@$MARKETPLACE_NAME"
echo "  - Disable:   claude plugin disable iter@$MARKETPLACE_NAME"
echo ""
echo "For development (no install needed):"
echo "  claude --plugin-dir $PROJECT_DIR/bin/plugins/iter"
