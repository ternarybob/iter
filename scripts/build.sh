#!/bin/bash
# Build iter-service

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Generate version
MAJOR_MINOR="2.1"
DATETIME_STAMP=$(date +"%Y%m%d-%H%M")
VERSION="${MAJOR_MINOR}.${DATETIME_STAMP}"

echo -n "$VERSION" > "$PROJECT_DIR/.version"
echo "Building iter-service v${VERSION}..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Install Go 1.23+ from https://go.dev/dl/"
    exit 1
fi

# Build binary
echo "Compiling..."
cd "$PROJECT_DIR"
go mod download
go build -ldflags "-X 'main.version=${VERSION}'" -o "$PROJECT_DIR/iter-service" "$PROJECT_DIR/cmd/iter-service"
chmod +x "$PROJECT_DIR/iter-service"

# Verify
"$PROJECT_DIR/iter-service" version > /dev/null 2>&1 || {
    echo "Error: binary verification failed"
    exit 1
}

echo ""
echo "Build complete: $PROJECT_DIR/iter-service"
echo "Version: ${VERSION}"
echo ""
echo "To install: sudo cp iter-service /usr/local/bin/"
