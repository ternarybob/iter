# run Skill

## Overview

The `run` skill is the core of the iter plugin. It implements adversarial iterative development - a structured workflow that iterates until requirements and tests pass.

Use this skill when you want Claude to:
- Implement new features
- Fix bugs
- Refactor code
- Make any code changes with high quality standards

The workflow uses three phases (ARCHITECT, WORKER, VALIDATOR) with task management to ensure changes are correct, complete, and meet all requirements.

## Usage

```bash
/iter:run "<task description>"

# Examples
/iter:run "add health check endpoint at /health"
/iter:run "fix authentication bug in login flow"
/iter:run "refactor user service to use repository pattern"

# Show version
/iter:run -v
```

**Shortcut**: If you've installed the wrapper with `/iter:install`, you can use:
```bash
/iter "<task description>"
```

## Workflow Phases

The run skill executes a three-phase loop that continues until all requirements are met:

### Phase 1: ARCHITECT

**Purpose**: Analyze and plan the implementation

**Activities**:
1. **Analyze codebase** - Examine existing patterns, conventions, architecture
2. **Extract requirements** - List ALL explicit and implicit requirements
3. **Identify cleanup** - Find dead code, redundant patterns, technical debt
4. **Create step plan** - Break work into discrete, verifiable steps

**Outputs** (in `.iter/workdir/{session-id}/`):
- `requirements.md` - All requirements with unique IDs (R1, R2, R3...)
- `architect-analysis.md` - Patterns found, architectural decisions, risks
- `step_1.md`, `step_2.md`, ... - Detailed specifications for each step

**Task Management**:
- Creates tasks for each step
- Sets dependencies (addBlockedBy) to ensure ordered execution
- Step 2 can't start until Step 1 completes

### Phase 2: WORKER

**Purpose**: Implement the current step exactly as specified

**Activities**:
1. **Read step specification** - Understand requirements and approach
2. **Make code changes** - Edit files using Edit/Write tools
3. **Verify build passes** - Run `go build ./...` (or appropriate command)
4. **Run tests** - Execute relevant test suites
5. **Mark task in progress** - Use TaskUpdate to set status

**Rules**:
- CORRECTNESS over SPEED - Never rush
- Requirements are LAW - No interpretation or deviation
- Existing patterns are LAW - Match codebase style exactly
- Build MUST pass - Verify after every change
- Cleanup is MANDATORY - Remove dead code
- Tests MUST pass - All existing tests plus new ones

**Documentation**:
- Can optionally create `step_N_impl.md` for implementation notes
- Documents changes made and decisions

### Phase 3: VALIDATOR

**Purpose**: Review with adversarial stance (DEFAULT: REJECT)

**Activities**:
1. **Check build** - Must compile successfully
2. **Check tests** - Must all pass
3. **Verify requirements** - Each requirement traced to implementation
4. **Check cleanup** - No dead code or artifacts remaining
5. **Review acceptance criteria** - All criteria from step spec met

**Validation Logic**:
- **AUTO-REJECT** if:
  - Build fails
  - Tests fail
  - Requirements not traced
  - Dead code remains
  - Acceptance criteria not met

- **PASS** only if:
  - ALL checks verified
  - Build passes
  - Tests pass
  - Cleanup complete
  - Requirements fully met

**On REJECT**:
- Task stays `in_progress`
- Returns to WORKER phase
- Provides rejection reasons
- WORKER fixes issues and resubmits

**On PASS**:
- Marks task `completed`
- Proceeds to next step
- Or marks session COMPLETE if all steps done

### The Loop

```
ARCHITECT → Create step plan
    ↓
WORKER → Implement Step 1
    ↓
VALIDATOR → Review Step 1
    ↓
    ├─ REJECT → Back to WORKER (fix issues)
    └─ PASS → Next step or COMPLETE
```

This continues until:
- ✅ All steps pass validation (SUCCESS)
- ⚠️ Max iterations reached (50, INCOMPLETE)
- 🛑 User runs `iter reset` (ABORTED)

## Task Management

The run skill uses Claude's task management for ordered execution:

### Task Creation
```javascript
TaskCreate({
  subject: "Implement Step 1: Add health check endpoint",
  description: "Create /health endpoint returning service status",
  activeForm: "Implementing health check endpoint"
})
```

### Task Dependencies
```javascript
// Step 2 can't start until Step 1 completes
TaskCreate({
  subject: "Implement Step 2: Add tests",
  metadata: { blockedBy: ["step-1-task-id"] }
})
```

