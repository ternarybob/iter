#!/bin/bash
# run-tests.sh - Test runner for iter-service
#
# Runs tests in completely isolated Docker containers.
# Creates per-test results directories.
#
# Usage:
#   ./tests/run-tests.sh [options] [test-pattern]
#
# Options:
#   --service     Run service tests only
#   --api         Run API tests only
#   --ui          Run UI tests only
#   --all         Run all tests (default)
#   --verbose     Verbose output
#   --help        Show this help
#
# Results structure:
#   tests/results/
#   ├── service/{datetime}-{testname}/
#   ├── api/{datetime}-{testname}/
#   └── ui/{datetime}-{testname}/

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$PROJECT_DIR/tests/results"

# Default options
TEST_SUITE="all"
VERBOSE="-v"
TEST_PATTERN=""
TIMEOUT="300s"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --service)
            TEST_SUITE="service"
            shift
            ;;
        --api)
            TEST_SUITE="api"
            shift
            ;;
        --ui)
            TEST_SUITE="ui"
            shift
            ;;
        --all)
            TEST_SUITE="all"
            shift
            ;;
        --verbose|-v)
            VERBOSE="-v"
            shift
            ;;
        --quiet|-q)
            VERBOSE=""
            shift
            ;;
        --help|-h)
            head -25 "$0" | tail -20
            exit 0
            ;;
        *)
            TEST_PATTERN="$1"
            shift
            ;;
    esac
done

# Create results directories for each test type
mkdir -p "$RESULTS_DIR/service"
mkdir -p "$RESULTS_DIR/api"
mkdir -p "$RESULTS_DIR/ui"

# Timestamp for this test run
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")

# Temporary directory for raw output
RAW_OUTPUT_DIR=$(mktemp -d)
trap "rm -rf $RAW_OUTPUT_DIR" EXIT

echo "========================================"
echo "iter-service Test Runner (Docker)"
echo "========================================"
echo "Timestamp: $TIMESTAMP"
echo "Suite: $TEST_SUITE"
echo "Results: $RESULTS_DIR/{service,api,ui}/"
echo "========================================"

cd "$PROJECT_DIR"

# Build fresh Docker image
echo "Building fresh Docker image..."
docker compose -f tests/docker/docker-compose.yml build --no-cache test 2>&1 | tee "$RAW_OUTPUT_DIR/build.log"
echo "Build complete."
echo ""

# Determine test command based on suite and pattern
TEST_PATH=""
case $TEST_SUITE in
    service)
        TEST_PATH="./tests/service/..."
        ;;
    api)
        TEST_PATH="./tests/api/..."
        ;;
    ui)
        TEST_PATH="./tests/ui/..."
        ;;
    all)
        TEST_PATH="./tests/..."
        ;;
esac

# Build test command
TEST_CMD="go test -p 1 $VERBOSE -timeout $TIMEOUT"
if [ -n "$TEST_PATTERN" ]; then
    TEST_CMD="$TEST_CMD -run $TEST_PATTERN"
fi
TEST_CMD="$TEST_CMD $TEST_PATH"

echo "Running: $TEST_CMD"
echo ""

# Run tests in isolated Docker container
set +e
docker compose -f tests/docker/docker-compose.yml run --rm \
    test \
    sh -c "$TEST_CMD" 2>&1 | tee "$RAW_OUTPUT_DIR/test-output.log"
TEST_EXIT_CODE=${PIPESTATUS[0]}
set -e

# Parse results and create per-test directories
echo ""
echo "Creating per-test result directories..."

# Extract test names and their results
while IFS= read -r line; do
    if [[ "$line" =~ ^---\ (PASS|FAIL):\ ([A-Za-z0-9_]+) ]]; then
        STATUS="${BASH_REMATCH[1]}"
        TEST_NAME="${BASH_REMATCH[2]}"

        # Determine test type from name
        if [[ "$TEST_NAME" =~ ^TestService ]]; then
            TEST_TYPE="service"
            SHORT_NAME=$(echo "$TEST_NAME" | sed 's/^TestService//' | tr '[:upper:]' '[:lower:]')
        elif [[ "$TEST_NAME" =~ ^TestAPI ]]; then
            TEST_TYPE="api"
            SHORT_NAME=$(echo "$TEST_NAME" | sed 's/^TestAPI//' | tr '[:upper:]' '[:lower:]')
        elif [[ "$TEST_NAME" =~ ^TestUI ]]; then
            TEST_TYPE="ui"
            SHORT_NAME=$(echo "$TEST_NAME" | sed 's/^TestUI//' | tr '[:upper:]' '[:lower:]')
        else
            TEST_TYPE="other"
            SHORT_NAME=$(echo "$TEST_NAME" | sed 's/^Test//' | tr '[:upper:]' '[:lower:]')
        fi

        # Create test-specific results directory
        TEST_DIR="$RESULTS_DIR/$TEST_TYPE/$TIMESTAMP-$SHORT_NAME"
        mkdir -p "$TEST_DIR"

        # Extract test-specific output
        awk "/^=== RUN   $TEST_NAME\$/,/^--- (PASS|FAIL): $TEST_NAME/" "$RAW_OUTPUT_DIR/test-output.log" > "$TEST_DIR/test-output.log" 2>/dev/null || true

        # Create test summary
        cat > "$TEST_DIR/summary.json" <<EOF
{
    "test_name": "$TEST_NAME",
    "status": "$STATUS",
    "type": "$TEST_TYPE",
    "timestamp": "$TIMESTAMP",
    "docker": true
}
EOF

        # Create markdown summary
        cat > "$TEST_DIR/SUMMARY.md" <<EOF
