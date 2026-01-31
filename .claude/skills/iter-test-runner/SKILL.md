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
4. **UI tests MUST capture screenshots** - Use agent-browser for actual browser screenshots
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
        ├── 01-home-page.png        # REQUIRED: Browser screenshots
        ├── 02-project-list.png
        └── ...
```

**Directory naming:** `{YYYY-MM-DD_HH-MM-SS}-{testname}`
- Example: `2026-01-31_15-30-00-home-page`

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

### Step 3: Capture UI Screenshots (UI Tests Only)

**MANDATORY for all UI tests** - After Docker tests pass, capture actual browser screenshots:

1. Start iter-service locally:
   ```bash
   ./bin/iter-service serve &
   ```

2. Use agent-browser to capture each UI page:
   ```bash
   # Create results directory
   RESULTS_DIR="tests/results/ui/$(date +%Y-%m-%d_%H-%M-%S)-{testname}"
   mkdir -p "$RESULTS_DIR"

   # Capture screenshots
   agent-browser open http://localhost:8420/web/
   agent-browser screenshot "$RESULTS_DIR/01-home-page.png"

   agent-browser open http://localhost:8420/web/settings
   agent-browser screenshot "$RESULTS_DIR/02-settings.png"

   agent-browser open http://localhost:8420/web/docs
   agent-browser screenshot "$RESULTS_DIR/03-docs.png"

   agent-browser close
   ```

3. Stop iter-service:
   ```bash
   pkill -f iter-service
   ```

**Required screenshots for UI tests:**
- `01-home-page.png` - Home/Projects page
- `02-settings.png` - Settings page
- `03-docs.png` - API Documentation page
- `04-project-detail.png` - Project detail page (if project exists)
- Additional screenshots as needed for specific UI tests

### Step 4: Analyze Results

Read `SUMMARY.md` from the results directory:
- `--- PASS`: Test passed
- `--- FAIL`: Extract failure reason, proceed to fix

### Step 5: Apply Fix (if tests failed)

1. Read error message from test-output.log
2. Identify root cause
3. Locate source file causing the issue
4. Apply minimal fix
5. **DO NOT modify test files** (tests/*.go)
6. Re-run tests

### Step 6: Iterate or Stop

- If test passes: Report success, DONE
- If 5 iterations reached: Report failure, STOP
- If same error repeats 3 times: STOP with "Unable to fix: {reason}"
- Otherwise: Go to Step 2

## Output

Final output includes:
1. Test result (PASS/FAIL/STOP)
2. Number of iterations
3. Summary of fixes applied
4. Path to results directory
5. **List of captured screenshots** (for UI tests)

## Test Suites

| Suite | Location | Screenshots Required |
|-------|----------|---------------------|
| service | `tests/service/` | No |
| api | `tests/api/` | No |
| ui | `tests/ui/` | **YES - MANDATORY** |

## UI Screenshot Requirements

For UI tests, you MUST:
1. Start iter-service locally after Docker tests pass
2. Use `agent-browser` to navigate to each page
3. Capture PNG screenshots to the test's results directory
4. Include screenshot paths in the SUMMARY.md

Example screenshot capture workflow:
```bash
# After Docker tests pass
./scripts/build.sh -deploy
cd bin && ./iter-service serve &
sleep 2

# Capture screenshots
agent-browser open http://localhost:8420/web/
agent-browser wait --load networkidle
agent-browser screenshot tests/results/ui/{datetime}-{test}/01-home.png --full

# Continue for each page...
agent-browser close
pkill -f iter-service
```

## Stop Conditions

STOP immediately and report if:
1. Test file has syntax errors
2. Test expects behavior that contradicts architecture
3. Same fix fails 3 times
4. 5 iterations without progress
5. **UI test without screenshots** - Screenshots are mandatory

## Docker Isolation

- Fresh container built each run (--no-cache)
- No volume mounts - container is isolated
- Tests run sequentially (-p 1)
- Results captured from stdout/stderr
