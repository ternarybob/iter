#!/bin/bash
# run-tests.sh - Test runner for iter-service
#
# Runs tests in completely isolated Docker containers.
# No directories are shared between host and container.
# Results are captured from container stdout/stderr.
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
# Examples:
#   ./tests/run-tests.sh                    # Run all tests
#   ./tests/run-tests.sh --all              # Run all tests
#   ./tests/run-tests.sh --api              # Run API tests only
#   ./tests/run-tests.sh TestAPISearch      # Run specific test

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

# Create results directory
mkdir -p "$RESULTS_DIR"

# Timestamp for this test run
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
RUN_DIR="$RESULTS_DIR/$TIMESTAMP-$TEST_SUITE"
mkdir -p "$RUN_DIR"

echo "========================================"
echo "iter-service Test Runner (Docker)"
echo "========================================"
echo "Timestamp: $TIMESTAMP"
echo "Suite: $TEST_SUITE"
echo "Results: $RUN_DIR"
echo "========================================"

cd "$PROJECT_DIR"

# Build fresh Docker image
echo "Building fresh Docker image..."
docker compose -f tests/docker/docker-compose.yml build --no-cache test 2>&1 | tee "$RUN_DIR/build.log"
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

# Run tests in isolated Docker container (no volume mounts)
# Capture all output from the container
set +e
docker compose -f tests/docker/docker-compose.yml run --rm \
    test \
    sh -c "$TEST_CMD" 2>&1 | tee "$RUN_DIR/test-output.log"
TEST_EXIT_CODE=${PIPESTATUS[0]}
set -e

# Parse results from captured output
TOTAL_TESTS=$(grep -c "^--- " "$RUN_DIR/test-output.log" 2>/dev/null) || TOTAL_TESTS=0
PASSED_TESTS=$(grep -c "^--- PASS" "$RUN_DIR/test-output.log" 2>/dev/null) || PASSED_TESTS=0
FAILED_TESTS=$(grep -c "^--- FAIL" "$RUN_DIR/test-output.log" 2>/dev/null) || FAILED_TESTS=0

# Extract individual test results
echo "Extracting test results..."
grep "^=== RUN\|^--- PASS\|^--- FAIL\|^PASS\|^FAIL\|^ok\|^FAIL" "$RUN_DIR/test-output.log" > "$RUN_DIR/test-summary.txt" 2>/dev/null || true

# Generate JSON summary
cat > "$RUN_DIR/summary.json" <<EOF
{
    "timestamp": "$TIMESTAMP",
    "suite": "$TEST_SUITE",
    "test_pattern": "$TEST_PATTERN",
    "total_tests": $TOTAL_TESTS,
    "passed": $PASSED_TESTS,
    "failed": $FAILED_TESTS,
    "exit_code": $TEST_EXIT_CODE,
    "isolated": true,
    "docker": true
}
EOF

echo ""
echo "========================================"
echo "Test Run Complete"
echo "========================================"
echo "Total: $TOTAL_TESTS"
echo "Passed: $PASSED_TESTS"
echo "Failed: $FAILED_TESTS"
echo "Exit Code: $TEST_EXIT_CODE"
echo "Results: $RUN_DIR"
echo "========================================"

# Show failed tests if any
if [ "$FAILED_TESTS" -gt 0 ]; then
    echo ""
    echo "Failed tests:"
    grep "^--- FAIL" "$RUN_DIR/test-output.log" || true
fi

# Cleanup containers
docker compose -f tests/docker/docker-compose.yml down --remove-orphans 2>/dev/null || true

exit $TEST_EXIT_CODE
