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
2. Run tests in completely isolated container (no volume mounts)
3. Capture all output from container stdout/stderr
4. Parse results and save to `./tests/results/{timestamp}-{suite}/`

### Step 3: Analyze Results

Check the output files:
- `test-output.log` - Full container output
- `test-summary.txt` - Extracted test results
- `summary.json` - JSON summary with pass/fail counts

Parse for:
- PASS: Test passed, DONE
- FAIL: Extract failure reason, proceed to fix
- ERROR: Build/compile error, analyze error

### Step 4: Create Iteration Summary

After each fix attempt, document in the results directory:

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

1. Read error message from test-output.log
2. Identify root cause from the error
3. Locate the source file causing the issue
4. Apply minimal fix to make test pass
5. **DO NOT modify test files** (tests/*.go)
6. Run tests again in Docker

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

## Docker Isolation

The test runner provides complete isolation:
- Fresh container built each run (--no-cache)
- **No volume mounts** - container is completely isolated from host
- Tests run sequentially (-p 1) to avoid port conflicts
- Results captured from container stdout/stderr
- Container is removed after tests complete
