# iter-test-runner

Run iter-service tests in Docker, analyze failures, fix code, and iterate until tests pass.

## Usage

```
/iter-test-runner [test-name-or-pattern]
```

## Examples

```
/iter-test-runner TestServiceStartStop
/iter-test-runner TestAPI
/iter-test-runner --all
```

## Description

This skill runs iter-service integration tests in isolated Docker containers and automatically fixes code issues to make tests pass.

**CRITICAL RULES:**
1. **ALWAYS use Docker** - Tests must run in isolated Docker containers
2. **NEVER modify test files** - Tests are the source of truth
3. **Fix only implementation code** - Modify files in `cmd/`, `internal/`, `pkg/`, `web/`
4. **STOP conditions:**
   - Test structure is invalid (syntax errors, missing imports)
   - Test requirement is impossible (e.g., expects magic behavior)
   - Maximum 5 iterations reached without progress
5. **Each iteration must save a summary** to `./tests/results/{type}/{datetime}-{testname}/`

## Workflow

### Step 1: Validate Test Structure

Before running, verify the test file:
1. Check test file exists and compiles: `go build ./tests/...`
2. Check test function signature is valid: `func Test*(t *testing.T)`
3. If invalid, STOP and report: "Test structure invalid: {reason}"

### Step 2: Run Test in Docker

```bash
cd /home/bobmc/development/iter

# Run all tests
./tests/run-tests.sh --all

# Run specific test pattern
./tests/run-tests.sh {TestPattern}

# Run specific suite
./tests/run-tests.sh --api
./tests/run-tests.sh --service
./tests/run-tests.sh --ui
```

The test runner will:
1. Build a fresh Docker image (--no-cache)
2. Run tests in isolated container
3. Collect results in `./tests/results/`

### Step 3: Analyze Results

Parse test output for:
- PASS: Test passed, create success summary, DONE
- FAIL: Extract failure reason, proceed to fix
- ERROR: Build/compile error, analyze error

### Step 4: Create Iteration Summary

Create summary file: `./tests/results/{datetime}-{testname}/iteration-{N}.json`

```json
{
  "iteration": N,
  "test_name": "TestXxx",
  "status": "pass|fail|error",
  "error_message": "...",
  "fix_applied": "description of fix",
  "files_modified": ["path/to/file.go"],
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### Step 5: Apply Fix

1. Read error message and identify root cause
2. Locate the source file causing the issue
3. Apply minimal fix to make test pass
4. **DO NOT modify test files** (tests/*.go)
5. Run tests again in Docker

### Step 6: Iterate or Stop

- If test passes: Create final success summary, DONE
- If 5 iterations reached: Create failure summary with all attempts, STOP
- If same error repeats 3 times: STOP with "Unable to fix: {reason}"
- Otherwise: Go to Step 2

## Output

Final output includes:
1. Test result (PASS/FAIL/STOP)
2. Number of iterations
3. Summary of fixes applied
4. Path to results directory

## Test Structure Requirements

Valid test files must:
1. Be in `tests/service/`, `tests/api/`, or `tests/ui/` directories
2. Import `"github.com/ternarybob/iter/tests/common"`
3. Use `common.NewTestEnv(t, "type", "test-name")` for setup
4. Call `env.Cleanup()` in defer
5. Call `env.WriteSummary()` with results

## Example Valid Test

```go
func TestExample(t *testing.T) {
    env := common.NewTestEnv(t, "api", "example")
    defer env.Cleanup()

    if err := env.Start(); err != nil {
        t.Fatalf("Failed to start: %v", err)
    }

    // Test logic here

    env.WriteSummary(true, time.Since(start), "Test passed")
}
```

## Stop Conditions

STOP immediately and report if:
1. Test file has syntax errors
2. Test function signature is wrong
3. Test expects behavior that contradicts architecture
4. Same fix fails 3 times
5. 5 iterations without progress

## Docker Requirements

The test runner always uses Docker:
- Fresh container built each run (--no-cache)
- Tests run sequentially (-p 1) to avoid port conflicts
- Results mounted to host at `./tests/results/`
- Isolated from host iter-service processes
