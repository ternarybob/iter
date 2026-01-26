---
name: test
description: Run tests with automated iteration until pass (max 10 iterations)
arguments: <test-file> [test-names...]
examples:
  - test tests/docker/plugin_test.go TestPluginInstallation
  - test tests/docker/iter_command_test.go TestIterRunCommandLine TestIterRunInteractive
  - test tests/docker/plugin_test.go
---

# Test-Driven Iteration

Executes Go tests with automated iteration to fix failures. Runs up to 10 iterations until tests pass.

## Usage

```bash
/iter:test <test-file> [test-names...]
```

## Arguments

- `<test-file>`: Required. Path to Go test file (e.g., `tests/docker/plugin_test.go`)
- `[test-names...]`: Optional. Specific test names to run (e.g., `TestPluginInstallation`). If omitted, runs all tests in file.

## Examples

```bash
# Run specific test with iteration
/iter:test tests/docker/plugin_test.go TestPluginInstallation

# Run multiple tests
/iter:test tests/docker/iter_command_test.go TestIterRunCommandLine TestIterRunInteractive

# Run all tests in file
/iter:test tests/docker/plugin_test.go
```

## Behavior

1. **Validates** test file exists
2. **Executes** specified tests
3. **Captures** test output and results
4. **Documents** failures and creates fix plan
5. **Iterates** up to 10 times to fix implementation (NOT the test)
6. **Saves** results to `tests/results/{timestamp}-{test-name}/`
7. **Advises** when test appears misconfigured (does NOT auto-fix tests)

## Output Documentation

Each iteration documents:
- Task list with current objectives
- Test execution results
- Changes made to fix failures
- Test configuration advisory (if applicable)

Results saved to test's results directory structure.

---

## Arguments Parsing

!`
# Parse arguments
TEST_FILE=""
TEST_NAMES=()

for arg in $ARGUMENTS; do
    if [ -z "$TEST_FILE" ]; then
        TEST_FILE="$arg"
    else
        TEST_NAMES+=("$arg")
    fi
done

# Validate test file
if [ -z "$TEST_FILE" ]; then
    echo "ERROR: Test file required"
    echo ""
    echo "Usage: /iter:test <test-file> [test-names...]"
    exit 1
fi

if [ ! -f "$TEST_FILE" ]; then
    echo "ERROR: Test file not found: $TEST_FILE"
    exit 1
fi

# Get test package
TEST_DIR=$(dirname "$TEST_FILE")
TEST_PACKAGE="./$TEST_DIR"

# Create session directory
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
if [ ${#TEST_NAMES[@]} -gt 0 ]; then
    TEST_SLUG=$(echo "${TEST_NAMES[0]}" | sed 's/^Test//' | tr '[:upper:]' '[:lower:]')
else
    TEST_SLUG=$(basename "$TEST_DIR")
fi

RESULTS_DIR="tests/results/$TIMESTAMP-$TEST_SLUG"
mkdir -p "$RESULTS_DIR"

SESSION_ID="test-$TEST_SLUG-$TIMESTAMP"
SESSION_DIR=".iter/workdir/$SESSION_ID"
mkdir -p "$SESSION_DIR"

# Save session info
cat > "$SESSION_DIR/session-info.txt" <<EOF
TEST_FILE=$TEST_FILE
TEST_PACKAGE=$TEST_PACKAGE
TEST_NAMES=${TEST_NAMES[*]}
RESULTS_DIR=$RESULTS_DIR
SESSION_ID=$SESSION_ID
TIMESTAMP=$TIMESTAMP
MAX_ITERATIONS=10
EOF

echo "✓ Session initialized: $SESSION_ID"
echo "✓ Test file: $TEST_FILE"
echo "✓ Test package: $TEST_PACKAGE"
if [ ${#TEST_NAMES[@]} -gt 0 ]; then
    echo "✓ Tests: ${TEST_NAMES[*]}"
else
    echo "✓ Tests: all tests in file"
fi
echo "✓ Results directory: $RESULTS_DIR"
echo "✓ Max iterations: 10"
echo ""
cat "$SESSION_DIR/session-info.txt"
`

