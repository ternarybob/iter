# workflow Skill

## Overview

The `workflow` skill executes custom workflow specifications for specialized iterative processes. Unlike `/iter:run` which follows a standard ARCHITECT/WORKER/VALIDATOR pattern, this skill allows you to define custom workflows with specific phases, priorities, and success criteria.

Use this skill when you need:
- Specialized iteration logic
- Custom priority rules
- Multi-iteration stabilization
- Domain-specific workflows (deployment, debugging, optimization)

## Usage

```bash
# From file
/iter:workflow docs/example-workflow.md
/iter:workflow .claude/stabilize-services.md

# Inline spec (for short workflows)
/iter:workflow "# Workflow\nMIN_ITERATIONS: 3\n..."
```

The skill accepts either:
- **File path** - Reads workflow specification from file
- **Inline spec** - Uses argument as workflow content directly

## Workflow Specification Format

A workflow spec is a markdown document defining the iteration structure:

```markdown
# Workflow Name

Brief description of what this workflow does.

## Configuration

- MIN_ITERATIONS: 3
- MAX_ITERATIONS: 10
- STABILIZATION_WAIT: 30s
- WORKING_DIR: .claude/workdir

## Phases

### ARCHITECT
Setup and planning phase.

### WORKER
Execute iterations:
1. GET STATUS - Collect system state
2. REVIEW & PLAN - Analyze and prioritize
3. IMPLEMENT - Apply fixes
4. SUMMARIZE - Document results

### VALIDATOR
Check success criteria after each iteration.

### COMPLETE
Generate final summary.

## Priority Rules

1. **P1: Critical** - Service down, errors blocking users
2. **P2: Warning** - Degraded performance, non-critical errors
3. **P3: Optimization** - Performance improvements, cleanup

## Success Criteria

- All services responding HTTP 200
- No ERROR level logs in last 5 minutes
- Response time < 500ms p95
- Test suite passing

## Data Collection

### Status Files
- service-status.txt - Health check results
- error-logs.txt - Error messages
- metrics.txt - Performance metrics

### Logs
- Container logs from last 10 minutes
- Application logs filtered for errors
```

## How It Works

### Phase 1: ARCHITECT - Setup

1. **Parse workflow spec** - Read configuration, phases, rules
2. **Create working directory**:
   ```
   .claude/workdir/{workflow-name}-{timestamp}/
   ```
3. **Document architecture** in `architecture.md`:
   - Workflow purpose and scope
   - System/service architecture
   - Iteration structure
   - Priority rules
   - Success criteria
4. **Create iteration tasks** with dependencies:
   ```javascript
   Task 1: "Iteration 1" (no dependencies)
   Task 2: "Iteration 2" (blockedBy: Task 1)
   Task 3: "Iteration 3" (blockedBy: Task 2)
   ...
   ```
5. **Initialize workflow state** in `workflow-state.md`

### Phase 2: WORKER - Execute Iterations

For each iteration (mark task `in_progress`):

#### Step 1: GET STATUS

Collect all status information:
- Service health checks
- Log files (errors, warnings)
- Metrics (performance, resources)
- Test results
- Previous iteration notes (if > Iteration 1)

Save to:
```
iteration-N/
  status/
    service-status.txt
    health-checks.txt
    ...
  logs/
    errors.log
    warnings.log
    ...
```

#### Step 2: REVIEW & PLAN

Create `iteration-N/notes.md`:

```markdown
# Iteration N Notes

## Current State Analysis

[Review all status files]

## Issues Detected

1. API service returning 500 errors
2. Database connections timing out
3. Cache hit rate low (45%)

## Priority Assessment

Using workflow priority rules:
- Issue 1: P1 Critical (service errors)
- Issue 2: P1 Critical (blocking requests)
- Issue 3: P3 Optimization

## Selected Issue

**Issue 1: API service returning 500 errors**
(Highest priority: P1 Critical)

## Action Plan

1. Check API service logs for error details
2. Identify root cause
3. Apply fix (restart/config/code change)
4. Verify service healthy
```

**CRITICAL**: Select ONE highest-priority issue per iteration.

#### Step 3: IMPLEMENT

Execute the action plan:

For each action, create `iteration-N/fixes/action-M.md`:

```markdown
# Action M: Check API logs

## Before State
API service returning 500 errors

## Command/Change
`docker logs api-service --since 10m`

## Output
Error: Database connection pool exhausted

## Verification
Identified root cause: connection pool too small

## Next Action
Increase connection pool size in config
```

Apply fixes using Edit/Write/Bash tools.

If changes require restart/redeploy, execute as specified in workflow.

Wait for stabilization (STABILIZATION_WAIT seconds).

#### Step 4: SUMMARIZE

Create `iteration-N/summary.md`:

