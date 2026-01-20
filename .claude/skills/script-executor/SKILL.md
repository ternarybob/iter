# Script Executor Skill

**Purpose:** Autonomous bash script execution with full output capture, monitoring, iteration to fix issues, and summarization. Output captured to files to prevent context overflow.

**Input:** User provides the script path in their prompt (e.g., "Execute docker/scripts/deploy.sh"). The script path replaces `$ARGUMENTS` in all examples below.

## EXECUTION MODE
```
┌─────────────────────────────────────────────────────────────────┐
│ AUTONOMOUS BATCH EXECUTION - NO USER INTERACTION               │
│                                                                 │
│ • Do NOT stop for confirmation between phases                   │
│ • Do NOT ask "should I proceed?" or "continue?"                 │
│ • Do NOT pause after completing steps                           │
│ • Do NOT wait for user input at any point                       │
│ • ONLY stop on unrecoverable errors (missing files, no access)  │
│ • Execute ALL phases sequentially until $WORKDIR/summary.md     │
│ • ITERATE up to MAX_ITERATIONS (3) to fix issues                │
│ • CLEANUP before each iteration (scratch approach)              │
└─────────────────────────────────────────────────────────────────┘
```

## INPUT VALIDATION
```
1. Normalize path: replace \ with /
2. Must be .sh file or executable script, or STOP
3. File must exist and be readable
```

## CONFIGURATION
```bash
MAX_ITERATIONS=3        # Maximum fix iterations before stopping
CLEANUP_BEFORE_RUN=true # Always cleanup before each iteration
```

## SETUP (MANDATORY - DO FIRST)

**Create workdir BEFORE any other action:**
```bash
SCRIPT_FILE="$ARGUMENTS"                        # e.g., scripts/deploy.sh
SCRIPT_FILE="${SCRIPT_FILE//\\//}"              # normalize: replace \ with /
TASK_SLUG=$(basename "$SCRIPT_FILE" .sh)        # e.g., "deploy"
TASK_SLUG=$(echo "$TASK_SLUG" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g')
DATE=$(date +%Y-%m-%d)
TIME=$(date +%H%M)
WORKDIR=".claude/workdir/${DATE}-${TIME}-script-${TASK_SLUG}"
mkdir -p "$WORKDIR"
mkdir -p "$WORKDIR/logs"
echo "Created workdir: $WORKDIR"
```

**STOP if workdir creation fails.**

## FUNDAMENTAL RULES
```
┌─────────────────────────────────────────────────────────────────┐
│ OUTPUT CAPTURE IS MANDATORY                                     │
│                                                                 │
│ • ALL script output → $WORKDIR/logs/*.log files                 │
│ • Claude sees ONLY status + last 30 lines on failure            │
│ • NEVER let full output into context                            │
│ • Reference log files by path, don't paste contents             │
│                                                                 │
│ This prevents context overflow during long-running scripts.     │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ ARTIFACTS ARE MANDATORY                                         │
│                                                                 │
│ • $WORKDIR/script_state.md - MUST create in Phase 1             │
│ • $WORKDIR/iteration_N.md - MUST create for each iteration      │
│ • $WORKDIR/summary.md - MUST create in final phase (ALWAYS)     │
│                                                                 │
│ Task is NOT complete without summary.md in workdir.             │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ ITERATE TO FIX (MAX 3 ITERATIONS)                               │
│                                                                 │
│ • Execute script → capture output                               │
│ • Analyze output → identify issues/actions                      │
│ • Document what script did and what failed                      │
│ • Fix script or configuration issues                            │
│ • CLEANUP → run script again from scratch                       │
│ • Repeat until SUCCESS or MAX_ITERATIONS reached                │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ CLEANUP IS MANDATORY (SCRATCH APPROACH)                         │
│                                                                 │
│ • Check if script has cleanup capability (--cleanup, --clean)   │
│ • If script lacks cleanup, IMPLEMENT cleanup before execution   │
│ • Each iteration starts from clean state                        │
│ • For Docker: stop containers, remove images if needed          │
│ • For AWS: delete resources before recreating                   │
│ • For builds: clean build directories                           │
└─────────────────────────────────────────────────────────────────┘
```

