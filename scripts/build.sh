#!/bin/bash
# Build iter plugin

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Generate version
MAJOR_MINOR="2.1"
DATETIME_STAMP=$(date +"%Y%m%d-%H%M")
VERSION="${MAJOR_MINOR}.${DATETIME_STAMP}"

echo -n "$VERSION" > "$PROJECT_DIR/.version"
echo "Building iter v${VERSION}..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Install Go 1.22+ from https://go.dev/dl/"
    exit 1
fi

# Update version in manifests
sed -i "s/\"version\": \"[^\"]*\"/\"version\": \"${VERSION}\"/" "$PROJECT_DIR/config/plugin.json"
sed -i "s/\"version\": \"[^\"]*\"/\"version\": \"${VERSION}\"/" "$PROJECT_DIR/config/marketplace.json"

# Create output structure
# Plugin at bin/ root, marketplace manifest in bin/.claude-plugin/
rm -rf "$PROJECT_DIR/bin"
mkdir -p "$PROJECT_DIR/bin/.claude-plugin"
mkdir -p "$PROJECT_DIR/bin/skills"
mkdir -p "$PROJECT_DIR/bin/hooks"

# Build binary
echo "Compiling..."
cd "$PROJECT_DIR"
go mod download
go build -ldflags "-X 'main.version=${VERSION}'" -o "$PROJECT_DIR/bin/iter" "$PROJECT_DIR/cmd/iter"
chmod +x "$PROJECT_DIR/bin/iter"

# Copy marketplace manifest to .claude-plugin (where Claude Code expects it)
cp "$PROJECT_DIR/config/marketplace.json" "$PROJECT_DIR/bin/.claude-plugin/marketplace.json"

# Copy plugin manifest to .claude-plugin (where Claude Code expects it)
cp "$PROJECT_DIR/config/plugin.json" "$PROJECT_DIR/bin/.claude-plugin/plugin.json"

# Copy skills (each skill in its own directory with SKILL.md)
for skill_dir in "$PROJECT_DIR/skills/"*/; do
    skill_name=$(basename "$skill_dir")
    mkdir -p "$PROJECT_DIR/bin/skills/$skill_name"
    cp "$skill_dir/SKILL.md" "$PROJECT_DIR/bin/skills/$skill_name/"
done

# Copy hooks
cp "$PROJECT_DIR/hooks/hooks.json" "$PROJECT_DIR/bin/hooks/"

# Verify
"$PROJECT_DIR/bin/iter" help > /dev/null 2>&1 || {
    echo "Error: binary verification failed"
    exit 1
}

echo ""
echo "Build complete: $PROJECT_DIR/bin/"
echo "Version: ${VERSION}"
echo ""
find "$PROJECT_DIR/bin" -type f | sed "s|$PROJECT_DIR/bin/||" | sort
