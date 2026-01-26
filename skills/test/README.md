# test Skill

## Overview

The `test` skill provides automated test execution with intelligent fix iteration. It runs Go tests, analyzes failures, implements fixes, and retries up to 10 times until tests pass.

**Critical**: This skill NEVER modifies test files. Tests are the source of truth for requirements. Only implementation code is fixed.

## Git Worktree Isolation

All changes are made in an isolated git worktree:
- Original branch is not affected during iteration
- On completion, changes are merged back to the original branch
- Changes are NOT pushed to remote (user must push manually)

This ensures safe iteration without affecting the main codebase until tests pass.

## Usage

```bash
# Run specific test
/iter:test tests/docker/plugin_test.go TestPluginInstallation

# Run multiple tests
/iter:test tests/docker/iter_command_test.go TestIterRunCommandLine TestIterRunInteractive

# Run all tests in file
/iter:test tests/docker/plugin_test.go
```

## How It Works

### Iteration Loop (Max 10 iterations)

1. **Execute Test**
   - Runs `go test` with verbose output
   - Captures stdout/stderr to log file
   - Checks exit code (0 = pass, non-zero = fail)

2. **Analyze Failures** (if test fails)
   - Extracts error messages and stack traces
   - Reads test source code to understand requirements
   - Identifies root cause of failure
   - Checks for test configuration issues (see advisory below)
   - Creates fix plan for **implementation code only**

3. **Implement Fixes**
   - Makes targeted code changes to **implementation** (NEVER the test)
   - Verifies build still passes
   - Documents all changes

4. **Retry or Complete**
   - If test passes: Run `iter complete` to merge and exit
   - If iteration < 10: Retry test with fixes
   - If iteration = 10 and still failing: Document failure

### Session Completion

When tests pass (or max iterations reached), `iter complete` is called to:
1. Merge the worktree branch to the original branch
2. Clean up the worktree
3. Note: Changes are NOT pushed to remote

## Test File Preservation

**NEVER modifies test files.** When a test fails:
- Fix the **implementation code** to satisfy the test
- Fix **configuration files** if needed
- Fix **environment setup** if required
- **DO NOT** change test assertions, expected values, or test logic

## Test Configuration Advisory

When the test itself appears misconfigured (not the implementation), the skill outputs an advisory but does NOT auto-fix:

```
TEST CONFIGURATION ADVISORY

The test file may need adjustment. This skill does NOT modify test files.

Issue detected: [description]

Suggested user action:
- [What the user should check/change in the test file]

Continuing to iterate on implementation code...
```

### Detectable Test Issues:
- Missing imports or fixtures
- Syntax errors in test file
- Impossible assertions
- Environment mismatches (Docker unavailable, wrong OS, etc.)

## Output Structure

### Session Directory
`.iter/workdir/{timestamp}-test-{slug}/`
- `session-info.txt` - Test configuration
- `iteration-N-analysis.md` - Failure analysis for iteration N
- `iteration-N-changes.md` - Changes made in iteration N

### Results Directory
`tests/results/{timestamp}-{test-slug}/`
- `iteration-N-output.log` - Full test output for iteration N
- `test-results.md` - Final summary document

## Example Session

```
/iter:test tests/docker/plugin_test.go TestPluginInstallation

-> Session initialized in worktree
   Branch: iter/20260127-172345-test-plugin
   Test file: tests/docker/plugin_test.go
   Test package: ./tests/docker
   Tests: TestPluginInstallation
   Results directory: tests/results/20260127-172345-plugin-installation/
   Max iterations: 10

=== Iteration 1/10 ===
Running tests...
Tests FAILED (exit code: 1)

Analyzing failures...
- Test: TestPluginInstallation
- Error: Docker image build failed: missing dependency
- Fix Plan: Add curl package to Dockerfile

Implementing fixes...
- Modified: tests/docker/Dockerfile
- Added: curl package to apt-get install

=== Iteration 2/10 ===
Running tests...
Tests FAILED (exit code: 1)

Analyzing failures...
- Test: TestPluginInstallation
- Error: ANTHROPIC_API_KEY not found
- Fix Plan: Update loadAPIKey to check .env file

Implementing fixes...
- Modified: tests/docker/setup_test.go
- Updated: loadAPIKey function to read .env

=== Iteration 3/10 ===
Running tests...
Tests PASSED

-> iter complete
   Merging branch iter/20260127-172345-test-plugin to main...
   Worktree cleaned up.

========================================
TESTS PASSED after 3 iteration(s)
Changes merged to main (not pushed)
========================================

Results: tests/results/20260127-172345-plugin-installation/
Summary: tests/results/20260127-172345-plugin-installation/test-results.md
```

