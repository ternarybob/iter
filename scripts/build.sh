#!/bin/bash
# -----------------------------------------------------------------------
# Build Script for iter-service
# -----------------------------------------------------------------------
#
# Usage:
#   ./scripts/build.sh           # Build only
#   ./scripts/build.sh -run      # Build and run
#   ./scripts/build.sh -deploy   # Build and deploy to bin/
#
# -----------------------------------------------------------------------

set -e

# Parse arguments
RUN=false
DEPLOY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -run|--run)
            RUN=true
            DEPLOY=true
            shift
            ;;
        -deploy|--deploy)
            DEPLOY=true
            shift
            ;;
        -h|--help)
            echo "Usage: ./scripts/build.sh [options]"
            echo ""
            echo "Options:"
            echo "  -deploy    Build and deploy all files to bin/"
            echo "  -run       Build, deploy, and start the service"
            echo "  -h, --help Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_DIR/bin"

# Generate version
MAJOR_MINOR="2.1"
DATETIME_STAMP=$(date +"%Y%m%d-%H%M")
VERSION="${MAJOR_MINOR}.${DATETIME_STAMP}"

# Get git commit
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "========================================"
echo "iter-service Build Script"
echo "========================================"
echo "Version: $VERSION"
echo "Git Commit: $GIT_COMMIT"
echo "Project: $PROJECT_DIR"
echo "========================================"

# Update .version file
echo -n "$VERSION" > "$PROJECT_DIR/.version"

# Check Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Install Go 1.23+ from https://go.dev/dl/"
    exit 1
fi

# Create bin directory
mkdir -p "$BIN_DIR"

# Stop existing service if running
if pgrep -f "iter-service" > /dev/null 2>&1; then
    echo "Stopping existing iter-service..."
    pkill -f "iter-service" 2>/dev/null || true
    sleep 1
fi

# Download dependencies
echo "Downloading dependencies..."
cd "$PROJECT_DIR"
go mod download

# Build binary
echo "Building iter-service..."
go build -ldflags "-X 'main.version=${VERSION}'" -o "$BIN_DIR/iter-service" "$PROJECT_DIR/cmd/iter-service"
chmod +x "$BIN_DIR/iter-service"

# Verify build
if [ ! -f "$BIN_DIR/iter-service" ]; then
    echo "Error: Build failed - binary not found"
    exit 1
fi

echo "Binary built: $BIN_DIR/iter-service"

# Deploy if requested
if [ "$DEPLOY" = true ]; then
    echo ""
    echo "Deploying files to bin/..."

    # Copy config example (only if config doesn't exist)
    if [ ! -f "$BIN_DIR/config.toml" ]; then
        if [ -f "$PROJECT_DIR/configs/config.example.toml" ]; then
            cp "$PROJECT_DIR/configs/config.example.toml" "$BIN_DIR/config.toml"
            echo "  Created: config.toml"
        fi
    else
        echo "  Preserved: config.toml (already exists)"
    fi

    # Create data directory
    mkdir -p "$BIN_DIR/data"
    echo "  Created: data/"

    # Create logs directory
    mkdir -p "$BIN_DIR/logs"
    echo "  Created: logs/"

    echo "Deployment complete."
fi

# Run if requested
if [ "$RUN" = true ]; then
    echo ""
    echo "========================================"
    echo "Starting iter-service"
    echo "========================================"

    cd "$BIN_DIR"

    if [ -f "$BIN_DIR/config.toml" ]; then
        echo "Config: $BIN_DIR/config.toml"
        exec "$BIN_DIR/iter-service" serve --config "$BIN_DIR/config.toml"
    else
        echo "Using default configuration"
        exec "$BIN_DIR/iter-service" serve
    fi
fi

echo ""
echo "========================================"
echo "Build Complete"
echo "========================================"
echo "Binary: $BIN_DIR/iter-service"
echo "Version: $VERSION"
echo ""
echo "To run:"
echo "  cd $BIN_DIR && ./iter-service serve"
echo ""
echo "Or use: ./scripts/build.sh -run"