```markdown
# Iteration N Summary

## Issue Addressed
API service returning 500 errors

## Root Cause
Database connection pool exhausted (max: 10 connections)

## Actions Taken
1. Checked API service logs
2. Identified connection pool limitation
3. Increased pool size to 50 connections
4. Restarted API service
5. Verified service healthy

## Files Modified
- config/database.yml

## Verification Results
✓ API service responding 200 OK
✓ No errors in last 5 minutes
✓ Response time improved (300ms → 150ms p95)

## Outcome
Fix successful. Service stable.

## Remaining Issues
- Issue 3: Cache hit rate still low (45%)

## Decision
CONTINUE - Issues remain, iteration < MAX_ITERATIONS
```

Update `workflow-state.md` with iteration results.

Mark task `completed`.

### Phase 3: VALIDATOR - Check Success

Review iteration summary against workflow success criteria:

**Continue to next iteration IF**:
- Iteration < MIN_ITERATIONS, OR
- Issues remain AND Iteration < MAX_ITERATIONS

**Stop with SUCCESS IF**:
- Iteration >= MIN_ITERATIONS, AND
- All success criteria met

**Stop with INCOMPLETE IF**:
- Iteration >= MAX_ITERATIONS, AND
- Issues still remain

### Phase 4: COMPLETE - Final Summary

Create `summary.md`:

```markdown
# Workflow Summary: {Workflow Name}

**Started**: 2026-01-26 14:30:00
**Completed**: 2026-01-26 15:45:00
**Duration**: 1h 15m
**Workdir**: .claude/workdir/stabilize-services-20260126-1430

## Final Status
SUCCESS

## Metrics
- Iterations completed: 5
- Issues fixed: 3
- Issues remaining: 0
- Success rate: 100%

## Iteration History

| Iteration | Issue | Priority | Outcome |
|-----------|-------|----------|---------|
| 1 | API errors | P1 | Fixed - connection pool |
| 2 | DB timeouts | P1 | Fixed - query optimization |
| 3 | Cache hit rate | P3 | Fixed - cache config |
| 4 | Verification | - | Confirmed stability |
| 5 | Final check | - | All criteria met |

## Final System Status
✓ All services responding 200 OK
✓ No errors in logs
✓ Response time < 500ms p95
✓ Tests passing

## Changes Made
1. config/database.yml - Increased connection pool
2. src/db/queries.go - Optimized slow queries
3. config/cache.yml - Improved cache settings

## Lessons Learned
- Connection pool was undersized for production load
- Query N+1 problem in user endpoint
- Cache TTL too short for static data

## Recommendations
- Monitor connection pool usage
- Add query performance tests
- Review cache strategy for other endpoints
```

## Task Management

The workflow skill uses task management for ordered execution:

### Task Creation

```javascript
// ARCHITECT phase creates all iteration tasks
TaskCreate({ subject: "Iteration 1", ... })
TaskCreate({ subject: "Iteration 2", blockedBy: ["task-1-id"] })
TaskCreate({ subject: "Iteration 3", blockedBy: ["task-2-id"] })
...
```

### Task Updates

```javascript
// Before starting iteration
TaskUpdate({ taskId: "2", status: "in_progress" })

// After completing iteration
TaskUpdate({ taskId: "2", status: "completed" })
```

### Progress Tracking

```javascript
TaskList() // See all iterations and their status
```

## Output Structure

Created in `.claude/workdir/{workflow-name}-{timestamp}/`:

```
architecture.md              # System architecture and workflow overview
workflow-state.md           # Current state, metrics, progress
iteration-1/
  status/                   # Status files (services, logs, metrics)
  logs/                     # Log files collected
  notes.md                  # Analysis, priority assessment, action plan
  fixes/
    action-1.md             # Action documentation
    action-2.md
    ...
  summary.md                # Iteration outcome
iteration-2/
  ...
iteration-N/
  ...
summary.md                  # Final workflow summary
```

## Priority Rules

Workflows define rules for issue prioritization:

### Example Priority Structure

```markdown
## Priority Rules

1. **P1: Critical**
   - Service down or unreachable
   - Errors blocking user actions
   - Data corruption or loss
   - Security vulnerabilities

2. **P2: Warning**
   - Degraded performance
   - Non-critical errors
   - Resource warnings
   - Intermittent issues

3. **P3: Optimization**
   - Performance improvements
   - Code cleanup
   - Configuration tuning
   - Monitoring enhancements
```

### Selection Logic

Each iteration:
1. **Detect all issues** from status and logs
2. **Assign priorities** using workflow rules
3. **Select highest priority** issue
4. **Fix that ONE issue** completely
5. **Verify fix** before next iteration

## Examples

### Example 1: Service Stabilization Workflow

**Workflow Spec** (`docs/stabilize-services.md`):
```markdown
# Service Stabilization Workflow

Stabilize production services after deployment.

## Configuration
- MIN_ITERATIONS: 3
- MAX_ITERATIONS: 10
- STABILIZATION_WAIT: 60s

## Priority Rules
1. P1: Service errors
2. P2: Performance degradation
3. P3: Warnings

## Success Criteria
- All services HTTP 200
- No errors in last 10 minutes
- p95 response time < 500ms
```

