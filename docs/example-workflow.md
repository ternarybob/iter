VLAB Workflow V2 - Iterative Issue Resolution
SOURCES & SCOPE
Code Sources
Source	Path	Purpose
DevOps Repo	/mnt/c/development/magentus/devops	Docker configs, entrypoints, compose files
VLAB Source	/mnt/c/development/magentus/vlab-auslab	C++ source (sqladapter, distweb, daemon)
Build Output	/mnt/c/development/magentus/devops/output	Compiled binaries, tar files
Skills	.claude/skills/vlab/	Reusable workflow skills
Docker Services
Container	Port	Purpose
vlab-backend	8082, 8085, 8086, 8094	AUSLAB daemon (HL7, NCSR, RESTful API, ttyd)
vlab-monitor	8092	Health monitoring API, ITT testing interface
vlab-terminal	8093	Web-based terminal
auslab-postgres	5432	PostgreSQL database
auslab-redis	6379	Redis cache
ITT (Interface Testing Tool) Endpoints
Endpoint	Method	Purpose
/itt	GET	Web UI for interface testing
/api/itt/scripts	GET	List available ITT test scripts
/api/itt/scripts/{name}	GET	Get script content
/api/itt/run	POST	Execute ITT script (JSON body: {"script": "..."})
Available ITT Scripts
Script Name	Purpose
hl7_eorder_test	Test HL7 eOrder message sending to AUSLAB backend
http_api_test	Test AUSLAB REST API endpoints (health, settings)
monitor_health_check	Test monitor's own health endpoints
Key Configuration Files
File	Purpose
docker/magentus-services/docker-compose.yml	Service definitions, networking
docker/config/03-entrypoint.sh	Backend container entrypoint
docker/config/entrypoint-terminal.sh	Terminal container entrypoint
EXECUTION RULES
+------------------------------------------------------------------+
|                    AUTONOMOUS EXECUTION                           |
+------------------------------------------------------------------+
| YOU (Claude) MUST:                                                |
|  - Execute ALL commands directly - never tell users to run them   |
|  - Apply ALL fixes using Edit/Write/Bash tools                    |
|  - Continue through iterations without user confirmation          |
|  - Document THINKING behind each decision                         |
|  - Read PREVIOUS iteration notes before each new iteration        |
|  - Fix ONE issue per iteration (highest priority)                 |
|                                                                   |
| ONLY STOP WHEN:                                                   |
|  - SUCCESS: System stable for 2+ iterations                       |
|  - MAX_ITERATIONS (5) reached                                     |
|  - Unrecoverable error (build failure, missing dependencies)      |
+------------------------------------------------------------------+
CONFIGURATION
MIN_ITERATIONS=2           # Minimum iterations to validate stability
MAX_ITERATIONS=5           # Maximum iterations before stopping
STABILIZATION_WAIT=60      # Seconds to wait after deploy for stability check
REPO_ROOT=/mnt/c/development/magentus/devops
WORKFLOW SETUP
FIRST: Create the workflow directory structure:

REPO_ROOT=$(git rev-parse --show-toplevel)
WORKDIR="$REPO_ROOT/.claude/workdir/$(date +%Y-%m-%d-%H%M)-vlab-workflow-v2"
mkdir -p "$WORKDIR"

# Create workflow state file
cat > "$WORKDIR/workflow-state.md" << 'EOF'
# Workflow State

| Field | Value |
|-------|-------|
| Started | $(date -Iseconds) |
| Status | IN_PROGRESS |
| Current Iteration | 1 |
| Total Issues Fixed | 0 |
EOF

echo "WORKDIR=$WORKDIR"
ITERATION STRUCTURE
Each iteration follows this exact structure:

