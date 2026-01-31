#!/bin/bash
# run-tests.sh - Test runner for iter-service
#
# Usage:
#   ./tests/run-tests.sh [options] [test-pattern]
#
# Options:
#   --docker      Run tests in Docker container (isolated)
#   --service     Run service tests only
#   --api         Run API tests only
#   --ui          Run UI tests only
#   --all         Run all tests (default)
#   --verbose     Verbose output
#   --help        Show this help
#
# Examples:
#   ./tests/run-tests.sh                    # Run all tests locally
#   ./tests/run-tests.sh --docker           # Run all tests in Docker
#   ./tests/run-tests.sh --api              # Run API tests only
#   ./tests/run-tests.sh TestAPISearch      # Run specific test

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$PROJECT_DIR/tests/results"

# Default options
USE_DOCKER=false
TEST_SUITE="all"
VERBOSE=""
TEST_PATTERN=""
TIMEOUT="300s"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --docker)
            USE_DOCKER=true
            shift
            ;;
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
        --help|-h)
            head -30 "$0" | tail -25
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
RUN_DIR="$RESULTS_DIR/$TIMESTAMP-run"
mkdir -p "$RUN_DIR"

echo "========================================"
echo "iter-service Test Runner"
echo "========================================"
echo "Timestamp: $TIMESTAMP"
echo "Suite: $TEST_SUITE"
echo "Docker: $USE_DOCKER"
echo "Results: $RUN_DIR"
echo "========================================"

# Build binary first (for local testing)
if [ "$USE_DOCKER" = false ]; then
    echo "Building iter-service..."
    cd "$PROJECT_DIR"
    go build -o iter-service ./cmd/iter-service
    echo "Build complete: $(./iter-service version)"
    echo ""
fi

# Determine test path
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

# Add test pattern filter
RUN_ARGS=""
if [ -n "$TEST_PATTERN" ]; then
    RUN_ARGS="-run $TEST_PATTERN"
fi

# Run tests
if [ "$USE_DOCKER" = true ]; then
    echo "Running tests in Docker..."
    cd "$PROJECT_DIR"

    docker-compose -f tests/docker/docker-compose.yml build test

    case $TEST_SUITE in
        service)
            docker-compose -f tests/docker/docker-compose.yml run --rm test-service
            ;;
        api)
            docker-compose -f tests/docker/docker-compose.yml run --rm test-api
            ;;
        ui)
            docker-compose -f tests/docker/docker-compose.yml run --rm test-ui
            ;;
        all)
            docker-compose -f tests/docker/docker-compose.yml run --rm test
            ;;
    esac
else
    echo "Running tests locally..."
    cd "$PROJECT_DIR"

    # Run tests with output capture
    set +e
    go test $VERBOSE -timeout "$TIMEOUT" $RUN_ARGS "$TEST_PATH" 2>&1 | tee "$RUN_DIR/test-output.log"
    TEST_EXIT_CODE=${PIPESTATUS[0]}
    set -e

    # Parse results
    TOTAL_TESTS=$(grep -c "^--- " "$RUN_DIR/test-output.log" 2>/dev/null || echo "0")
    PASSED_TESTS=$(grep -c "^--- PASS" "$RUN_DIR/test-output.log" 2>/dev/null || echo "0")
    FAILED_TESTS=$(grep -c "^--- FAIL" "$RUN_DIR/test-output.log" 2>/dev/null || echo "0")

    # Generate summary
    cat > "$RUN_DIR/summary.json" <<EOF
{
    "timestamp": "$TIMESTAMP",
    "suite": "$TEST_SUITE",
    "docker": $USE_DOCKER,
    "test_pattern": "$TEST_PATTERN",
    "total_tests": $TOTAL_TESTS,
    "passed": $PASSED_TESTS,
    "failed": $FAILED_TESTS,
    "exit_code": $TEST_EXIT_CODE,
    "duration_seconds": $(date +%s)
}
EOF

    echo ""
    echo "========================================"
    echo "Test Run Complete"
    echo "========================================"
    echo "Total: $TOTAL_TESTS"
    echo "Passed: $PASSED_TESTS"
    echo "Failed: $FAILED_TESTS"
    echo "Results: $RUN_DIR"
    echo "========================================"

    # Collect individual test results
    echo "Collecting test results..."
    find "$RESULTS_DIR" -maxdepth 2 -name "summary.json" -newer "$RUN_DIR/test-output.log" -exec cat {} \; > "$RUN_DIR/test-summaries.json" 2>/dev/null || true

    exit $TEST_EXIT_CODE
fi