**Execution**:
```bash
User: /iter:workflow docs/stabilize-services.md

Claude: [ARCHITECT]
        Initializing workflow...
        Created: .claude/workdir/stabilize-services-20260126-1430/

        [ITERATION 1]
        GET STATUS: Checking services...
        ✗ API service returning 500 errors

        REVIEW & PLAN: Selected P1 issue
        IMPLEMENT: Fixed connection pool config
        SUMMARIZE: Service stable

        [ITERATION 2]
        GET STATUS: All services healthy
        REVIEW & PLAN: No critical issues
        Selected P3: Cache optimization

        ...

        [COMPLETE]
        SUCCESS - All services stable after 3 iterations
```

### Example 2: Performance Optimization Workflow

**Workflow Spec** (`docs/optimize-performance.md`):
```markdown
# Performance Optimization Workflow

Improve system performance iteratively.

## Configuration
- MIN_ITERATIONS: 5
- MAX_ITERATIONS: 15

## Priority Rules
1. P1: Response time > 1000ms
2. P2: Response time > 500ms
3. P3: Response time > 200ms

## Success Criteria
- p50 < 100ms
- p95 < 200ms
- p99 < 500ms
```

**Execution**:
```bash
User: /iter:workflow docs/optimize-performance.md

Claude: [ARCHITECT]
        Setting up performance optimization workflow...

        [ITERATION 1]
        Current p95: 850ms (P1 priority)
        Fix: Add database connection pooling
        Result: p95 reduced to 520ms

        [ITERATION 2]
        Current p95: 520ms (P2 priority)
        Fix: Optimize slow database queries
        Result: p95 reduced to 280ms

        ...

        [COMPLETE]
        SUCCESS - Performance targets met
        p50: 85ms, p95: 180ms, p99: 420ms
```

### Example 3: Debugging Workflow

**Workflow Spec** (`docs/debug-errors.md`):
```markdown
# Error Debugging Workflow

Systematically debug and fix errors in production.

## Configuration
- MIN_ITERATIONS: 2
- MAX_ITERATIONS: 8

## Priority Rules
1. P1: Errors affecting > 10% of requests
2. P2: Errors affecting > 1% of requests
3. P3: Rare errors (< 1%)

## Data Collection
- Error logs from last hour
- Stack traces
- Request patterns
- User impact metrics

## Success Criteria
- Error rate < 0.1%
- No P1 or P2 errors remaining
```

## Execution Rules

1. **AUTONOMOUS EXECUTION** - Execute all commands directly
2. **ONE ISSUE PER ITERATION** - Fix highest priority issue only
3. **DOCUMENT THINKING** - Always explain reasoning in notes.md
4. **READ PREVIOUS NOTES** - Review before each iteration
5. **FOLLOW PRIORITIES** - Use workflow priority rules strictly
6. **VERIFY CHANGES** - Always verify fixes work before proceeding

## Related Skills

- **/iter:run** - Standard iterative implementation (general purpose)
- **/iter:test** - Test-driven iteration with auto-fix
- **/iter:install** - Install `/iter` shortcut wrapper

## Tips

### Writing Good Workflow Specs

**Include**:
- Clear configuration parameters
- Well-defined priority rules
- Measurable success criteria
- Specific data collection requirements
- Verification procedures

**Example**:
```markdown
## Success Criteria
✓ All services HTTP 200 (measurable)
✓ p95 < 500ms (specific threshold)
✓ No ERROR logs in last 10min (verifiable)
```

**Avoid**:
- Vague criteria ("services should work well")
- Unmeasurable goals ("improve performance")
- Missing priorities ("fix all issues")

### Iteration Count

- **MIN_ITERATIONS**: Ensures thorough verification even if criteria met early
- **MAX_ITERATIONS**: Prevents infinite loops

Typical values:
- Quick fixes: MIN=2, MAX=5
- Stabilization: MIN=3, MAX=10
- Deep optimization: MIN=5, MAX=20

### Stabilization Wait

After applying fixes, wait for:
- Services to restart
- Metrics to stabilize
- Caches to warm up
- Logs to reflect changes

Typical values:
- Quick changes: 10-30 seconds
- Service restarts: 30-60 seconds
- System-wide changes: 60-120 seconds

## Troubleshooting

### Workflow Not Progressing

Check `workflow-state.md` for:
- Current iteration number
- Issues detected
- Priority assessment
- Last action taken

### Issues Not Being Fixed

Verify:
- Priority rules are correct
- Data collection is working
- Status files have current data
- Actions are being executed

### Max Iterations Reached

Review `summary.md`:
- What was fixed
- What remains
- Why fixes didn't work
- Adjust workflow spec and retry

## Technical Notes

- Workflow state persisted in markdown files
- All artifacts preserved for debugging
- Task management ensures ordered iterations
- Safe to interrupt (state is saved)
- Each iteration is atomic (completes or fails)
- No parallel iterations (sequential only)

## Philosophy

workflow embodies:

1. **Systematic approach** - One issue at a time, highest priority first
2. **Documentation-driven** - Every decision and action documented
3. **Verification-focused** - Check success criteria after each iteration
4. **Autonomous execution** - Minimal user intervention required
5. **Adaptable** - Custom workflows for specific needs

This produces reliable, repeatable processes for complex iterative tasks.
