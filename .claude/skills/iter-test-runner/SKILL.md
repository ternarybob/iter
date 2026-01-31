# iter-test-runner

Run iter-service tests in isolated Docker containers, analyze failures, fix code, and iterate until tests pass.

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

This skill runs iter-service integration tests in completely isolated Docker containers. No directories are shared between the host and container - results are captured from container stdout/stderr.

**CRITICAL RULES:**
1. **ALWAYS use Docker** - Tests run in isolated containers with no shared directories
2. **NEVER modify test files** - Tests are the source of truth
3. **Fix only implementation code** - Modify files in `cmd/`, `internal/`, `pkg/`, `web/`
4. **STOP conditions:**
   - Test structure is invalid (syntax errors, missing imports)
   - Test requirement is impossible (e.g., expects magic behavior)
   - Maximum 5 iterations reached without progress
5. **Results captured from stdout** - Container output is captured and parsed

## Workflow

### Step 1: Validate Test Structure

Before running, verify the test file compiles:
```bash
go build ./tests/...
```
If invalid, STOP and report: "Test structure invalid: {reason}"

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
2. Run tests in completely isolated container (no volume mounts)
3. Capture all output from container stdout/stderr
4. Parse results and save to `./tests/results/{timestamp}-{suite}/`

### Step 3: Analyze Results

Results are saved to `./tests/results/{timestamp}-{suite}/` with these files:
- `build.log` - Docker image build output
- `test-output.log` - Full container stdout/stderr
- `test-summary.txt` - Extracted test pass/fail lines
- `summary.json` - JSON summary:
  ```json
  {
      "timestamp": "2026-01-31_14-51-33",
      "suite": "all",
      "test_pattern": "",
      "total_tests": 18,
      "passed": 18,
      "failed": 0,
      "exit_code": 0,
      "isolated": true,
      "docker": true
  }
  ```

Parse test-output.log for:
- `--- PASS`: Test passed
- `--- FAIL`: Test failed - extract error message and proceed to fix
- Build errors: Analyze compilation error

### Step 4: Apply Fix (if tests failed)

1. Read error message from test-output.log
2. Identify root cause from the error
3. Locate the source file causing the issue
4. Apply minimal fix to make test pass
5. **DO NOT modify test files** (tests/*.go)
6. Run tests again in Docker

### Step 5: Iterate or Stop

- If test passes: Report success, DONE
- If 5 iterations reached: Report failure with all attempts, STOP
- If same error repeats 3 times: STOP with "Unable to fix: {reason}"
- Otherwise: Go to Step 2

## Output

Final output includes:
1. Test result (PASS/FAIL/STOP)
2. Number of iterations
3. Summary of fixes applied
4. Path to results directory

## Test Structure

Tests are located in:
- `tests/service/` - Service lifecycle tests
- `tests/api/` - REST API tests
- `tests/ui/` - Web UI tests
- `tests/common/` - Shared test utilities

Valid test files must:
1. Import `"github.com/ternarybob/iter/tests/common"`
2. Use `common.NewTestEnv(t, "type", "test-name")` for setup
3. Call `defer env.Cleanup()`
4. Call `env.Start()` to start the service

Example:
```go
func TestExample(t *testing.T) {
    env := common.NewTestEnv(t, "api", "example")
    defer env.Cleanup()

    if err := env.Start(); err != nil {
        t.Fatalf("Failed to start: %v", err)
    }

    client := env.NewHTTPClient()
    // Test logic using client.Get(), client.Post(), etc.
}
```

## Stop Conditions

STOP immediately and report if:
1. Test file has syntax errors
2. Test expects behavior that contradicts architecture
3. Same fix fails 3 times
4. 5 iterations without progress

## Docker Isolation

The test runner provides complete isolation:
- Fresh container built each run (--no-cache)
- **No volume mounts** - container is completely isolated from host
- Tests run sequentially (-p 1) to avoid port conflicts
- Results captured from container stdout/stderr
- Container is removed after tests complete
