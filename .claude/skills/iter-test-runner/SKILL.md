# iter-test-runner

Run iter-service tests with MANDATORY screenshot capture, log collection, and summary output.

## Usage

```
/iter-test-runner [test-name-or-pattern]
```

## Examples

```
/iter-test-runner TestProjectIsolation
/iter-test-runner TestIndexStatus
/iter-test-runner --all
```

## CRITICAL REQUIREMENTS (ENFORCE OR FAIL)

These are NOT optional. If a test does not meet these requirements, the skill MUST FAIL and report the deficiency. The skill CANNOT modify test files to fix these issues.

### 1. Screenshots are MANDATORY for ALL Tests

**EVERY test** must capture before/after screenshots to prove execution:

| Screenshot | When | Purpose |
|------------|------|---------|
| `01-before.png` | Before any test actions | Prove initial state |
| `02-after.png` | After test completes | Prove final state |

**Enforcement:** After test run, verify screenshots exist. If missing:
```
FAIL: Test {TestName} missing required screenshots
  - Expected: 01-before.png, 02-after.png
  - Found: {list actual files}

ACTION REQUIRED: Test must be updated to capture screenshots.
This skill cannot modify test files.
```

### 2. Log Collection is MANDATORY

Every test results directory MUST contain:

| File | Source | Content |
|------|--------|---------|
| `test-output.log` | Go test stdout/stderr | Test execution output |
| `iter-service.log` | iter-service container | Service logs |
| `container.log` | Docker container logs | Container runtime logs |
| `summary.json` | Test framework | Structured test results |
| `SUMMARY.md` | Test framework | Human-readable summary |

**Enforcement:** After test run, verify all logs exist. If missing, FAIL.

### 3. Summary Output is MANDATORY

Every test MUST produce:

**summary.json:**
```json
{
  "test_name": "TestProjectIsolation",
  "passed": true,
  "duration": "20.5s",
  "timestamp": "2026-02-01T14:00:23Z",
  "screenshots": ["01-before.png", "02-after.png"],
  "logs": ["test-output.log", "iter-service.log", "container.log"],
  "details": "MCP returns project-specific data correctly",
  "errors": []
}
```

**SUMMARY.md:**
```markdown
# Test: TestProjectIsolation

**Result:** PASS
**Duration:** 20.5s
**Timestamp:** 2026-02-01T14:00:23Z

## Screenshots
- 01-before.png - Initial state
- 02-after.png - Final state

## Logs
- test-output.log
- iter-service.log
- container.log

## Details
MCP returns project-specific data correctly.

## Errors
None
```

### 4. Results Directory Structure (Per-Test, NOT Per-Run)

Results are organized BY TEST NAME, not by timestamp:

```
tests/results/
├── api/
│   ├── TestIndexStatusAPIWithoutProjects/
│   │   ├── SUMMARY.md
│   │   ├── summary.json
│   │   ├── test-output.log
│   │   ├── iter-service.log
│   │   ├── 01-before.png
│   │   └── 02-after.png
│   └── TestIndexStatusAPIWithProjects/
│       └── ...
├── mcp/
│   ├── TestProjectIsolation/
│   │   ├── SUMMARY.md
│   │   ├── summary.json
│   │   ├── test-output.log
│   │   ├── iter-service.log
│   │   ├── container.log
│   │   ├── 01-before.png
│   │   └── 02-after.png
│   └── TestIndexStatusWithoutGeminiAPIKey/
│       └── ...
└── ui/
    ├── TestIndexStatusUIWithoutProjects/
    │   └── ...
    └── TestIndexStatusUIWithProjects/
        └── ...
```

**Key:** Each test run OVERWRITES the previous results for that test. This ensures you always see the latest results for each test.

## Workflow

### Step 1: Validate Test Structure

```bash
go build ./tests/...
```

If build fails, STOP and report: "Test structure invalid: {reason}"

### Step 2: Run Tests

```bash
go test -v ./tests/{suite}/... -run {pattern} -timeout 300s 2>&1 | tee test-output.log
```

### Step 3: Verify MANDATORY Outputs

After each test completes, verify:

```bash
# For each test results directory:
REQUIRED_FILES=(
    "SUMMARY.md"
    "summary.json"
    "test-output.log"
    "01-before.png"
    "02-after.png"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [[ ! -f "$RESULTS_DIR/$file" ]]; then
        echo "FAIL: Missing required file: $file"
        MISSING_FILES+=("$file")
    fi
done

if [[ ${#MISSING_FILES[@]} -gt 0 ]]; then
    echo ""
    echo "TEST INFRASTRUCTURE FAILURE"
    echo "The test does not meet mandatory requirements."
    echo "Missing: ${MISSING_FILES[*]}"
    echo ""
    echo "ACTION REQUIRED: Update the test to capture screenshots and generate summaries."
    echo "This skill CANNOT modify test files."
    exit 1
fi
```

### Step 4: Generate Final Report

Output MUST include:

```markdown
## iter-test-runner Results

**Test:** {TestName}
**Result:** PASS | FAIL | INFRASTRUCTURE_FAILURE
**Duration:** {duration}
**Iterations:** {count}

### Artifacts Verified

| File | Status |
|------|--------|
| SUMMARY.md | OK |
| summary.json | OK |
| test-output.log | OK |
| iter-service.log | OK |
| 01-before.png | OK |
| 02-after.png | OK |

### Results Directory
`tests/results/{suite}/{TestName}/`

### Summary
{Contents of SUMMARY.md}

### Fixes Applied (if any)
1. {fix description}
2. {fix description}

### Recommendations
- {recommendation if test failed}
```

## STOP Conditions

STOP immediately and report INFRASTRUCTURE_FAILURE if:

1. **Screenshots missing** - Test did not capture before/after screenshots
2. **Summary missing** - Test did not generate SUMMARY.md or summary.json
3. **Logs missing** - Test did not collect required logs
4. Test file has syntax errors
5. Maximum 5 iterations reached without progress
6. Same error repeats 3 times

**Important:** For infrastructure failures (missing screenshots, summaries, logs), the skill CANNOT fix the problem because it cannot modify test files. It must STOP and report what is missing.

## Test Implementation Requirements

For a test to be compatible with this runner, it MUST:

```go
func TestExample(t *testing.T) {
    // 1. Create test environment with proper results directory
    env := common.NewTestEnv(t, "mcp", "example")
    defer env.Cleanup()

    startTime := time.Now()

    // 2. Start service
    if err := env.Start(); err != nil {
        t.Fatalf("Failed to start: %v", err)
    }

    // 3. Create browser for screenshots
    browser, err := env.NewBrowser()
    if err != nil {
        t.Fatalf("Failed to create browser: %v", err)
    }
    defer browser.Close()

    // 4. MANDATORY: Capture before screenshot
    if err := browser.NavigateAndScreenshot("/", "01-before"); err != nil {
        t.Fatalf("Failed to capture before screenshot: %v", err)
    }

    // 5. Perform test actions
    // ...

    // 6. MANDATORY: Capture after screenshot
    if err := browser.FullPageScreenshot("02-after"); err != nil {
        t.Fatalf("Failed to capture after screenshot: %v", err)
    }

    // 7. MANDATORY: Verify screenshots exist
    env.RequireScreenshots([]string{"01-before", "02-after"})

    // 8. MANDATORY: Write summary
    duration := time.Since(startTime)
    env.WriteSummary(true, duration, "Test completed successfully")
}
```

## Docker Requirements

- Chromium/Chrome must be installed for screenshot capture
- Container logs must be accessible
- Service logs must be written to results directory