+==========================================+
|           ITERATION N STRUCTURE          |
+==========================================+
|                                          |
|  STEP 1: GET STATUS                      |
|  - Read previous iteration notes         |
|  - Collect current state (docker/logs)   |
|  - Check all service endpoints           |
|                                          |
|  STEP 2: REVIEW & PLAN                   |
|  - Analyze issues found                  |
|  - Select ONE priority issue             |
|  - Document thinking/rationale           |
|                                          |
|  STEP 3: IMPLEMENT                       |
|  - Execute ordered fix actions           |
|  - Document each change made             |
|  - Redeploy/restart as needed            |
|                                          |
|  STEP 4: SUMMARIZE                       |
|  - Record actions taken                  |
|  - Record outcomes/results               |
|  - Determine if another iteration needed |
|                                          |
+==========================================+
ITERATION LOOP
For each iteration (N = 1 to MAX_ITERATIONS):

Create Iteration Directory
ITERATION=N
ITER_DIR="$WORKDIR/iteration-$ITERATION"
mkdir -p "$ITER_DIR"
mkdir -p "$ITER_DIR/status"
mkdir -p "$ITER_DIR/logs"
mkdir -p "$ITER_DIR/fixes"
STEP 1: GET STATUS
1.1 Read Previous Iteration Notes (MANDATORY for N > 1)

if [ "$ITERATION" -gt 1 ]; then
    PREV_ITER=$((ITERATION - 1))
    PREV_NOTES="$WORKDIR/iteration-$PREV_ITER/notes.md"

    if [ -f "$PREV_NOTES" ]; then
        echo "Reading previous iteration notes from: $PREV_NOTES"
        cat "$PREV_NOTES"
    else
        echo "WARNING: No previous notes found at $PREV_NOTES"
    fi

    # Also read the previous summary
    PREV_SUMMARY="$WORKDIR/iteration-$PREV_ITER/summary.md"
    if [ -f "$PREV_SUMMARY" ]; then
        echo "Reading previous iteration summary..."
        cat "$PREV_SUMMARY"
    fi
fi
1.2 Collect Docker Status

echo "=== Docker Container Status ===" > "$ITER_DIR/status/docker.txt"
docker ps -a --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "vlab-|auslab-" >> "$ITER_DIR/status/docker.txt"

echo "" >> "$ITER_DIR/status/docker.txt"
echo "=== Container Health ===" >> "$ITER_DIR/status/docker.txt"
for c in vlab-backend vlab-monitor vlab-terminal auslab-postgres auslab-redis; do
    health=$(docker inspect --format='{{.State.Health.Status}}' "$c" 2>/dev/null || echo "N/A")
    status=$(docker inspect --format='{{.State.Status}}' "$c" 2>/dev/null || echo "not found")
    restart=$(docker inspect --format='{{.RestartCount}}' "$c" 2>/dev/null || echo "N/A")
    echo "$c: status=$status health=$health restarts=$restart" >> "$ITER_DIR/status/docker.txt"
done
1.3 Collect Container Logs (last 5 minutes)

for c in vlab-backend vlab-monitor vlab-terminal; do
    echo "=== $c logs (last 5 min) ===" > "$ITER_DIR/logs/$c.log"
    docker logs --since 5m --timestamps "$c" >> "$ITER_DIR/logs/$c.log" 2>&1 || echo "Container not running" >> "$ITER_DIR/logs/$c.log"
done
1.4 Check HTTP Endpoints

echo "=== HTTP Health Checks ===" > "$ITER_DIR/status/http.txt"
for endpoint in "8092:monitor" "8093:terminal"; do
    port=$(echo $endpoint | cut -d: -f1)
    name=$(echo $endpoint | cut -d: -f2)

    if curl -sf --connect-timeout 5 "http://localhost:$port/" > /dev/null 2>&1; then
        echo "$name (port $port): OK" >> "$ITER_DIR/status/http.txt"
    else
        echo "$name (port $port): FAILED" >> "$ITER_DIR/status/http.txt"
    fi
done
1.5 Collect Monitor API Data (if available)

if curl -sf http://localhost:8092/health > /dev/null 2>&1; then
    curl -s http://localhost:8092/health > "$ITER_DIR/status/monitor-health.json"
    curl -s http://localhost:8092/status > "$ITER_DIR/status/monitor-status.json"
    curl -s "http://localhost:8092/logs/errors?lines=100" > "$ITER_DIR/status/errors.json"
    # CRITICAL: Get AUSLAB verification results (daemon/login status)
    curl -s http://localhost:8092/api/auslab/verify > "$ITER_DIR/status/verification.json"