## CONTEXT MANAGEMENT

**Script output can be massive. Capture everything, show summaries.**

### Output Limits (CRITICAL)
| Output Type | Max Lines in Context | Action |
|-------------|---------------------|--------|
| Script stdout/stderr | 0 (captured to file) | Always redirect to $WORKDIR/logs/*.log |
| Progress summary | 10 | Brief status updates only |
| Error summary | 30 | Use `tail -30` on failure |
| Warnings extract | 20 | grep and head |
| Final status | 5 | Exit code + duration |

**Note:** Auto-compacting is instant in modern AI assistants. Manual compaction is rarely needed.

## CONTEXT RECOVERY

If context is lost or execution is interrupted:
1. Read `$WORKDIR/script_state.md` for current state
2. Read `$WORKDIR/iteration_*.md` to find last completed iteration
3. Resume from the next iteration or Phase 3 if all iterations complete
4. Always ensure `summary.md` is written before considering task complete

## WORKFLOW

### PHASE 0: VALIDATE INPUT

```bash
SCRIPT_FILE="$ARGUMENTS"
SCRIPT_FILE="${SCRIPT_FILE//\\//}"

# Check file exists
if [ ! -f "$SCRIPT_FILE" ]; then
    echo "ERROR: Script not found: $SCRIPT_FILE"
    exit 1
fi

# Check readable
if [ ! -r "$SCRIPT_FILE" ]; then
    echo "ERROR: Script not readable: $SCRIPT_FILE"
    exit 1
fi

echo "✓ Script validated: $SCRIPT_FILE"
```

### PHASE 1: SETUP, ANALYZE & PRE-CHECK CLEANUP

**Step 1.1: Create workdir (MANDATORY)**
```bash
TASK_SLUG=$(basename "$SCRIPT_FILE" .sh)
TASK_SLUG=$(echo "$TASK_SLUG" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g')
DATE=$(date +%Y-%m-%d)
TIME=$(date +%H%M)
WORKDIR=".claude/workdir/${DATE}-${TIME}-script-${TASK_SLUG}"
mkdir -p "$WORKDIR/logs"
```

**Step 1.2: Analyze script for cleanup capability**
```bash
# Get script info
SCRIPT_SIZE=$(wc -l < "$SCRIPT_FILE")
SCRIPT_SHEBANG=$(head -1 "$SCRIPT_FILE")

# Check for cleanup capabilities
HAS_CLEANUP_FLAG=$(grep -cE "\-\-cleanup|\-\-clean|\-\-reset|\-\-destroy" "$SCRIPT_FILE" || echo 0)
HAS_CLEANUP_FUNC=$(grep -cE "cleanup\(\)|do_cleanup|clean_up" "$SCRIPT_FILE" || echo 0)
HAS_STOP_FLAG=$(grep -cE "\-\-stop|\-\-down|\-\-teardown" "$SCRIPT_FILE" || echo 0)

# Check for common patterns
HAS_SET_E=$(grep -c "set -e" "$SCRIPT_FILE" || echo 0)
HAS_SET_X=$(grep -c "set -x" "$SCRIPT_FILE" || echo 0)
USES_DOCKER=$(grep -c "docker" "$SCRIPT_FILE" || echo 0)
USES_COMPOSE=$(grep -cE "docker-compose|docker compose" "$SCRIPT_FILE" || echo 0)
USES_TERRAFORM=$(grep -c "terraform" "$SCRIPT_FILE" || echo 0)
USES_AWS=$(grep -c "aws " "$SCRIPT_FILE" || echo 0)
USES_CURL=$(grep -c "curl\|wget" "$SCRIPT_FILE" || echo 0)

# Determine cleanup strategy
if [ "$HAS_CLEANUP_FLAG" -gt 0 ] || [ "$HAS_STOP_FLAG" -gt 0 ]; then
    CLEANUP_STRATEGY="script-builtin"
elif [ "$USES_COMPOSE" -gt 0 ]; then
    CLEANUP_STRATEGY="docker-compose-down"
elif [ "$USES_DOCKER" -gt 0 ]; then
    CLEANUP_STRATEGY="docker-stop-rm"
elif [ "$USES_TERRAFORM" -gt 0 ]; then
    CLEANUP_STRATEGY="terraform-destroy"
elif [ "$USES_AWS" -gt 0 ]; then
    CLEANUP_STRATEGY="aws-resources"
else
    CLEANUP_STRATEGY="none-required"
fi
```

**Step 1.3: MUST write `$WORKDIR/script_state.md`:**
```markdown
# Script Execution State

## Script
- File: `{script_file}`
- Size: {n} lines
- Shebang: {shebang}

## Workdir
`{workdir}`

## Analysis
- Uses `set -e`: {yes/no}
- Uses `set -x`: {yes/no}
- Docker commands: {yes/no}
- Docker Compose: {yes/no}
- Terraform: {yes/no}
- AWS CLI: {yes/no}
- Network calls: {yes/no}

## Cleanup Capability
- Has --cleanup/--clean flag: {yes/no}
- Has cleanup function: {yes/no}
- Has --stop/--down flag: {yes/no}
- **Cleanup Strategy**: {strategy}

## Execution Plan
- Max iterations: 3
- Cleanup before each: YES
- Strategy: {strategy description}

## Iterations
| # | Status | Exit Code | Duration | Issues Found |
|---|--------|-----------|----------|--------------|
| 1 | PENDING | - | - | - |
| 2 | PENDING | - | - | - |
| 3 | PENDING | - | - | - |
```

### PHASE 2: ITERATE TO FIX (MAX 3 ITERATIONS)

```
┌─────────────────────────────────────────────────────────────────┐
│ FOR EACH ITERATION (1 to MAX_ITERATIONS):                       │
│                                                                 │
│   2.1. CLEANUP (except first run if nothing deployed)           │
│      → Execute cleanup based on detected strategy               │
│      → Log to $WORKDIR/logs/cleanup_iter{N}.log                 │
│                                                                 │
│   2.2. EXECUTE SCRIPT                                           │
│      → Run script with full output capture                      │
│      → Log to $WORKDIR/logs/script_iter{N}.log                  │
│                                                                 │
│   2.3. ANALYZE OUTPUT                                           │
│      → Identify what the script did (actions taken)             │
│      → Identify failures, errors, issues                        │
│      → Document in $WORKDIR/iteration_{N}.md                    │
│                                                                 │
│   2.4. DECIDE                                                   │
│      → SUCCESS (exit code 0, no critical errors) → Phase 3      │
│      → FAILURE → Investigate, Fix, Continue to next iteration   │
│                                                                 │
│   2.5. FIX (if failed)                                          │
│      → Identify root cause from output                          │
│      → Fix script, config, or environment                       │
│      → Document fix in iteration_{N}.md                         │
│                                                                 │
│ STOP if: SUCCESS or MAX_ITERATIONS reached                      │
└─────────────────────────────────────────────────────────────────┘
```

**Step 2.1: Cleanup before execution**
```bash
ITERATION=$1  # Current iteration number

# Skip cleanup on first iteration if nothing to clean
if [ "$ITERATION" -eq 1 ] && [ "$CLEANUP_STRATEGY" = "none-required" ]; then
    echo "Iteration 1: No cleanup required (fresh start)"
else
    echo "Iteration $ITERATION: Running cleanup..."
    CLEANUP_LOG="$WORKDIR/logs/cleanup_iter${ITERATION}.log"

    case "$CLEANUP_STRATEGY" in
        "script-builtin")
            # Use script's own cleanup
            if grep -qE "\-\-cleanup" "$SCRIPT_FILE"; then
                "$SCRIPT_FILE" --cleanup > "$CLEANUP_LOG" 2>&1 || true
            elif grep -qE "\-\-clean" "$SCRIPT_FILE"; then
                "$SCRIPT_FILE" --clean > "$CLEANUP_LOG" 2>&1 || true
            elif grep -qE "\-\-stop" "$SCRIPT_FILE"; then
                "$SCRIPT_FILE" --stop > "$CLEANUP_LOG" 2>&1 || true
            fi
            ;;
        "docker-compose-down")
            # Find and use docker-compose file
            COMPOSE_FILE=$(grep -oE "[a-zA-Z0-9/_.-]+docker-compose[a-zA-Z0-9/_.-]*\.ya?ml" "$SCRIPT_FILE" | head -1)
            if [ -n "$COMPOSE_FILE" ] && [ -f "$COMPOSE_FILE" ]; then
                docker compose -f "$COMPOSE_FILE" down -v --remove-orphans > "$CLEANUP_LOG" 2>&1 || true
            else
                docker compose down -v --remove-orphans > "$CLEANUP_LOG" 2>&1 || true
            fi
            ;;
        "docker-stop-rm")
            # Stop and remove Docker containers/images from script
            docker container prune -f > "$CLEANUP_LOG" 2>&1 || true
            ;;
        "terraform-destroy")
            # Terraform destroy (careful!)
            echo "Terraform cleanup skipped (manual intervention required)" > "$CLEANUP_LOG"
            ;;
        "aws-resources")
            # AWS resource cleanup (careful!)
            echo "AWS cleanup skipped (manual intervention required)" > "$CLEANUP_LOG"
            ;;
        *)
            echo "No cleanup required" > "$CLEANUP_LOG"
            ;;
    esac

    echo "Cleanup completed: $CLEANUP_LOG"
fi
```

**Step 2.2: Execute script with full capture**
```bash
ITERATION=$1
OUTPUT_LOG="$WORKDIR/logs/script_iter${ITERATION}.log"
TIMING_LOG="$WORKDIR/logs/timing_iter${ITERATION}.log"

# Make script executable if needed
chmod +x "$SCRIPT_FILE" 2>/dev/null || true

# Record start
START_TIME=$(date +%s)
START_TIMESTAMP=$(date -Iseconds)
echo "Started: $START_TIMESTAMP" > "$TIMING_LOG"

# Execute with full output capture
"$SCRIPT_FILE" > "$OUTPUT_LOG" 2>&1
EXIT_CODE=$?

# Record completion
END_TIME=$(date +%s)
END_TIMESTAMP=$(date -Iseconds)
DURATION=$((END_TIME - START_TIME))

echo "Ended: $END_TIMESTAMP" >> "$TIMING_LOG"
echo "Duration: ${DURATION}s" >> "$TIMING_LOG"
echo "Exit code: $EXIT_CODE" >> "$TIMING_LOG"
```

**Step 2.3: Analyze output and document actions**

**MUST read the script output log and identify:**
1. **Actions Taken**: What did the script actually do? (created files, built images, deployed services, etc.)
2. **Success Indicators**: What completed successfully?
3. **Failure Points**: Where did it fail? What error messages?
4. **Root Cause**: Why did it fail? (missing dependency, config error, permission issue, etc.)

```bash
# Extract key information (DO NOT paste full logs)
TOTAL_LINES=$(wc -l < "$OUTPUT_LOG")
ERROR_COUNT=$(grep -ciE "error|failed|fatal|exception" "$OUTPUT_LOG" || echo 0)
WARNING_COUNT=$(grep -ciE "warn|warning" "$OUTPUT_LOG" || echo 0)
SUCCESS_COUNT=$(grep -ciE "success|completed|done|passed|created" "$OUTPUT_LOG" || echo 0)

# Create extraction summary
EXTRACT_LOG="$WORKDIR/logs/extract_iter${ITERATION}.log"
{
    echo "=== ITERATION $ITERATION SUMMARY ==="
    echo "Exit code: $EXIT_CODE"
    echo "Duration: ${DURATION}s"
    echo "Total output lines: $TOTAL_LINES"
    echo "Success mentions: $SUCCESS_COUNT"
    echo "Error mentions: $ERROR_COUNT"
    echo "Warning mentions: $WARNING_COUNT"
    echo ""

    if [ "$EXIT_CODE" -ne 0 ]; then
        echo "=== LAST 30 LINES (failure context) ==="
        tail -30 "$OUTPUT_LOG"
        echo ""
    fi

    if [ "$ERROR_COUNT" -gt 0 ]; then
        echo "=== ERROR LINES (first 20) ==="
        grep -iE "error|failed|fatal|exception" "$OUTPUT_LOG" | head -20
        echo ""
    fi

    echo "=== ACTIONS DETECTED ==="
    grep -iE "creating|copying|building|deploying|starting|running|installing" "$OUTPUT_LOG" | head -20
    echo ""

    echo "=== SUCCESS INDICATORS ==="
    grep -iE "success|completed|done|passed|created|built" "$OUTPUT_LOG" | head -10
} > "$EXTRACT_LOG"
```

**Step 2.4: MUST write `$WORKDIR/iteration_{N}.md`:**
```markdown
# Iteration {N}

## Execution
- Start: {timestamp}
- End: {timestamp}
- Duration: {duration}s
- Exit Code: {exit_code}
- Status: {SUCCESS/FAILED}

## Cleanup Performed
{What cleanup was done before this iteration}

## Actions Observed
{List of actions the script took, extracted from output}

1. {action 1 - e.g., "Created Docker build context"}
2. {action 2 - e.g., "Built Docker image vlab-auslab-runtime:latest"}
3. {action 3 - e.g., "Started container via docker-compose"}

## Success Points
{What completed successfully}

- {success 1}
- {success 2}

## Failure Points
{What failed and why}

| Error | Location | Root Cause |
|-------|----------|------------|
| {error message} | {file:line or command} | {why it failed} |

## Investigation
{Analysis of the failure - what went wrong and why}

## Fix Applied
{If iteration > 1, what fix was applied before this run}

```diff
- {old code/config}
+ {new code/config}
```

## Files Modified
- {file 1}: {change description}
- {file 2}: {change description}

## Log Files
- Output: logs/script_iter{N}.log
- Cleanup: logs/cleanup_iter{N}.log
- Extract: logs/extract_iter{N}.log

## Next Steps
{If failed, what needs to be fixed for next iteration}
```

**Step 2.5: Fix and Continue (if failed)**

If the script failed:
1. Read the error output from the log
2. Identify the root cause
3. Determine if the issue is in:
   - The script itself (fix the script)
   - Configuration files (fix the config)
   - Environment (may not be fixable)
   - External dependencies (may not be fixable)
4. Apply the fix
5. Document the fix in iteration_{N}.md
6. Continue to next iteration

**Continue to next iteration or Phase 3 if success/max reached**

### PHASE 3: SUMMARIZE (MANDATORY)

**This phase MUST execute. Task is incomplete without it.**

**MUST write `$WORKDIR/summary.md`:**
```markdown
# Script Execution Summary

## Script
- **File**: `{script_file}`
- **Workdir**: `{workdir}`
- **Executed**: {timestamp}

## Result
| Metric | Value |
|--------|-------|
| Total Iterations | {n} |
| Final Exit Code | {exit_code} |
| Final Status | {SUCCESS/FAILED} |
| Total Duration | {duration}s |

## Iteration Summary
| # | Status | Exit Code | Duration | Key Issue |
|---|--------|-----------|----------|-----------|
| 1 | {status} | {code} | {time}s | {issue or "None"} |
| 2 | {status} | {code} | {time}s | {issue or "None"} |
| 3 | {status} | {code} | {time}s | {issue or "None"} |

## Issues Identified and Fixed

### Issue 1: {title}
**Root Cause**: {explanation}

**Fix Applied**:
```diff
- {old}
+ {new}
```

**Files Changed**:
- {file path}

### Issue 2: {title}
...

## Remaining Issues (if any)
{Issues that could not be fixed within MAX_ITERATIONS}

| Issue | Severity | Suggested Fix |
|-------|----------|---------------|
| {issue} | {HIGH/MEDIUM/LOW} | {suggestion} |

## Actions Performed by Script
{Summary of what the script does when successful}

1. {action 1}
2. {action 2}
3. {action 3}

## Cleanup Strategy Used
- **Strategy**: {strategy}
- **Commands**: {cleanup commands used}

## Log Files
| File | Purpose | Lines |
|------|---------|-------|
| logs/script_iter1.log | Iteration 1 output | {n} |
| logs/script_iter2.log | Iteration 2 output | {n} |
| logs/cleanup_iter1.log | Cleanup output | {n} |
| logs/extract_iter*.log | Extracted summaries | {n} |

## Recommendations
- [ ] {recommendation 1}
- [ ] {recommendation 2}

## Quick Reference
\`\`\`bash
# Re-run script
{script_file}

# Cleanup
{cleanup_command}

# View iteration logs
cat "$WORKDIR/logs/script_iter1.log"
cat "$WORKDIR/logs/script_iter2.log"
\`\`\`
```

**Step 3.2: Verify summary was written**
```bash
if [ -f "$WORKDIR/summary.md" ]; then
    echo "✓ Summary written: $WORKDIR/summary.md"
    ls -la "$WORKDIR/summary.md"
else
    echo "✗ ERROR: Summary not written!"
fi
```

**Step 3.3: Final status report**
```bash
echo ""
echo "=========================================="
echo "SCRIPT EXECUTION COMPLETE"
echo "=========================================="
echo "Script: $SCRIPT_FILE"
echo "Iterations: $TOTAL_ITERATIONS"
echo "Final Status: $([ $FINAL_EXIT_CODE -eq 0 ] && echo "✓ SUCCESS" || echo "✗ FAILED")"
echo "Final Exit code: $FINAL_EXIT_CODE"
echo "Total Duration: ${TOTAL_DURATION}s"
echo ""
echo "Artifacts:"
echo "  - Summary: $WORKDIR/summary.md"
echo "  - State: $WORKDIR/script_state.md"
for i in $(seq 1 $TOTAL_ITERATIONS); do
    echo "  - Iteration $i: $WORKDIR/iteration_$i.md"
done
echo "  - Logs: $WORKDIR/logs/"
echo "=========================================="
```

---

## CODEBUFF AGENT USAGE

When using this skill in Codebuff, spawn agents using the proper JSON format:

### Commander Agent for Script Execution
```json
{
  "agents": [
    {
      "agent_type": "commander",
      "prompt": "Execute script and capture output, report exit code",
      "params": {
        "command": "bash docker/scripts/deploy.sh > .claude/workdir/logs/script_iter1.log 2>&1; echo \"Exit code: $?\"",
        "timeout_seconds": 300
      }
    }
  ]
}
```

### File-Picker for Script Discovery
```json
{
  "agents": [
    {
      "agent_type": "file-picker",
      "prompt": "Find bash scripts in the project",
      "params": {
        "directories": ["scripts", "docker/scripts", "aws-magentus/scripts"]
      }
    }
  ]
}
```

### Code-Searcher for Script Analysis
```json
{
  "agents": [
    {
      "agent_type": "code-searcher",
      "params": {
        "searchQueries": [
          {
            "pattern": "docker|terraform|aws",
            "flags": "-g *.sh"
          }
        ]
      }
    }
  ]
}
```

### Editor for Fixes
```json
{
  "agents": [
    {
      "agent_type": "editor"
    }
  ]
}
```
Note: Editor agent inherits conversation context, no prompt needed.

---

## FORBIDDEN PHRASES
```
┌─────────────────────────────────────────────────────────────────┐
│ NEVER OUTPUT THESE:                                             │
│                                                                 │
│ • "Should I proceed?"                                           │
│ • "Ready to continue?"                                          │
│ • "Let me know when..."                                         │
│ • "Would you like me to..."                                     │
│ • "Shall I..."                                                  │
│ • "Do you want me to..."                                        │
│ • "I'll wait for..."                                            │
│ • "Before I continue..."                                        │
│ • Any question expecting user response                          │
│                                                                 │
│ INSTEAD: Just do it. Document in $WORKDIR. Keep moving.         │
└─────────────────────────────────────────────────────────────────┘
```

---

## FORBIDDEN ACTIONS

| Action | Result |
|--------|--------|
| Stop for user confirmation | FAILURE |
| Ask questions expecting response | FAILURE |
| Paste full log output into context | FAILURE |
| Cat entire log file | FAILURE |
| Skip writing summary.md | FAILURE |
| Skip writing iteration_N.md | FAILURE |
| Show more than 30 lines of error output | FAILURE |
| Let unbounded output into Claude context | FAILURE |
| Skip cleanup between iterations | FAILURE |
| Not investigating script output | FAILURE |
| Not documenting actions from script | FAILURE |

## ALLOWED ACTIONS

| Action | Rationale |
|--------|-----------|
| `tail -30` on log files | Bounded output |
| `head -20` on log files | Bounded output |
| `grep pattern \| head -N` | Filtered + bounded |
| `wc -l` for line counts | Single number |
| Extract specific patterns | Targeted information |
| Proceed without confirmation | Autonomous execution |
| Reference log paths without pasting | Preserves context |
| Fix scripts/configs during iteration | Part of iterate-to-fix |
| Implement cleanup if script lacks it | Part of pre-check |

## OUTPUT EXTRACTION HELPERS

```bash
# Extract errors (max 20 lines)
extract_errors() {
    local LOG_FILE=$1
    grep -iE "error|failed|fatal|exception" "$LOG_FILE" | head -20
}

# Extract warnings (max 10 lines)
extract_warnings() {
    local LOG_FILE=$1
    grep -iE "warn|warning" "$LOG_FILE" | head -10
}

# Extract actions (max 20 lines)
extract_actions() {
    local LOG_FILE=$1
    grep -iE "creating|copying|building|deploying|starting|running|installing|downloading" "$LOG_FILE" | head -20
}

# Get failure context (last 30 lines)
failure_context() {
    local LOG_FILE=$1
    echo "=== Last 30 lines ==="
    tail -30 "$LOG_FILE"
}

# Quick stats
log_stats() {
    local LOG_FILE=$1
    echo "Lines: $(wc -l < "$LOG_FILE")"
    echo "Actions: $(grep -ciE 'creating|building|deploying' "$LOG_FILE" || echo 0)"
    echo "Errors: $(grep -ciE 'error|fail' "$LOG_FILE" || echo 0)"
    echo "Warnings: $(grep -ciE 'warn' "$LOG_FILE" || echo 0)"
}
```

## CLEANUP STRATEGIES REFERENCE

| Detected Pattern | Cleanup Strategy | Commands |
|-----------------|------------------|----------|
| `--cleanup` flag in script | script-builtin | `$SCRIPT_FILE --cleanup` |
| `--stop` flag in script | script-builtin | `$SCRIPT_FILE --stop` |
| docker-compose/docker compose | docker-compose-down | `docker compose -f FILE down -v --remove-orphans` |
| docker commands | docker-stop-rm | `docker container prune -f` |
| terraform | terraform-destroy | Manual (dangerous) |
| aws CLI | aws-resources | Manual (dangerous) |
| None detected | none-required | Skip cleanup |

## WORKDIR ARTIFACTS (MANDATORY)

| File | Purpose | When Created | Required |
|------|---------|--------------|----------|
| `script_state.md` | Execution state tracking | Phase 1 | **YES** |
| `iteration_N.md` | Per-iteration analysis | Phase 2 (each iter) | **YES** |
| `summary.md` | Final execution summary | Phase 3 | **YES - ALWAYS** |
| `logs/` | All captured output | Throughout | **YES** |
| `logs/script_iter*.log` | Script output per iteration | Phase 2 | **YES** |
| `logs/cleanup_iter*.log` | Cleanup output per iteration | Phase 2 | **YES** |
| `logs/timing_iter*.log` | Timing per iteration | Phase 2 | **YES** |
| `logs/extract_iter*.log` | Extracted summaries | Phase 2 | **YES** |

**Task is NOT complete until `summary.md` exists in workdir.**

## INVOKE EXAMPLE

```bash
# Execute a deployment script
# → .claude/workdir/2024-12-17-1430-script-deploy/
#    ├── script_state.md     (created Phase 1)
#    ├── iteration_1.md      (created Phase 2)
#    ├── iteration_2.md      (created Phase 2, if needed)
#    ├── iteration_3.md      (created Phase 2, if needed)
#    ├── summary.md          (created Phase 3 - REQUIRED)
#    └── logs/
#        ├── script_iter1.log
#        ├── script_iter2.log
#        ├── cleanup_iter1.log
#        ├── cleanup_iter2.log
#        ├── timing_iter1.log
#        ├── extract_iter1.log
#        └── ...
```

## ITERATION FLOW DIAGRAM

```
┌─────────────────────────────────────────────────────────────────┐
│                    ITERATION FLOW                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   Phase 1: Analyze Script                                       │
│      │                                                          │
│      ├── Check for cleanup capability                           │
│      ├── Determine cleanup strategy                             │
│      └── Write script_state.md                                  │
│      │                                                          │
│      ▼                                                          │
│   ┌─────────────────────────────────────────┐                   │
│   │ ITERATION LOOP (max 3)                  │                   │
│   │                                         │                   │
│   │   ┌─────────────────┐                   │                   │
│   │   │ 1. CLEANUP      │ (skip if iter 1)  │                   │
│   │   │    - docker down│                   │                   │
│   │   │    - script     │                   │                   │
│   │   │      --cleanup  │                   │                   │
│   │   └────────┬────────┘                   │                   │
│   │            ▼                            │                   │
│   │   ┌─────────────────┐                   │                   │
│   │   │ 2. EXECUTE      │                   │                   │
│   │   │    - Run script │                   │                   │
│   │   │    - Capture out│                   │                   │
│   │   └────────┬────────┘                   │                   │
│   │            ▼                            │                   │
│   │   ┌─────────────────┐                   │                   │
│   │   │ 3. ANALYZE      │                   │                   │
│   │   │    - Read output│                   │                   │
│   │   │    - Find errors│                   │                   │
│   │   │    - Doc actions│                   │                   │
│   │   └────────┬────────┘                   │                   │
│   │            ▼                            │                   │
│   │   ┌─────────────────┐                   │                   │
│   │   │ 4. DOCUMENT     │                   │                   │
│   │   │    - Write      │                   │                   │
│   │   │      iteration_ │                   │                   │
│   │   │      N.md       │                   │                   │
│   │   └────────┬────────┘                   │                   │
│   │            ▼                            │                   │
│   │   ┌─────────────────┐                   │                   │
│   │   │ 5. SUCCESS?     │                   │                   │
│   │   └────────┬────────┘                   │                   │
│   │            │                            │                   │
│   │      ┌─────┴─────┐                      │                   │
│   │      │           │                      │                   │
│   │     YES          NO                     │                   │
│   │      │           │                      │                   │
│   │      │    ┌──────┴──────┐               │                   │
│   │      │    │ 6. FIX      │               │                   │
│   │      │    │    - Find   │               │                   │
│   │      │    │      cause  │               │                   │
│   │      │    │    - Apply  │               │                   │
│   │      │    │      fix    │               │                   │
│   │      │    └──────┬──────┘               │                   │
│   │      │           │                      │                   │
│   │      │    iter++ │                      │                   │
│   │      │           │                      │                   │
│   │      │    ┌──────┴──────┐               │                   │
│   │      │    │ iter <= 3?  │               │                   │
│   │      │    └──────┬──────┘               │                   │
│   │      │           │                      │                   │
│   │      │     ┌─────┴─────┐                │                   │
│   │      │    YES          NO               │                   │
│   │      │     │           │                │                   │
│   │      │     │     ┌─────┴─────┐          │                   │
│   │      │     │     │ MAX ITER  │          │                   │
│   │      │     │     │ REACHED   │          │                   │
│   │      │     │     └─────┬─────┘          │                   │
│   │      │     │           │                │                   │
│   │      │  LOOP ──────────┘                │                   │
│   │      │                                  │                   │
│   └──────┼──────────────────────────────────┘                   │
│          ▼                                                      │
│   Phase 3: Write summary.md                                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```