## Key Features

### Worktree Isolation
- All changes made in isolated git worktree
- Safe iteration without affecting main branch
- Automatic merge on success
- No automatic push to remote

### Comprehensive Documentation
- Every iteration fully documented
- All changes explained with rationale
- Complete test outputs preserved
- Timeline of fixes maintained

### Intelligent Analysis
- Reads test source to understand requirements
- Distinguishes root causes from symptoms
- Makes targeted fixes (not blanket changes)
- Verifies changes don't break other code
- Detects test configuration issues

### Screenshot Support (Optional)
For Docker or UI tests:
- Captures screenshots every 10-30 seconds during execution
- Saves to `tests/results/{timestamp}-{test-slug}/screenshots/`
- References screenshots in iteration docs

## Rules

1. **Max 10 Iterations** - Never exceed 10 test-fix cycles
2. **NEVER Modify Tests** - Tests are the source of truth
3. **Verbose Output** - Always run tests with `-v` flag
4. **Save Everything** - All outputs, logs, changes to results directory
5. **Fix Root Cause** - Don't patch symptoms
6. **Verify Builds** - Run `go build ./...` after changes
7. **Advise on Test Issues** - Output advisory when test appears misconfigured
8. **Use iter complete** - Always call `iter complete` when done

## Requirements

- Go test framework
- Tests runnable with `go test`
- Project has `go.mod`
- Tests have clear pass/fail criteria

## Environment Variables

- `ANTHROPIC_API_KEY` - Required for Claude integration tests
- `TEST_TIMEOUT` - Override default 15m timeout
- `TEST_SCREENSHOTS` - Enable/disable screenshots (default: auto)

## Integration

The skill integrates with the iter workflow system:
- Uses `.iter/workdir/` for session state
- Uses git worktree for isolation
- Documents in markdown format
- Follows structured analysis -> fix -> verify pattern
- Merges on completion (no push)

## Examples

### Example 1: Docker Integration Test
```bash
/iter:test tests/docker/plugin_test.go TestPluginInstallation
```

Runs the Docker integration test in an isolated worktree, fixes any issues with Docker setup, plugin installation, or API configuration, then merges on success.

### Example 2: Multiple Unit Tests
```bash
/iter:test cmd/iter/workflow_test.go TestWorkflowParsing TestWorkflowValidation
```

Runs specific unit tests, fixes parsing or validation logic issues.

### Example 3: All Tests in Package
```bash
/iter:test tests/docker/iter_command_test.go
```

Runs all Test* functions in the file, fixing any failures iteratively.

## Troubleshooting

### Test File Not Found
```
ERROR: Test file not found: tests/docker/missing_test.go
HINT: Check file path is relative to project root
```

Verify the path is correct relative to project root.

### Max Iterations Exceeded
```
FAILED: Test still failing after 10 iterations
Last Error: [error message]
Manual intervention required.
```

Review the iteration documents in the session directory to understand what was attempted and why it failed.

### Test Package Won't Compile
```
ERROR: Cannot build test package
Fix build errors before running iter-test.
```

Run `go build ./...` to identify and fix compilation errors first.

### Test Configuration Advisory Output
```
TEST CONFIGURATION ADVISORY

The test file may need adjustment. This skill does NOT modify test files.

Issue detected: Test expects environment variable API_KEY but none defined
```

Review the test file manually and make necessary corrections.
