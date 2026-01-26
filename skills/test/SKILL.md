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

## Session Initialization

!`${CLAUDE_PLUGIN_ROOT}/iter test "$(printf '%s' "$ARGUMENTS" | sed 's/"/\\"/g')" 2>&1`

## Your Mission

You are executing the **test** skill with **git worktree isolation**. The session information is shown above.

Run the specified Go tests and iterate to fix any failures (max 10 iterations). All changes are made in an isolated worktree.

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
TEST CONFIGURATION ADVISORY

The test file may need adjustment. This skill does NOT modify test files.

Issue detected: [description]

Suggested user action:
- [What the user should check/change in the test file]

Continuing to iterate on implementation code...
```

Then continue trying to make the implementation pass the test (if possible), or document that manual test changes are required.

## Execution Instructions

### Iteration Workflow (Repeat up to 10 times)

For each iteration (1 through 10):

**Step 1: Run Tests**

Execute the tests shown in the session info above.

Save output to the session workdir.

**Step 2: Analyze Results**

- Check exit code (0 = pass, non-zero = fail)
- If PASS: Go to "Complete Session" below
- If FAIL: Continue to Step 3

**Step 3: Diagnose Failures**

1. Read the test output log
2. Identify which tests failed
3. Extract error messages and stack traces
4. Read the test source code to understand what's being tested
5. Identify the root cause
6. **Check for test configuration issues** (see advisory section above)

Document:
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

Document:
- Files modified (should NOT include test file)
- Changes made
- Rationale for each change

**Step 5: Proceed to Next Iteration**

- If this was iteration 10 and still failing: Document failure and exit
- Otherwise: Return to Step 1 for next iteration

## Complete Session

When tests pass (or max iterations reached), run:

```bash
iter complete
```

This will:
1. Merge the worktree branch to the original branch
2. Clean up the worktree
3. Note: Changes are NOT pushed to remote

Create a summary document in the session workdir with:
- Test results (PASS/FAIL)
- Number of iterations
- Changes made
- Any test configuration advisories

## Critical Rules

1. **MAX 10 ITERATIONS** - Never exceed 10 test-fix cycles
2. **NEVER MODIFY TEST FILES** - Tests are the source of truth
3. **VERBOSE OUTPUT** - Always run tests with `-v` flag
4. **DOCUMENT EVERYTHING** - Save all outputs and analysis
5. **FIX ROOT CAUSE** - Don't just patch symptoms
6. **VERIFY BUILDS** - Always run `go build ./...` after changes
7. **ADVISE ON TEST ISSUES** - Output advisory when test appears misconfigured
8. **USE iter complete** - Always call `iter complete` when done

## Worktree Isolation

All changes are made in an isolated git worktree:
- Original branch is not affected until completion
- On success, changes are merged back automatically
- Changes are NOT pushed to remote (user must push manually)

## Example Flow

```
/iter:test tests/docker/plugin_test.go TestPluginInstallation

-> Session initialized in worktree
   Branch: iter/20260127-1234-test-plugin
   Test: TestPluginInstallation
   Max iterations: 10

-> Iteration 1: FAIL
   Error: Docker image build failed
   Fix: Add missing dependency to Dockerfile

-> Iteration 2: FAIL
   Error: API key not found
   Fix: Update loadAPIKey helper

-> Iteration 3: PASS
   Duration: 2m34s

-> iter complete
   Merging to main...
   Worktree cleaned up.

COMPLETE - Tests pass, changes merged to main (not pushed)
```

---

Begin execution now. Run tests, iterate until pass (max 10x), then call `iter complete`.