### Task Status Flow
```
pending → in_progress → completed
          ↑          ↓
          └─ REJECT ─┘
```

### Checking Progress
```javascript
TaskList() // See all tasks and their status
TaskGet("task-id") // Get specific task details
```

## Session Artifacts

All session data is stored in `.iter/workdir/{session-id}/`

| File | Purpose |
|------|---------|
| `requirements.md` | All requirements with IDs (R1, R2, R3...) |
| `architect-analysis.md` | Codebase patterns, architectural decisions, risks |
| `step_1.md` | Step 1 specification |
| `step_2.md` | Step 2 specification |
| `step_N.md` | Step N specification |
| `step_N_impl.md` | Implementation notes (optional) |
| `summary.md` | Completion summary (created when done) |

### Requirement Format (requirements.md)

```markdown
## R1: Add health check endpoint [MUST]
**Priority**: MUST
**Source**: User requirement
**Description**: Create HTTP endpoint at /health that returns service status

**Acceptance Criteria**:
- Endpoint responds to GET /health
- Returns 200 OK when healthy
- Returns JSON with status field
- Includes timestamp in response
```

### Step Format (step_N.md)

```markdown
# Step 1: Create Health Check Endpoint

## Requirements
- R1: Add health check endpoint

## Objective
Create HTTP endpoint at /health returning service status

## Implementation
[Detailed approach, files to modify, code structure]

## Acceptance Criteria
✅ Endpoint exists at /health
✅ Returns 200 OK
✅ Returns JSON response
✅ Build passes
✅ Tests pass
```

## Git Worktree Isolation

The run skill creates an isolated workspace for safe experimentation:

1. **Creates branch**: `iter/{task-slug}-{timestamp}`
2. **Creates worktree**: `.iter/worktrees/iter/{task-slug}-{timestamp}`
3. **All changes in worktree**: Main branch unaffected
4. **On completion**: Changes merged back to main
5. **On abort**: Worktree deleted, main unchanged

This allows you to:
- Continue working in main branch
- Safely experiment in iter worktree
- Abandon changes if needed
- Multiple iter sessions can run (different worktrees)

## Execution Rules

These rules are enforced by the VALIDATOR:

1. **CORRECTNESS OVER SPEED** - Never rush, quality is mandatory
2. **REQUIREMENTS ARE LAW** - No interpretation, no deviation, no "improvements"
3. **EXISTING PATTERNS ARE LAW** - Match codebase style exactly
4. **BUILD MUST PASS** - Verify after every change
5. **CLEANUP IS MANDATORY** - Remove dead code, no orphaned artifacts
6. **TESTS MUST PASS** - All existing tests plus new tests

### Auto-Reject Conditions
- Build fails
- Tests fail
- Lint errors
- Missing requirement traceability
- Dead code not cleaned up
- Acceptance criteria not met

### Pass Conditions
- ALL checklist items verified
- Build passes
- Tests pass
- Requirements traced
- Cleanup complete

## Examples

### Example 1: Add New Feature

```bash
User: /iter:run "add health check endpoint at /health"

Claude: [ARCHITECT]
        Creating implementation plan...

        Requirements:
        - R1: Add health check endpoint [MUST]
        - R2: Return JSON response [MUST]
        - R3: Include timestamp [SHOULD]
        - R4: Add tests [MUST]

        Steps:
        1. Create health check handler
        2. Register route
        3. Add tests

        [WORKER - Step 1]
        Implementing health check handler...

        Created: src/handlers/health.go
        Modified: src/routes.go

        [VALIDATOR]
        ✓ Build passes
        ✓ Tests pass
        ✓ Requirements met
        ✓ No dead code

        Step 1 PASSED

        [WORKER - Step 2]
        ...

        [COMPLETE]
        All steps completed successfully!
```

### Example 2: Fix Bug

```bash
User: /iter:run "fix authentication bug - users can't log in with special characters in password"

Claude: [ARCHITECT]
        Analyzing authentication flow...

        Found issue: Password encoding not URL-safe

        Requirements:
        - R1: Fix password encoding [MUST]
        - R2: Handle special characters [MUST]
        - R3: Add test cases [MUST]
        - R4: Maintain backward compatibility [MUST]

        [WORKER]
        Implementing fix...

        Modified: src/auth/encoding.go
        Added test: tests/auth_special_chars_test.go

        [VALIDATOR]
        ✓ Build passes
        ✓ Tests pass (including new special char tests)
        ✓ Backward compatible

        [COMPLETE]
        Bug fixed! Users can now log in with special characters.
```

