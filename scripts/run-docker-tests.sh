#!/bin/bash
# Run iter-service tests in Docker containers
# Usage: ./scripts/run-docker-tests.sh [test-type] [test-pattern]
#   test-type: api, mcp, all (default: all)
#   test-pattern: specific test pattern (optional)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DOCKER_DIR="$PROJECT_DIR/tests/docker"
RESULTS_DIR="$PROJECT_DIR/tests/results"

TEST_TYPE="${1:-all}"
TEST_PATTERN="${2:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() { echo -e "${BLUE}[TEST]${NC} $1"; }
success() { echo -e "${GREEN}[PASS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[FAIL]${NC} $1"; }

cleanup() {
    log "Cleaning up containers..."
    docker compose -f "$DOCKER_DIR/docker-compose.yml" down --remove-orphans 2>/dev/null || true
}

trap cleanup EXIT

# Ensure results directory exists
mkdir -p "$RESULTS_DIR"

# Check for Claude credentials
CLAUDE_CREDS=""
if [ -f "$HOME/.claude/.credentials.json" ]; then
    CLAUDE_CREDS="$HOME/.claude/.credentials.json"
    log "Found Claude credentials at $CLAUDE_CREDS"
elif [ -n "$ANTHROPIC_API_KEY" ]; then
    log "Using ANTHROPIC_API_KEY environment variable"
else
    warn "No Claude credentials found - MCP tests may be skipped"
fi

# Build containers
log "Building Docker containers..."
docker compose -f "$DOCKER_DIR/docker-compose.yml" build

# Start containers
log "Starting containers..."
docker compose -f "$DOCKER_DIR/docker-compose.yml" up -d

# Wait for iter service to be healthy
log "Waiting for iter-service to be healthy..."
for i in {1..30}; do
    if docker exec iter-server curl -sf http://localhost:19000/health > /dev/null 2>&1; then
        success "iter-service is healthy"
        break
    fi
    if [ $i -eq 30 ]; then
        error "iter-service failed to start"
        docker logs iter-server
        exit 1
    fi
    sleep 1
done

# Copy Claude credentials into container (if available)
if [ -n "$CLAUDE_CREDS" ]; then
    log "Copying Claude credentials to container..."
    docker exec claude-runner mkdir -p /home/testuser/.claude
    docker cp "$CLAUDE_CREDS" claude-runner:/home/testuser/.claude/.credentials.json
    docker exec claude-runner chown testuser:testuser /home/testuser/.claude/.credentials.json
fi

# Create test results directory in container
docker exec claude-runner mkdir -p /home/testuser/results

# Configure MCP server in Claude container
log "Configuring MCP server in Claude..."
docker exec -u testuser claude-runner bash -c '
    export PATH="/home/testuser/.local/bin:$PATH"
    claude mcp remove iter 2>/dev/null || true
    claude mcp add --transport http iter http://iter:19000/mcp/v1
    claude mcp list
'

# Run tests based on type
run_tests() {
    local test_path="$1"
    local test_name="$2"

    log "Running $test_name tests..."

    # Run tests in container
    local test_cmd="cd /app && go test -v -timeout 300s"
    if [ -n "$TEST_PATTERN" ]; then
        test_cmd="$test_cmd -run $TEST_PATTERN"
    fi
    test_cmd="$test_cmd $test_path 2>&1"

    # Execute and capture output
    local output_file="/home/testuser/results/${test_name}-output.txt"
    if docker exec -u testuser -e HOME=/home/testuser -e ITER_BASE_URL=http://iter:19000 \
        -e CHROME_BIN=/usr/bin/chromium -e CHROMEDP_NO_SANDBOX=true \
        claude-runner bash -c "$test_cmd | tee $output_file"; then
        success "$test_name tests passed"
        return 0
    else
        error "$test_name tests failed"
        return 1
    fi
}

# Track results
FAILED=0

case "$TEST_TYPE" in
    api)
        run_tests "./tests/api/..." "api" || FAILED=1
        ;;
    mcp)
        run_tests "./tests/mcp/..." "mcp" || FAILED=1
        ;;
    all)
        run_tests "./tests/api/..." "api" || FAILED=1
        run_tests "./tests/mcp/..." "mcp" || FAILED=1
        ;;
    *)
        error "Unknown test type: $TEST_TYPE"
        echo "Usage: $0 [api|mcp|all] [test-pattern]"
        exit 1
        ;;
esac

# Copy results from container
log "Copying results from container..."
TIMESTAMP=$(date +%Y-%m-%d_%H-%M-%S)
LOCAL_RESULTS="$RESULTS_DIR/docker-$TIMESTAMP"
mkdir -p "$LOCAL_RESULTS"

docker cp claude-runner:/home/testuser/results/. "$LOCAL_RESULTS/" 2>/dev/null || true

# Also copy any test artifacts from /app/tests/results if they exist
docker exec claude-runner bash -c 'ls /app/tests/results 2>/dev/null' && \
    docker cp claude-runner:/app/tests/results/. "$LOCAL_RESULTS/" 2>/dev/null || true

# List results
if [ -d "$LOCAL_RESULTS" ] && [ "$(ls -A "$LOCAL_RESULTS" 2>/dev/null)" ]; then
    log "Results saved to: $LOCAL_RESULTS"
    ls -la "$LOCAL_RESULTS"
else
    warn "No result files were generated"
fi

# Summary
echo ""
echo "================================"
if [ $FAILED -eq 0 ]; then
    success "All tests passed!"
    exit 0
else
    error "Some tests failed"
    exit 1
fi