## Your Mission

You are executing the **test** skill for automated test iteration with the session information above.

Run the specified Go tests and iterate to fix any failures (max 10 iterations). Document everything thoroughly.

## CRITICAL: Test File Preservation

**NEVER modify the test file itself.** The test file is the source of truth for requirements.

When a test fails:
1. Fix the **implementation code** to satisfy the test
2. Fix **configuration files** if the test expects certain config
3. Fix **environment setup** if the test requires specific conditions
4. **DO NOT** change the test assertions, expected values, or test logic

If the test itself appears to be incorrectly configured, see "Test Configuration Advisory" below.

## Test Configuration Advisory

Sometimes tests fail because the **test itself** is misconfigured, not the implementation. When you detect any of the following issues, output an advisory message but **DO NOT automatically fix the test**:

### Detectable Test Issues

1. **Missing imports or fixtures**
   - Test imports packages that don't exist
   - Test references fixture files that are missing
   - Test expects environment variables not defined

2. **Syntax errors in test**
   - Compilation errors in the test file itself
   - Malformed test function signatures

3. **Impossible assertions**
   - Test expects values that can never be produced
   - Test compares unrelated types
   - Test has logical contradictions

4. **Environment mismatches**
   - Test expects Docker but Docker is unavailable
   - Test expects specific OS/architecture
   - Test expects network connectivity that's unavailable

### Advisory Format

When you detect a test configuration issue, output:

```
⚠️ TEST CONFIGURATION ADVISORY

The test file may need adjustment. This skill does NOT modify test files.

Issue detected: [description]

Suggested user action:
- [What the user should check/change in the test file]

Continuing to iterate on implementation code...
```

Then continue trying to make the implementation pass the test (if possible), or document that manual test changes are required.

## Execution Instructions

### IMPORTANT: Use TaskCreate and TaskUpdate

1. **First, create tasks** for the iteration workflow:
   ```
   Task 1: "Run test iteration 1" - Run tests and capture results
   Task 2: "Run test iteration 2" - Retry after fixes (if needed)
   ...
   Task 10: "Run test iteration 10" - Final retry (if needed)
   Task 11: "Create final summary" - Document all iterations
   ```

2. **Mark tasks in progress** before starting each iteration

3. **Mark tasks completed** when done

### Iteration Workflow (Repeat up to 10 times)

For each iteration (1 through 10):

**Step 1: Run Tests**

Read the session info from the command output above to get TEST_PACKAGE and TEST_NAMES.

Execute the tests using Bash tool:
```bash
# Build test command based on session info
if [ -n "$TEST_NAMES" ]; then
    go test $TEST_PACKAGE -run "$TEST_NAMES" -v -timeout 15m
else
    go test $TEST_PACKAGE -v -timeout 15m
fi
```

Save output to `$RESULTS_DIR/iteration-N-output.log`

**Step 2: Analyze Results**

- Check exit code (0 = pass, non-zero = fail)
- If PASS: Go to "Document Success" below
- If FAIL: Continue to Step 3

**Step 3: Diagnose Failures**

1. Read the test output log
2. Identify which tests failed
3. Extract error messages and stack traces
4. Read the test source code to understand what's being tested
5. Identify the root cause
6. **Check for test configuration issues** (see advisory section above)

Create `$SESSION_DIR/iteration-N-analysis.md` documenting:
- Failed test names
- Error messages
- Root cause analysis
- Test configuration advisory (if applicable)
- Fix plan (for implementation code only)

**Step 4: Implement Fixes**

**REMINDER: Only fix implementation code, NOT the test file.**

1. Based on analysis, identify implementation files to modify
2. Read the relevant source files
3. Make targeted fixes using Edit tool
4. Verify changes compile: `go build ./...`