### Example 3: Refactor

```bash
User: /iter:run "refactor user service to use repository pattern"

Claude: [ARCHITECT]
        Current pattern: Direct database access in service
        Target pattern: Repository layer

        Steps:
        1. Create user repository interface
        2. Implement repository
        3. Update service to use repository
        4. Update tests
        5. Remove direct DB calls

        [WORKER - Step 1]
        Creating repository interface...

        [VALIDATOR]
        ✗ REJECTED
        Reason: Repository interface missing error types

        [WORKER - Step 1 retry]
        Adding error types...

        [VALIDATOR]
        ✓ PASSED

        [WORKER - Step 2]
        ...

        [COMPLETE]
        Refactoring complete! All services now use repository pattern.
```

## Version Check

```bash
User: /iter:run -v

Claude: iter version 2.1.20260126-1724
```

## Exit Conditions

### Success
All steps pass validation, requirements met, session COMPLETE.

### Incomplete
- Max iterations reached (default: 50)
- Session still in progress but needs manual intervention
- Use `iter reset` to abort or continue manually

### Aborted
- User runs `iter reset` to stop
- Worktree cleaned up
- Changes not merged

## State Management

Session state tracked in `.iter/state.json`:

```json
{
  "task": "add health check endpoint",
  "phase": "worker",
  "current_step": 2,
  "total_steps": 3,
  "iteration": 5,
  "max_iterations": 50,
  "validation_pass": 1,
  "rejections": 3,
  "completed": false
}
```

## Related Skills

- **/iter:install** - Install `/iter` shortcut wrapper
- **/iter:workflow** - Custom workflow-based implementation
- **/iter:test** - Test-driven iteration with auto-fix
- **/iter:index** - Build code index for navigation
- **/iter:search** - Search indexed codebase

## Binary Commands

The run skill uses the iter binary, which has these commands:

```bash
iter run "<task>"           # Start iterative implementation
iter status                 # Show session status
iter step [N]               # Show step instructions
iter pass                   # Record validation pass
iter reject "<reason>"      # Record validation rejection
iter next                   # Move to next step
iter complete               # Mark session complete
iter reset                  # Reset session
```

Claude uses these commands automatically during the workflow.

## Tips

### Write Clear Task Descriptions

**Good**:
```bash
/iter:run "add health check endpoint at /health returning JSON with status and timestamp"
```

**Too vague**:
```bash
/iter:run "add health check"
```

### Let Claude Ask Questions

If requirements are ambiguous, Claude may ask clarifying questions before starting.

### Trust the Process

The VALIDATOR is adversarial by design:
- DEFAULT stance is REJECT
- Finds problems proactively
- Ensures high quality
- Multiple rejections are normal and healthy

### Monitor Progress

Use Claude's task list to see progress:
- Which steps are completed
- Which step is in progress
- What's left to do

### Be Patient

Quality takes time:
- ARCHITECT phase can take several minutes for complex tasks
- WORKER implements carefully, not quickly
- VALIDATOR checks thoroughly
- Rejections and retries are part of the process

## Troubleshooting

### Session Stuck

If session seems stuck, check:
```bash
# View current session state
# Claude can read: .iter/state.json

# View current step
# Claude can read: .iter/workdir/{session-id}/step_N.md
```

### Want to Abort

```bash
# Stop and clean up
iter reset

# Or ask Claude to run it
"Please run iter reset to abort this session"
```

### Multiple Sessions

Only one session can be active at a time. Complete or reset current session before starting new one.

## Technical Notes

- Uses Git worktrees for isolation
- Task management ensures ordered execution
- Adversarial validation prevents low-quality output
- All artifacts preserved for debugging
- Safe to interrupt (state is persisted)
- Can resume after interruption (state in `.iter/state.json`)

## Philosophy

The run skill embodies these principles:

1. **Correctness over speed** - Take time to do it right
2. **Requirements are sacred** - No interpretation or deviation
3. **Patterns matter** - Match existing codebase style
4. **Validation is adversarial** - Default to REJECT, earn the PASS
5. **Cleanup is mandatory** - Leave code better than you found it
6. **Tests are required** - Verify everything works

This produces high-quality, maintainable code that fits seamlessly into your codebase.
