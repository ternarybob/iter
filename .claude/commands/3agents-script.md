---
name: 3agents-script
description: Execute bash/PowerShell scripts with output capture and fix-iterate loop.
allowed-tools:
  - Read
  - Bash
  - Write
  - Edit
  - Glob
  - Grep
  - Task
  - TodoWrite
  - Skill
---

Execute: $ARGUMENTS

## EXECUTION MODE
```
┌─────────────────────────────────────────────────────────────────┐
│ AUTONOMOUS EXECUTION - NO USER INTERACTION                      │
│                                                                 │
│ • Do NOT stop for confirmation                                  │
│ • Execute script → analyze output → fix → retry (max 3)         │
│ • VERIFY OUTCOME after script completes (not just exit code)    │
│ • Use /gofix skill for Go build errors                          │
│ • All output → $WORKDIR/logs/ (never paste full logs)           │
└─────────────────────────────────────────────────────────────────┘
```

## OS DETECTION (MANDATORY)

| Indicator | OS | Script |
|-----------|-----|--------|
| `C:\...` or `D:\...` | Windows | `.\scripts\build.ps1` |
| `/home/...` or `/Users/...` | Unix/Linux/macOS | `./scripts/build.sh` |
| `/mnt/c/...` | WSL | `./scripts/build.sh` |

## SETUP
```bash
SCRIPT_FILE="$ARGUMENTS"
SCRIPT_FILE="${SCRIPT_FILE//\\//}"  # normalize path
# Extract script path (first word) and args (rest)
SCRIPT_PATH=$(echo "$SCRIPT_FILE" | awk '{print $1}')
SCRIPT_ARGS=$(echo "$SCRIPT_FILE" | cut -d' ' -f2-)
[ "$SCRIPT_ARGS" = "$SCRIPT_PATH" ] && SCRIPT_ARGS=""

TASK_SLUG=$(basename "$SCRIPT_PATH" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | cut -c1-20)
DATE=$(date +%Y-%m-%d)
TIME=$(date +%H%M)
WORKDIR=".claude/workdir/${DATE}-${TIME}-script-${TASK_SLUG}"
mkdir -p "$WORKDIR/logs"
```

## WORKFLOW

### PHASE 1: EXECUTE WITH CAPTURE

**Run script with full output capture:**
```bash
ITERATION=1
OUTPUT_LOG="$WORKDIR/logs/script_iter${ITERATION}.log"

chmod +x "$SCRIPT_PATH" 2>/dev/null || true
START=$(date +%s)

"$SCRIPT_PATH" $SCRIPT_ARGS > "$OUTPUT_LOG" 2>&1
EXIT_CODE=$?

DURATION=$(($(date +%s) - START))

if [ $EXIT_CODE -eq 0 ]; then
    echo "✓ SCRIPT PASSED (${DURATION}s)"
else
    echo "✗ SCRIPT FAILED (exit $EXIT_CODE)"
    tail -30 "$OUTPUT_LOG"
fi
```

### PHASE 2: VERIFY OUTCOME (CRITICAL)

**Exit code 0 is NOT sufficient. Verify the actual outcome:**

| Script Type | Verification Method |
|-------------|---------------------|
| `build.sh` | Binary exists: `[ -f bin/quaero ]` or `[ -f bin/quaero.exe ]` |
| `build.sh --run` | Service responds: `curl -s http://localhost:PORT/api/health` |
| `build.sh --deploy` | Files deployed: `[ -d bin/pages ]` |
| `deploy.sh` | Check deployment target is accessible |
| `test.sh` | All tests pass (check output for failures) |