fi
1.5.1 Check Verification Results (CRITICAL)

# Extract verification failures - daemon/login status is fundamental to service health
if [ -f "$ITER_DIR/status/verification.json" ]; then
    echo "=== AUSLAB Verification Results ===" > "$ITER_DIR/status/verification-summary.txt"

    # Check for failed verification checks
    VERIFY_FAILED=$(cat "$ITER_DIR/status/verification.json" | grep -o '"failed":[0-9]*' | grep -o '[0-9]*' || echo "0")
    echo "Verification failures: $VERIFY_FAILED" >> "$ITER_DIR/status/verification-summary.txt"

    # Check specifically for disabled daemons/logins (fundamental service health)
    DAEMONS_DISABLED=$(cat "$ITER_DIR/status/verification.json" | grep -i "DISABLED.*DAEMON" | wc -l || echo "0")
    LOGINS_DISABLED=$(cat "$ITER_DIR/status/verification.json" | grep -i "logins.*DISABLED" | wc -l || echo "0")

    echo "Daemons disabled: $DAEMONS_DISABLED" >> "$ITER_DIR/status/verification-summary.txt"
    echo "Logins disabled: $LOGINS_DISABLED" >> "$ITER_DIR/status/verification-summary.txt"

    # Extract individual check results
    echo "" >> "$ITER_DIR/status/verification-summary.txt"
    echo "Check Results:" >> "$ITER_DIR/status/verification-summary.txt"
    cat "$ITER_DIR/status/verification.json" | grep -oE '"name":"[^"]*","status":"[^"]*","result":"[^"]*"' | \
        sed 's/"name":"//g;s/","status":"/|/g;s/","result":"/|/g;s/"//g' >> "$ITER_DIR/status/verification-summary.txt" 2>/dev/null || true
fi
1.6 Check for Internal Restart Loops

echo "=== Internal Restart Loop Detection ===" > "$ITER_DIR/status/restart-loops.txt"

# Terminal process exits
TERMINAL_EXITS=$(docker logs --since 5m vlab-terminal 2>&1 | grep -c "process exited" || echo "0")
echo "vlab-terminal process exits (5m): $TERMINAL_EXITS" >> "$ITER_DIR/status/restart-loops.txt"

# Backend daemon failures
BACKEND_FAILS=$(docker logs --since 5m vlab-backend 2>&1 | grep -c "daemon is no longer running" || echo "0")
echo "vlab-backend daemon failures (5m): $BACKEND_FAILS" >> "$ITER_DIR/status/restart-loops.txt"

# Crash indicators
CRASHES=$(docker logs --since 5m vlab-backend 2>&1 | grep -c -E "SIGABRT|Aborted|core dumped|Segmentation" || echo "0")
echo "Crash indicators (5m): $CRASHES" >> "$ITER_DIR/status/restart-loops.txt"
1.7 Execute ITT Scripts (Interface Testing)