Create `$SESSION_DIR/iteration-N-changes.md` documenting:
- Files modified (should NOT include test file)
- Changes made
- Rationale for each change

**Step 5: Proceed to Next Iteration**

- If this was iteration 10 and still failing: Go to "Document Failure"
- Otherwise: Return to Step 1 for next iteration

### Document Success

When tests pass, create `$RESULTS_DIR/test-results.md`:

```markdown
# Test Results: PASS

**Test File**: [file]
**Tests**: [test names]
**Final Status**: PASS
**Iterations**: [N]
**Session**: [session-id]

## Summary

Tests passed after [N] iteration(s).

## Iteration History

[For each iteration, summarize what happened]

## Changes Made

[List all changes across iterations - should be implementation code only]

## Test Output

See: $RESULTS_DIR/iteration-[N]-output.log
```

Mark final task as completed and report success to user.

### Document Failure

If tests still fail after 10 iterations, create `$RESULTS_DIR/test-results.md`:

```markdown
# Test Results: FAIL

**Test File**: [file]
**Tests**: [test names]
**Final Status**: FAIL after 10 iterations
**Session**: [session-id]

## Summary

Tests still failing after maximum 10 iterations. Manual intervention required.

## Last Error

[Error from iteration 10]

## Attempted Fixes

[Summary of all changes made to implementation code]

## Test Configuration Issues

[Any advisories about test file issues detected]

## Recommendations

[Suggestions for manual debugging, including any test changes that may be needed]

## Artifacts

- Analysis: $SESSION_DIR/iteration-*.md
- Output: $RESULTS_DIR/iteration-*-output.log
```

Report failure to user with recommendations.

## Critical Rules

1. **MAX 10 ITERATIONS** - Never exceed 10 test-fix cycles
2. **NEVER MODIFY TEST FILES** - Tests are the source of truth
3. **VERBOSE OUTPUT** - Always run tests with `-v` flag
4. **SAVE EVERYTHING** - All outputs, logs, and changes to results directory
5. **FIX ROOT CAUSE** - Don't just patch symptoms
6. **VERIFY BUILDS** - Always run `go build ./...` after changes
7. **USE TASKS** - Create and update tasks for each iteration
8. **ADVISE ON TEST ISSUES** - Output advisory when test appears misconfigured

## Screenshots (Optional Enhancement)

For Docker or UI tests, consider capturing screenshots:

1. **Check for monitoring tools** in test environment:
   - Docker: `docker ps`, `docker logs`, `docker stats`
   - UI: Browser automation tools

2. **Capture periodically** during test execution (every 10-30 seconds)

3. **Save to** `$RESULTS_DIR/screenshots/`

4. **Reference in docs** - Link screenshots in iteration analysis

This is optional - only if test involves visual monitoring.

## Example Flow

```
/iter:test tests/docker/plugin_test.go TestPluginInstallation

→ Session initialized
  ✓ Test: TestPluginInstallation
  ✓ Results: tests/results/20260126-180523-plugin-installation/
  ✓ Max iterations: 10

→ Iteration 1: FAIL
  Error: Docker image build failed
  Fix: Add missing dependency to Dockerfile

→ Iteration 2: FAIL
  Error: API key not found
  Fix: Update loadAPIKey helper

→ Iteration 3: PASS
  Duration: 2m34s

→ COMPLETE
  Results saved to tests/results/20260126-180523-plugin-installation/
```

## Important Notes

- Session state saved to `.iter/workdir/test-{slug}-{timestamp}/`
- All outputs saved to `tests/results/{timestamp}-{test-slug}/`
- Use TaskCreate/TaskUpdate for tracking progress
- Maximum 10 iterations (never exceed)
- **NEVER modify test files** - advise user if test needs changes
- Document everything in markdown files

---

Begin execution now. Create tasks, run tests, iterate until pass (max 10x).
