#!/bin/bash
# Install iter plugin from source
#
# This script builds the iter binary and sets up the plugin
# in the local project directory.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Installing iter plugin..."
echo "Project directory: $PROJECT_DIR"

# Check Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go 1.22+ first."
    echo "Download from: https://go.dev/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 22 ]); then
    echo "Error: Go 1.22+ is required. Found: go$GO_VERSION"
    exit 1
fi

echo "Found Go version: $GO_VERSION"

# Create bin directory
mkdir -p "$PROJECT_DIR/bin"

# Download dependencies
echo "Downloading dependencies..."
cd "$PROJECT_DIR"
go mod download

# Build the iter binary
echo "Building iter binary..."
go build -o "$PROJECT_DIR/bin/iter" "$PROJECT_DIR/cmd/iter"

# Verify the binary was created
if [ ! -f "$PROJECT_DIR/bin/iter" ]; then
    echo "Error: Failed to build iter binary"
    exit 1
fi

# Make binary executable
chmod +x "$PROJECT_DIR/bin/iter"

# Verify the binary works
echo "Verifying installation..."
"$PROJECT_DIR/bin/iter" help > /dev/null 2>&1 || {
    echo "Error: iter binary failed to run"
    exit 1
}

echo ""
echo "Installation complete!"
echo ""
echo "Binary installed to: $PROJECT_DIR/bin/iter"
echo ""
echo "To use the iter plugin with Claude Code:"
echo "  claude --plugin-dir $PROJECT_DIR"
echo ""
echo "Or add to your claude config:"
echo "  {\"pluginDirs\": [\"$PROJECT_DIR\"]}"
