# iter-test-runner

Run iter-service tests, capture screenshots, analyze failures, fix code, and iterate until tests pass.

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

This skill runs iter-service integration tests and automatically fixes code issues to make tests pass.

**CRITICAL RULES:**
1. **ALWAYS use Docker** - Tests run in isolated containers
2. **NEVER modify test files** - Tests are the source of truth
3. **Fix only implementation code** - Modify files in `cmd/`, `internal/`, `pkg/`, `web/`
4. **UI tests MUST capture screenshots** - Tests use chromedp to capture PNG screenshots
5. **Each test has its own results directory** - Enforce structure below
6. **STOP conditions:**
   - Test structure is invalid (syntax errors, missing imports)
   - Test requirement is impossible
   - Maximum 5 iterations reached without progress

## Results Directory Structure (MANDATORY)

Each test MUST have its own results directory:

```
tests/results/
├── service/
│   └── {datetime}-{testname}/
│       ├── SUMMARY.md
│       ├── test-output.log
│       └── summary.json
├── api/
│   └── {datetime}-{testname}/
│       ├── SUMMARY.md
│       ├── test-output.log
│       └── summary.json
└── ui/
    └── {datetime}-{testname}/
        ├── SUMMARY.md
        ├── test-output.log
        ├── summary.json
        ├── 01-before.png           # REQUIRED: Before state
        └── 02-after.png            # REQUIRED: After state
```

**Directory naming:** `{YYYY-MM-DD_HH-MM-SS}-{testname}`
- Example: `2026-01-31_15-30-00-home-page`

## UI Screenshot Template (MANDATORY)

Every UI test MUST capture before/after PNG screenshots using chromedp:

| Screenshot | Description |
|------------|-------------|
| `01-before.png` | Initial state before test actions |
| `02-after.png` | Final state after test actions |

Some tests may have additional screenshots (e.g., `02-settings.png`, `03-docs.png`) but ALL UI tests MUST have at least `01-before.png` and a final screenshot.

### Test-Specific Screenshot Requirements

| Test | Required Screenshots |
|------|---------------------|
| TestUIHomePage | `01-before.png`, `02-after.png` |
| TestUIStyles | `01-before.png`, `02-after.png` |
| TestUIProjectList | `01-before.png`, `02-after.png` |
| TestUIProjectPage | `01-before.png`, `02-after.png` |
| TestUIDocsPage | `01-before.png`, `02-after.png` |
| TestUISettingsPage | `01-before.png`, `02-after.png` |
| TestUINavigation | `01-before.png`, `02-settings.png`, `03-docs.png`, `04-after.png` |
| TestUISearchResults | `01-before.png`, `02-after.png` |

## Workflow

### Step 1: Validate Test Structure

```bash
go build ./tests/...
```
If invalid, STOP and report: "Test structure invalid: {reason}"

### Step 2: Run Tests in Docker

```bash
cd /home/bobmc/development/iter
./tests/run-tests.sh --all
```

**Note:** UI tests use chromedp to capture screenshots automatically. Chrome/Chromium must be available in the Docker container.

### Step 3: Verify UI Screenshots Exist

After tests complete, verify each UI test directory contains the required PNG screenshots:

```bash
# Check for required screenshots
for dir in tests/results/ui/*/; do
    if [[ ! -f "$dir/01-before.png" ]]; then
        echo "FAIL: Missing 01-before.png in $dir"
    fi
    # Check for at least one "after" screenshot
    if ! ls "$dir"/*-after.png &>/dev/null && ! ls "$dir"/02-*.png &>/dev/null; then
        echo "FAIL: Missing after screenshot in $dir"
    fi
done
```

If screenshots are missing, the test has FAILED even if the Go test passed.

### Step 4: Analyze Results

Read `SUMMARY.md` from the results directory:
- `--- PASS`: Test passed (verify screenshots exist for UI tests)
- `--- FAIL`: Extract failure reason, proceed to fix

**For UI tests:** A test is ONLY considered passing if:
1. The Go test passed
2. All required PNG screenshots exist in the results directory

### Step 5: Apply Fix (if tests failed)

1. Read error message from test-output.log
2. Identify root cause
3. Locate source file causing the issue
4. Apply minimal fix
5. **DO NOT modify test files** (tests/*.go)
6. Re-run tests

### Step 6: Iterate or Stop

- If test passes AND screenshots exist: Report success, DONE
- If 5 iterations reached: Report failure, STOP
- If same error repeats 3 times: STOP with "Unable to fix: {reason}"
- Otherwise: Go to Step 2

## Output

Final output includes:
1. Test result (PASS/FAIL/STOP)
2. Number of iterations
3. Summary of fixes applied
4. Path to results directory
5. **Screenshot verification** (for UI tests)

Example output:
```
## Test Results

**Result: PASS**
**Iterations:** 1

### UI Test Screenshots Verified

| Test | Before | After | Status |
|------|--------|-------|--------|
| TestUIHomePage | 01-before.png | 02-after.png | OK |
| TestUIProjectList | 01-before.png | 02-after.png | OK |
...

### Results Directories
- tests/results/ui/2026-01-31_16-07-38-homepage/
- tests/results/ui/2026-01-31_16-07-38-projectlist/
...
```

## Test Suites

| Suite | Location | Screenshots Required |
|-------|----------|---------------------|
| service | `tests/service/` | No |
| api | `tests/api/` | No |
| ui | `tests/ui/` | **YES - chromedp captures before/after PNGs** |

## UI Test Implementation

UI tests use chromedp (Go Chrome DevTools Protocol library) to capture real browser screenshots:

```go
// Example from ui_test.go
browser, err := env.NewBrowser()
if err != nil {
    t.Fatalf("Failed to create browser: %v", err)
}
defer browser.Close()

// Capture before screenshot
if err := browser.NavigateAndScreenshot("/web/", "01-before"); err != nil {
    t.Fatalf("Failed to capture before screenshot: %v", err)
}

// ... perform test actions ...

// Capture after screenshot
if err := browser.FullPageScreenshot("02-after"); err != nil {
    t.Fatalf("Failed to capture after screenshot: %v", err)
}

// Verify screenshots exist (test will fail if missing)
env.RequireScreenshots([]string{"01-before", "02-after"})
```

## Stop Conditions

STOP immediately and report if:
1. Test file has syntax errors
2. Test expects behavior that contradicts architecture
3. Same fix fails 3 times
4. 5 iterations without progress
5. **UI test missing PNG screenshots** - Screenshots are mandatory

## Docker Requirements

- Fresh container built each run (--no-cache)
- **Chromium/Chrome must be installed** for UI tests
- Tests run sequentially (-p 1)
- Results captured from stdout/stderr
