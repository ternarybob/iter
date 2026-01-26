# iter-test Skill

## Overview

The `iter-test` skill provides automated test execution with intelligent fix iteration. It runs Go tests, analyzes failures, implements fixes, and retries up to 3 times until tests pass.

## Usage

```bash
# Run specific test
/iter-test tests/docker/plugin_test.go TestPluginInstallation

# Run multiple tests
/iter-test tests/docker/iter_command_test.go TestIterRunCommandLine TestIterRunInteractive

# Run all tests in file
/iter-test tests/docker/plugin_test.go
```

## How It Works

### Iteration Loop (Max 3 iterations)

1. **Execute Test**
   - Runs `go test` with verbose output
   - Captures stdout/stderr to log file
   - Checks exit code (0 = pass, non-zero = fail)

2. **Analyze Failures** (if test fails)
   - Extracts error messages and stack traces
   - Reads test source code to understand requirements
   - Identifies root cause of failure
   - Creates fix plan

3. **Implement Fixes**
   - Makes targeted code changes
   - Verifies build still passes
   - Documents all changes

4. **Retry or Complete**
   - If test passes: Document success and exit
   - If iteration < 3: Retry test with fixes
   - If iteration = 3 and still failing: Document failure

## Output Structure

### Session Directory
`.iter/workdir/test-{slug}-{timestamp}/`
- `session-info.txt` - Test configuration
- `iteration-N-analysis.md` - Failure analysis for iteration N
- `iteration-N-changes.md` - Changes made in iteration N

### Results Directory
`tests/results/{timestamp}-{test-slug}/`
- `iteration-N-output.log` - Full test output for iteration N
- `test-results.md` - Final summary document

## Example Session

```
/iter-test tests/docker/plugin_test.go TestPluginInstallation

✓ Session initialized: test-plugin-installation-20260126-172345
✓ Test file: tests/docker/plugin_test.go
✓ Test package: ./tests/docker
✓ Tests: TestPluginInstallation
✓ Results directory: tests/results/20260126-172345-plugin-installation/

=== Iteration 1/3 ===
→ Running tests...
✗ Tests FAILED (exit code: 1)

Analyzing failures...
- Test: TestPluginInstallation
- Error: Docker image build failed: missing dependency
- Fix Plan: Add curl package to Dockerfile

Implementing fixes...
- Modified: tests/docker/Dockerfile
- Added: curl package to apt-get install

=== Iteration 2/3 ===
→ Running tests...
✗ Tests FAILED (exit code: 1)

Analyzing failures...
- Test: TestPluginInstallation
- Error: ANTHROPIC_API_KEY not found
- Fix Plan: Update loadAPIKey to check .env file

Implementing fixes...
- Modified: tests/docker/setup_test.go
- Updated: loadAPIKey function to read .env

=== Iteration 3/3 ===
→ Running tests...
✓ Tests PASSED

========================================
✓ TESTS PASSED after 3 iteration(s)
========================================

Results: tests/results/20260126-172345-plugin-installation/
Summary: tests/results/20260126-172345-plugin-installation/test-results.md
```

## Key Features

### Task Management
- Creates tasks for each iteration using `TaskCreate`
- Updates task status with `TaskUpdate`
- Tracks progress through task list

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

### Screenshot Support (Optional)
For Docker or UI tests:
- Captures screenshots every 10-30 seconds during execution
- Saves to `tests/results/{timestamp}-{test-slug}/screenshots/`
- References screenshots in iteration docs

## Rules

1. **Max 3 Iterations** - Never exceed 3 test-fix cycles
2. **Verbose Output** - Always run tests with `-v` flag
3. **Save Everything** - All outputs, logs, changes to results directory
4. **Fix Root Cause** - Don't patch symptoms
5. **Verify Builds** - Run `go build ./...` after changes
6. **Use Tasks** - Track progress with TaskCreate/TaskUpdate

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
- Creates task list for iteration tracking
- Documents in markdown format
- Follows structured analysis → fix → verify pattern

## Examples

### Example 1: Docker Integration Test
```bash
/iter-test tests/docker/plugin_test.go TestPluginInstallation
```

Runs the Docker integration test, fixes any issues with Docker setup, plugin installation, or API configuration.

### Example 2: Multiple Unit Tests
```bash
/iter-test cmd/iter/workflow_test.go TestWorkflowParsing TestWorkflowValidation
```

Runs specific unit tests, fixes parsing or validation logic issues.

### Example 3: All Tests in Package
```bash
/iter-test tests/docker/iter_command_test.go
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
FAILED: Test still failing after 3 iterations
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