# $TEST_NAME

**Status:** $STATUS
**Type:** $TEST_TYPE
**Timestamp:** $TIMESTAMP

## Output

\`\`\`
$(cat "$TEST_DIR/test-output.log" 2>/dev/null || echo "No output captured")
\`\`\`
EOF

        echo "  Created: $TEST_TYPE/$TIMESTAMP-$SHORT_NAME/ ($STATUS)"
    fi
done < "$RAW_OUTPUT_DIR/test-output.log"

# Parse overall results
TOTAL_TESTS=$(grep -c "^--- " "$RAW_OUTPUT_DIR/test-output.log" 2>/dev/null) || TOTAL_TESTS=0
PASSED_TESTS=$(grep -c "^--- PASS" "$RAW_OUTPUT_DIR/test-output.log" 2>/dev/null) || PASSED_TESTS=0
FAILED_TESTS=$(grep -c "^--- FAIL" "$RAW_OUTPUT_DIR/test-output.log" 2>/dev/null) || FAILED_TESTS=0

# Create overall run summary in results root
RUN_SUMMARY_DIR="$RESULTS_DIR/_runs"
mkdir -p "$RUN_SUMMARY_DIR"

if [ "$TEST_EXIT_CODE" -eq 0 ]; then
    STATUS="PASS"
else
    STATUS="FAIL"
fi

PASSED_LIST=$(grep "^--- PASS" "$RAW_OUTPUT_DIR/test-output.log" 2>/dev/null | sed 's/--- PASS: /- /' | sed 's/ (.*//' || true)
FAILED_LIST=$(grep "^--- FAIL" "$RAW_OUTPUT_DIR/test-output.log" 2>/dev/null | sed 's/--- FAIL: /- /' | sed 's/ (.*//' || true)

cat > "$RUN_SUMMARY_DIR/$TIMESTAMP-$TEST_SUITE.md" <<EOF
# Test Run: $TIMESTAMP

**Status:** $STATUS
**Suite:** $TEST_SUITE
**Pattern:** ${TEST_PATTERN:-"(none)"}

## Results

| Metric | Value |
|--------|-------|
| Total | $TOTAL_TESTS |
| Passed | $PASSED_TESTS |
| Failed | $FAILED_TESTS |
| Exit Code | $TEST_EXIT_CODE |

## Passed Tests

$PASSED_LIST

EOF

if [ "$FAILED_TESTS" -gt 0 ]; then
    cat >> "$RUN_SUMMARY_DIR/$TIMESTAMP-$TEST_SUITE.md" <<EOF
## Failed Tests

$FAILED_LIST
EOF
fi

# Copy build log to runs directory
cp "$RAW_OUTPUT_DIR/build.log" "$RUN_SUMMARY_DIR/$TIMESTAMP-build.log"

echo ""
echo "========================================"
echo "Test Run Complete"
echo "========================================"
echo "Total: $TOTAL_TESTS"
echo "Passed: $PASSED_TESTS"
echo "Failed: $FAILED_TESTS"
echo "Exit Code: $TEST_EXIT_CODE"
echo ""
echo "Results:"
echo "  Per-test: $RESULTS_DIR/{service,api,ui}/$TIMESTAMP-*/"
echo "  Run summary: $RUN_SUMMARY_DIR/$TIMESTAMP-$TEST_SUITE.md"
echo "========================================"

# Show failed tests if any
if [ "$FAILED_TESTS" -gt 0 ]; then
    echo ""
    echo "Failed tests:"
    grep "^--- FAIL" "$RAW_OUTPUT_DIR/test-output.log" || true
fi

# Cleanup containers
docker compose -f tests/docker/docker-compose.yml down --remove-orphans 2>/dev/null || true

exit $TEST_EXIT_CODE