# Only run ITT tests if monitor is accessible
if curl -sf http://localhost:8092/health > /dev/null 2>&1; then
    echo "=== ITT (Interface Testing Tool) Results ===" > "$ITER_DIR/status/itt-summary.txt"

    # Get list of available ITT scripts
    ITT_SCRIPTS=$(curl -s http://localhost:8092/api/itt/scripts 2>/dev/null)
    echo "Available scripts: $ITT_SCRIPTS" >> "$ITER_DIR/status/itt-summary.txt"
    echo "" >> "$ITER_DIR/status/itt-summary.txt"

    # Initialize counters
    ITT_PASSED=0
    ITT_FAILED=0

    # Execute each ITT script
    for script_name in hl7_eorder_test http_api_test monitor_health_check; do
        echo "Running ITT script: $script_name" >> "$ITER_DIR/status/itt-summary.txt"

        # Get script content
        SCRIPT_CONTENT=$(curl -s "http://localhost:8092/api/itt/scripts/$script_name" 2>/dev/null | jq -r '.content // empty')

        if [ -n "$SCRIPT_CONTENT" ]; then
            # Execute the script
            ITT_RESULT=$(curl -s -X POST http://localhost:8092/api/itt/run \
                -H "Content-Type: application/json" \
                -d "{\"script\": $(echo "$SCRIPT_CONTENT" | jq -Rs .)}" 2>/dev/null)

            # Save full result to individual file
            echo "$ITT_RESULT" > "$ITER_DIR/status/itt-$script_name.json"

            # Extract pass/fail status
            PASSED=$(echo "$ITT_RESULT" | jq -r '.passed // false')
            DURATION=$(echo "$ITT_RESULT" | jq -r '.duration // "N/A"')
            ERROR=$(echo "$ITT_RESULT" | jq -r '.error // empty')

            if [ "$PASSED" = "true" ]; then
                echo "  Result: PASSED (duration: $DURATION)" >> "$ITER_DIR/status/itt-summary.txt"
                ITT_PASSED=$((ITT_PASSED + 1))
            else
                echo "  Result: FAILED (duration: $DURATION)" >> "$ITER_DIR/status/itt-summary.txt"
                [ -n "$ERROR" ] && echo "  Error: $ERROR" >> "$ITER_DIR/status/itt-summary.txt"
                ITT_FAILED=$((ITT_FAILED + 1))
            fi
        else
            echo "  Result: SKIPPED (script not found)" >> "$ITER_DIR/status/itt-summary.txt"
        fi
        echo "" >> "$ITER_DIR/status/itt-summary.txt"
    done

    # Summary
    echo "=== ITT Summary ===" >> "$ITER_DIR/status/itt-summary.txt"
    echo "Passed: $ITT_PASSED" >> "$ITER_DIR/status/itt-summary.txt"
    echo "Failed: $ITT_FAILED" >> "$ITER_DIR/status/itt-summary.txt"
    echo "Total: $((ITT_PASSED + ITT_FAILED))" >> "$ITER_DIR/status/itt-summary.txt"
else
    echo "=== ITT Tests Skipped ===" > "$ITER_DIR/status/itt-summary.txt"
    echo "Monitor not accessible - ITT tests require monitor at :8092" >> "$ITER_DIR/status/itt-summary.txt"
fi
STEP 2: REVIEW & PLAN
2.1 Claude reads all status files and creates notes.md with thinking

Create $ITER_DIR/notes.md with:

# Iteration N Notes

**Timestamp:** [ISO timestamp]
**Previous Iteration Result:** [from previous summary or "N/A"]

## Current State Analysis

### Docker Status
[Analysis of docker.txt - which containers running, health, restart counts]

### HTTP Endpoints
[Which endpoints working/failing]

### Log Analysis
[Key errors/warnings found in logs]

### Restart Loop Status
[Any internal restart loops detected]

### AUSLAB Verification (CRITICAL)
[Analysis of verification.json - MUST check daemon and login status]
- Daemon Status: [PASS/FAIL - if DISABLED, this is a critical issue]
- Login Status: [PASS/FAIL - if DISABLED, this is a critical issue]
- Other checks: [list failed checks]

**NOTE:** Disabled daemons/logins are FUNDAMENTAL to service health. If these are failing, the service cannot function properly even if containers appear healthy.

### ITT Test Results
[Analysis of itt-summary.txt and individual itt-*.json files]
- HL7 eOrder Test: [PASSED/FAILED/SKIPPED]
- HTTP API Test: [PASSED/FAILED/SKIPPED]
- Monitor Health Check: [PASSED/FAILED/SKIPPED]
- Summary: [X passed, Y failed]

**NOTE:** ITT failures indicate interface integration issues. Services may appear healthy but fail to communicate correctly.

## Thinking / Reasoning

[Claude documents thinking here:]
- What is the root cause of the primary issue?
- What is the priority order of issues?
- Why was this specific issue selected for this iteration?
- What is the expected fix?
- What could go wrong?

## Selected Issue

| Field | Value |
|-------|-------|
| Priority | [1-CRASH, 2-PORT, 3-CONFIG, 4-DATABASE, 5-APP] |
| Category | [crash/network/config/database/application] |
| Description | [one-line description] |
| Root Cause | [what's causing this] |
| Planned Fix | [what will be done] |

## Action Plan

1. [First action]
2. [Second action]
3. [Verify step]
STEP 3: IMPLEMENT
3.1 Execute the planned actions in order

For each action in the plan:

Before: Document what will be done
Execute: Run the command/edit
After: Document the result
Save: Record in fixes/action-N.md
# Fix Action N

**Timestamp:** [ISO timestamp]
**Action:** [description]

## Before State
[relevant state before change]

## Command/Change Executed
[exact command or edit made]

## Result
[output or confirmation]

## Verification
[how this was verified to work]
3.2 Redeploy if needed

If configuration files were changed:

# Rebuild images if entrypoints changed
docker compose -f /mnt/c/development/magentus/devops/docker/magentus-services/docker-compose.yml build --no-cache [service]

# Restart services
docker compose -f /mnt/c/development/magentus/devops/docker/magentus-services/docker-compose.yml up -d --force-recreate [service]
3.3 Wait for stabilization

echo "Waiting ${STABILIZATION_WAIT}s for services to stabilize..."
sleep $STABILIZATION_WAIT
STEP 4: SUMMARIZE
4.1 Create iteration summary

Create $ITER_DIR/summary.md:

# Iteration N Summary

**Timestamp:** [ISO timestamp]
**Duration:** [time taken]

## Issue Addressed

| Field | Value |
|-------|-------|
| Priority | [priority level] |
| Category | [category] |
| Description | [what was fixed] |

## Actions Taken

| # | Action | Result |
|---|--------|--------|
| 1 | [action] | [SUCCESS/FAILED] |
| 2 | [action] | [SUCCESS/FAILED] |

## Files Modified

| File | Change |
|------|--------|
| [path] | [what changed] |

## Verification Results

| Check | Status |
|-------|--------|
| HTTP :8092 monitor | [OK/FAILED] |
| HTTP :8093 terminal | [OK/FAILED] |
| Internal restart loops | [NONE/DETECTED] |
| ITT: HL7 eOrder Test | [PASSED/FAILED/SKIPPED] |
| ITT: HTTP API Test | [PASSED/FAILED/SKIPPED] |
| ITT: Monitor Health | [PASSED/FAILED/SKIPPED] |

## Outcome

| Field | Value |
|-------|-------|
| Fix Applied | [YES/NO] |
| Fix Successful | [YES/NO/PARTIAL] |
| Issues Remaining | [count] |
| Stability Status | [STABLE/UNSTABLE] |

## Next Iteration Needed?

[YES/NO] - [reason]
4.2 Update workflow state

# Update workflow-state.md with current iteration results
**4.3 Decision: Continue or Stop?

IF all of:
  - Iteration >= MIN_ITERATIONS
  - All HTTP endpoints responding
  - No internal restart loops
  - Stability status = STABLE
  - AUSLAB verification passed (no DISABLED daemons/logins)
  - ITT tests passed (all interface tests successful)
THEN:
  STOP with SUCCESS

ELSE IF Iteration >= MAX_ITERATIONS:
  STOP with INCOMPLETE (document remaining issues)

ELSE:
  Continue to next iteration
NOTE: ITT test failures are acceptable for SUCCESS if they are documented as known limitations (e.g., external system unavailable). Critical interface tests (monitor health check) should pass.

OUTPUT STRUCTURE
$WORKDIR/
|-- workflow-state.md           # Overall workflow state
|-- summary.md                   # Final summary (at end)
|-- iteration-1/
|   |-- status/
|   |   |-- docker.txt          # Docker container status
|   |   |-- http.txt            # HTTP endpoint checks
|   |   |-- restart-loops.txt   # Internal restart detection
|   |   |-- monitor-health.json # Monitor API health
|   |   |-- monitor-status.json # Monitor API status
|   |   |-- errors.json         # Error logs from monitor
|   |   |-- verification.json   # AUSLAB verification results (CRITICAL)
|   |   |-- verification-summary.txt # Parsed verification summary
|   |   |-- itt-summary.txt     # ITT test results summary
|   |   |-- itt-hl7_eorder_test.json    # HL7 eOrder test full results
|   |   |-- itt-http_api_test.json      # HTTP API test full results
|   |   |-- itt-monitor_health_check.json # Monitor health test full results
|   |-- logs/
|   |   |-- vlab-backend.log
|   |   |-- vlab-monitor.log
|   |   |-- vlab-terminal.log
|   |-- fixes/
|   |   |-- action-1.md         # First fix action
|   |   |-- action-2.md         # Second fix action
|   |-- notes.md                 # Claude's thinking/reasoning
|   |-- summary.md               # Iteration outcome summary
|-- iteration-2/
|   |-- ... (same structure)
PRIORITY ORDER
Fix issues in this order (ONE per iteration):

Priority	Category	Indicators
1	CRASH	SIGABRT, core dump, process restart loop
2	VERIFICATION	Daemons DISABLED, Logins DISABLED, verification failures
3	PORT	Connection refused, port not listening
4	CONFIG	Wrong IP binding, port mismatch
5	DATABASE	Connection failed, missing table
6	APPLICATION	PHP error, missing file
7	ITT_FAILURE	Interface test failures (HL7, HTTP API, ASTM)
8	WARNING	Deprecation, non-critical config
CRITICAL: Priority 2 (VERIFICATION) failures mean the AUSLAB service cannot function properly even if containers appear healthy. Daemons must be ENABLED for service operations.

NOTE: Priority 7 (ITT_FAILURE) indicates interface integration issues. The service may appear healthy but fail to communicate with external systems correctly. These should be investigated after core functionality is stable.

COMMON FIX PATTERNS
Port Binding (127.0.0.1 -> 0.0.0.0)
Symptom: Container A cannot connect to Container B (connection refused) Fix: Change 127.0.0.1 to 0.0.0.0 in entrypoint/config

Docker Image Stale
Symptom: Config changes not taking effect Fix: docker compose build --no-cache [service] && docker compose up -d --force-recreate [service]

Port Mismatch
Symptom: Service expects port X, configured for port Y Fix: Align port in docker-compose.yml environment variables

Internal Restart Loop
Symptom: Container "healthy" but service keeps crashing internally Fix: Check entrypoint logs for crash cause, fix root issue

FINAL SUMMARY FORMAT
At workflow end, write $WORKDIR/summary.md:

# VLAB Workflow V2 Summary

**Workdir:** $WORKDIR
**Started:** [timestamp]
**Completed:** [timestamp]
**Final Status:** [SUCCESS/INCOMPLETE/FAILED]

## Results

| Metric | Value |
|--------|-------|
| Iterations Completed | N |
| Minimum Required | 2 |
| Issues Fixed | N |
| Issues Remaining | N |

## Iteration History

| # | Issue | Fix Applied | Outcome |
|---|-------|-------------|---------|
| 1 | [issue] | [fix] | [SUCCESS/FAILED] |
| 2 | [issue] | [fix] | [SUCCESS/FAILED] |

## Services Final Status

| Service | Status | Endpoint |
|---------|--------|----------|
| vlab-backend | [status] | :8082, :8085, :8086, :8094 |
| vlab-monitor | [status] | :8092 |
| vlab-terminal | [status] | :8093 |

## Remaining Issues (if any)
[List or "None"]

## Lessons Learned
[What was discovered during this workflow run]
MANDATORY CHECKLIST
Before each iteration:

[ ] Read previous iteration notes.md (if exists)
[ ] Read previous iteration summary.md (if exists)
During each iteration:

[ ] Create notes.md with thinking/reasoning
[ ] Select ONE priority issue
[ ] Document each action in fixes/action-N.md
[ ] Wait for stabilization after changes
After each iteration:

[ ] Create summary.md with outcomes
[ ] Verify HTTP endpoints
[ ] Check for restart loops
[ ] Execute ITT scripts and verify results
[ ] Decide: continue or stop
At workflow end:

[ ] Write final summary.md
[ ] Document remaining issues (including ITT failures)
[ ] Update workflow-state.md with COMPLETE