**Example verification for `--run`:**
```bash
# Wait for service to initialize
sleep 3

# Get port from config
PORT=$(grep "^port" bin/quaero.toml | sed 's/.*= *//' | tr -d ' #')
[ -z "$PORT" ] && PORT=8080

# Verify service responds
if curl -s --connect-timeout 5 "http://localhost:$PORT/api/health" | grep -q "ok"; then
    echo "✓ OUTCOME VERIFIED: Service running on port $PORT"
    OUTCOME="SUCCESS"
else
    echo "✗ OUTCOME FAILED: Service not responding on port $PORT"
    OUTCOME="FAILED"
    # Check logs for errors
    LATEST_LOG=$(ls -t bin/logs/quaero.*.log 2>/dev/null | head -1)
    [ -f "$LATEST_LOG" ] && tail -20 "$LATEST_LOG" | grep -E "FTL|ERR|error"
fi

# Verify process exists
QUAERO_PID=$(pgrep -f "quaero" | head -1)
if [ -n "$QUAERO_PID" ]; then
    echo "✓ Process running: PID $QUAERO_PID"
else
    echo "✗ No quaero process found"
    OUTCOME="FAILED"
fi
```

### PHASE 3: ITERATE TO FIX (MAX 3)

```
┌─────────────────────────────────────────────────────────────────┐
│ IF OUTCOME FAILED (not just exit code):                         │
│                                                                 │
│   1. Analyze output log AND application logs for errors         │
│   2. For Go build errors → Use /gofix skill                     │
│   3. For port conflicts → Check orphan sockets, change port     │
│   4. For process death → Check nohup.out, application logs      │
│   5. Re-run script → capture to new log → verify outcome        │
│   6. Repeat until OUTCOME SUCCESS or max 3 iterations           │
│                                                                 │
│ Document each iteration in $WORKDIR/iteration_N.md              │
└─────────────────────────────────────────────────────────────────┘
```

**Common issues and fixes:**

| Issue | Symptom | Fix |
|-------|---------|-----|
| Port in use | `bind: address already in use` | Kill process or change port |
| Orphan socket (WSL) | Port stuck, no process | Change port, wait, or restart WSL |
| SIGPIPE (141) | Script fails with redirected output | Conditional `tee` transcript |
| SIGHUP | Service dies after script exits | Use `nohup` + `disown` |

**Iteration doc template (`$WORKDIR/iteration_N.md`):**
```markdown
# Iteration N
- Exit Code: {code}
- Outcome: SUCCESS/FAILED
- Duration: {seconds}s

## Verification Results
- Binary exists: YES/NO
- Service responds: YES/NO (port XXXX)
- Process running: YES/NO (PID XXXX)

## Errors Found
{extracted error lines from script log AND application logs}

## Fix Applied
```diff
- {old}
+ {new}
```

## Log: logs/script_iterN.log
```

### PHASE 4: SUMMARIZE (MANDATORY)

**Write `$WORKDIR/summary.md`:**
```markdown
# Script Execution Summary

## Script: `{script_file} {args}`
## Result: SUCCESS/FAILED
## Iterations: {n}
## Final Exit Code: {code}
## Outcome Verified: YES/NO

## Verification
| Check | Result |
|-------|--------|
| Binary exists | YES/NO |
| Service responds | YES/NO (port XXXX) |
| Process running | YES/NO (PID XXXX) |

## Issues Fixed
| Issue | Fix Applied |
|-------|-------------|
| {error} | {fix} |

## Log Files
- logs/script_iter1.log
- logs/script_iter2.log (if needed)
- bin/logs/quaero.*.log (application logs)
```

## OUTPUT LIMITS

| Type | Max Lines | Method |
|------|-----------|--------|
| Script output | 0 in context | Redirect to log file |
| Error context | 30 | `tail -30` on failure |
| Error extract | 20 | `grep -E "error|fail|FTL|ERR" \| head -20` |
| App log errors | 10 | `tail -50 app.log \| grep -E "FTL\|ERR"` |

## SKILLS REFERENCE

| Error Type | Skill |
|------------|-------|
| Go build errors | `/gofix` |
| Complex multi-step | `/3agents` |

## FORBIDDEN

| Action | Why |
|--------|-----|
| Paste full log | Context overflow |
| Stop for confirmation | Autonomous mode |
| Skip outcome verification | Task not actually complete |
| Skip summary.md | Task incomplete |
| Trust exit code alone | Exit 0 doesn't mean service works |

## INVOKE
```
/3agents-script scripts/build.sh
/3agents-script scripts/build.sh --run
/3agents-script scripts/build.sh --deploy
/3agents-script scripts/deploy.sh
```
