// Package main provides the CLI entry point for the iter Claude Code plugin.
//
// iter is an adversarial multi-agent DevOps loop that runs within Claude Code.
// It implements a structured iterative approach where work is planned, executed,
// and validated until requirements/tests are achieved.
//
// Usage:
//
//	iter run "<task>"                  - Start iterative implementation
//	iter workflow "<spec>"             - Start workflow-based implementation
//	iter status                        - Show current session status
//	iter pass                          - Record validation pass
//	iter reject "<reason>"             - Record validation rejection
//	iter next                          - Move to next step
//	iter complete                      - Mark session complete
//	iter reset                         - Reset session state
//	iter hook-stop                     - Stop hook handler (JSON output)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ternarybob/iter/index"
)

const (
	stateDir  = ".iter"
	stateFile = "state.json"
	version   = "2.0.0"
)

// Embedded prompts - all prompt content lives here in the binary
var prompts = struct {
	SystemRules     string
	ArchitectRole   string
	WorkerRole      string
	ValidatorRole   string
	WorkflowSystem  string
	ValidationRules string
}{
	SystemRules: `## EXECUTION RULES

These rules are NON-NEGOTIABLE:

1. **CORRECTNESS OVER SPEED** - Never rush. Quality is mandatory.
2. **REQUIREMENTS ARE LAW** - No interpretation, no deviation, no "improvements"
3. **EXISTING PATTERNS ARE LAW** - Match codebase style exactly
4. **BUILD MUST PASS** - Verify after every change
5. **CLEANUP IS MANDATORY** - Remove dead code, no orphaned artifacts
6. **TESTS MUST PASS** - All existing tests, plus new tests for new code`,

	ArchitectRole: `## ARCHITECT PHASE

You are analyzing requirements and creating an implementation plan.

### Instructions
1. **Analyze Codebase First**: Examine existing patterns, conventions, architecture
2. **Extract Requirements**: List ALL explicit and implicit requirements
3. **Identify Cleanup**: Find dead code, redundant patterns, technical debt
4. **Create Step Plan**: Break work into discrete, verifiable steps

### Output Files (create in .iter/workdir/)

**requirements.md**
- Unique IDs: R1, R2, R3...
- Priority: MUST | SHOULD | MAY
- Include implicit requirements (error handling, tests, docs)

**step_N.md** (one per step)
- Title: Brief description
- Requirements: Which R-IDs this addresses
- Approach: Detailed implementation plan
- Cleanup: Dead code to remove
- Acceptance: How to verify completion

**architect-analysis.md**
- Patterns found in codebase
- Architectural decisions made
- Risk assessment`,

	WorkerRole: `## WORKER PHASE

You are implementing the current step exactly as specified.

### CRITICAL RULES
- Follow step document EXACTLY - no interpretation
- Make ONLY the changes specified
- Verify build passes after each change
- Perform ALL cleanup listed in step doc
- Document what you did in step_N_impl.md

### Workflow
1. Read .iter/workdir/step_N.md
2. Implement exactly as specified
3. Run build verification
4. Create step_N_impl.md with summary
5. Request validation`,

	ValidatorRole: `## VALIDATOR PHASE

**DEFAULT STANCE: REJECT**

Your job is to find problems. Assume implementation is wrong until proven correct.

### Validation Checklist

**1. BUILD VERIFICATION** (AUTO-REJECT if fails)
- Run build command
- Run tests
- Run linter

**2. REQUIREMENTS TRACEABILITY** (AUTO-REJECT if missing)
- Every change must trace to a requirement
- Every requirement for this step must be implemented

**3. CODE QUALITY**
- Changes match existing patterns
- Error handling is complete
- No debug code left behind
- Proper logging (not print statements)

**4. CLEANUP VERIFICATION**
- All cleanup items addressed
- No new dead code introduced
- No orphaned files

**5. STEP COMPLETION**
- All acceptance criteria met
- Changes are minimal and focused
- No scope creep

### Verdict
After review, call:
- 'iter pass' - ALL checks pass
- 'iter reject "specific reason"' - ANY check fails`,

	WorkflowSystem: `## WORKFLOW MODE

You are executing a custom workflow specification.

The workflow defines:
- Phases/stages to execute
- Success criteria for each phase
- Iteration rules

Parse the workflow spec and execute each phase in order.
Track progress and iterate until all success criteria are met.`,

	ValidationRules: `### Auto-Reject Conditions
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
- Cleanup complete`,
}

// State represents the persistent state of an iter session.
type State struct {
	Task           string    `json:"task"`
	Mode           string    `json:"mode"` // "iter" or "workflow"
	WorkflowSpec   string    `json:"workflow_spec,omitempty"`
	Workdir        string    `json:"workdir"`
	MaxIterations  int       `json:"max_iterations"`
	Iteration      int       `json:"iteration"`
	Phase          string    `json:"phase"` // architect, worker, validator, complete
	CurrentStep    int       `json:"current_step"`
	TotalSteps     int       `json:"total_steps"`
	StartedAt      time.Time `json:"started_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
	ValidationPass int       `json:"validation_pass"`
	Rejections     int       `json:"rejections"`
	ExitSignal     bool      `json:"exit_signal"`
	Completed      bool      `json:"completed"`
	Artifacts      []string  `json:"artifacts"`
	Verdicts       []Verdict `json:"verdicts"`
}

// Verdict represents a validator verdict.
type Verdict struct {
	Step      int       `json:"step"`
	Pass      int       `json:"pass"`
	Status    string    `json:"status"` // pass, reject
	Reasons   []string  `json:"reasons"`
	Timestamp time.Time `json:"timestamp"`
}

// HookResponse is the JSON output format for Claude Code hooks.
type HookResponse struct {
	Continue           bool                `json:"continue"`
	SuppressOutput     bool                `json:"suppressOutput,omitempty"`
	SystemMessage      string              `json:"systemMessage,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput provides hook-specific control.
type HookSpecificOutput struct {
	HookEventName      string         `json:"hookEventName,omitempty"`
	PermissionDecision string         `json:"permissionDecision,omitempty"`
	AdditionalContext  string         `json:"additionalContext,omitempty"`
	UpdatedInput       map[string]any `json:"updatedInput,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "run":
		err = cmdRun(args)
	case "workflow":
		err = cmdWorkflow(args)
	case "status":
		err = cmdStatus(args)
	case "step":
		err = cmdStep(args)
	case "pass":
		err = cmdPass(args)
	case "reject":
		err = cmdReject(args)
	case "next":
		err = cmdNext(args)
	case "complete":
		err = cmdComplete(args)
	case "reset":
		err = cmdReset(args)
	case "hook-stop":
		err = cmdHookStop(args)
	case "index":
		err = cmdIndex(args)
	case "search":
		err = cmdSearch(args)
	case "version", "-v", "--version":
		cmdVersion()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`iter - Adversarial iterative implementation for Claude Code

Commands:
  run "<task>"           Start iterative implementation until requirements/tests pass
  workflow "<spec>"      Start workflow-based implementation
  status                 Show current session status
  step [N]               Show current/specific step instructions
  pass                   Record validation pass
  reject "<reason>"      Record validation rejection
  next                   Move to next step
  complete               Mark session complete
  reset                  Reset session state
  hook-stop              Stop hook handler (JSON output)
  version                Show version
  help                   Show this help

Code Index Commands:
  index                  Show index status
  index build            Build/rebuild the full code index
  index clear            Clear and rebuild the index
  index watch            Start file watcher for real-time indexing
  search "<query>"       Search the code index
    --kind=<type>        Filter by symbol type (function, method, type, const)
    --path=<prefix>      Filter by file path prefix
    --branch=<branch>    Filter by git branch
    --limit=<n>          Maximum results (default 10)

The iter loop:
  1. ARCHITECT: Analyze requirements, create step documents
  2. WORKER: Implement each step exactly as specified
  3. VALIDATOR: Review with adversarial stance (default REJECT)
  4. Loop until all steps pass or max iterations reached`)
}

func cmdVersion() {
	fmt.Printf("iter version %s\n", version)
}

// summarizeTask creates a short slug from a task description.
// It extracts the first few meaningful words, converts to lowercase,
// and uses hyphens as separators.
func summarizeTask(task string) string {
	// Convert to lowercase
	s := strings.ToLower(task)

	// Replace newlines and multiple spaces with single space
	s = regexp.MustCompile(`[\r\n]+`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	// Extract words (alphanumeric only)
	words := regexp.MustCompile(`[a-z0-9]+`).FindAllString(s, -1)

	// Take first 5 words max
	if len(words) > 5 {
		words = words[:5]
	}

	// Join with hyphens
	slug := strings.Join(words, "-")

	// Truncate to 40 chars max
	if len(slug) > 40 {
		slug = slug[:40]
		// Don't end with a hyphen
		slug = strings.TrimSuffix(slug, "-")
	}

	// Fallback if empty
	if slug == "" {
		slug = "task"
	}

	return slug
}

// generateWorkdirPath creates a unique workdir path from a task.
func generateWorkdirPath(task string) string {
	slug := summarizeTask(task)
	timestamp := time.Now().Format("20060102-03-04")
	return filepath.Join(stateDir, "workdir", fmt.Sprintf("%s-%s", slug, timestamp))
}

// cmdRun starts an iterative implementation session.
func cmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter run \"<task>\" [--max-iterations N]")
	}

	task := args[0]
	maxIterations := 50

	for i := 1; i < len(args); i++ {
		if args[i] == "--max-iterations" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid max-iterations: %w", err)
			}
			maxIterations = n
			i++
		}
	}

	// Generate unique workdir path
	workdir := generateWorkdirPath(task)

	// Initialize state
	state := &State{
		Task:           task,
		Mode:           "iter",
		Workdir:        workdir,
		MaxIterations:  maxIterations,
		Iteration:      1,
		Phase:          "architect",
		CurrentStep:    1,
		TotalSteps:     0,
		StartedAt:      time.Now(),
		LastActivityAt: time.Now(),
		Artifacts:      []string{},
		Verdicts:       []Verdict{},
	}

	// Create workdir
	if err := os.MkdirAll(workdir, 0755); err != nil {
		return fmt.Errorf("failed to create workdir: %w", err)
	}

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Output the full iterative implementation prompt
	fmt.Printf(`# ITERATIVE IMPLEMENTATION

## Task
%s

%s

%s

---

## PHASE: ARCHITECT

Begin by analyzing the codebase and creating your implementation plan.

%s

---

## Session Info
- Iteration: %d/%d
- State: .iter/state.json
- Artifacts: %s/

## Next Steps
1. Create requirements.md, step_N.md files, architect-analysis.md
2. Set total steps: Update .iter/state.json "total_steps" field
3. Then implementation begins automatically

The session will continue until all steps pass validation or max iterations reached.
Exit is blocked until completion - use 'iter complete' when done or 'iter reset' to abort.
`,
		task,
		prompts.SystemRules,
		prompts.ValidationRules,
		prompts.ArchitectRole,
		state.Iteration, state.MaxIterations, state.Workdir)

	return nil
}

// cmdWorkflow starts a workflow-based implementation session.
func cmdWorkflow(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter workflow \"<workflow-spec>\"")
	}

	spec := args[0]
	maxIterations := 50

	for i := 1; i < len(args); i++ {
		if args[i] == "--max-iterations" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid max-iterations: %w", err)
			}
			maxIterations = n
			i++
		}
	}

	// Generate unique workdir path
	workdir := generateWorkdirPath(spec)

	state := &State{
		Task:           "Workflow execution",
		Mode:           "workflow",
		WorkflowSpec:   spec,
		Workdir:        workdir,
		MaxIterations:  maxIterations,
		Iteration:      1,
		Phase:          "architect",
		CurrentStep:    1,
		StartedAt:      time.Now(),
		LastActivityAt: time.Now(),
		Artifacts:      []string{},
		Verdicts:       []Verdict{},
	}

	if err := os.MkdirAll(workdir, 0755); err != nil {
		return fmt.Errorf("failed to create workdir: %w", err)
	}

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf(`# WORKFLOW EXECUTION

%s

%s

---

## Workflow Specification
%s

---

%s

## Instructions
1. Parse the workflow specification above
2. Identify phases/stages and success criteria
3. Execute each phase, validating success criteria
4. Iterate until all criteria are met

## Session Info
- Iteration: %d/%d
- State: .iter/state.json
- Artifacts: %s/

Use 'iter pass'/'iter reject' to record phase outcomes.
Use 'iter next' to advance phases.
Use 'iter complete' when workflow is done.
`,
		prompts.SystemRules,
		prompts.ValidationRules,
		spec,
		prompts.WorkflowSystem,
		state.Iteration, state.MaxIterations, state.Workdir)

	return nil
}

// cmdStatus shows current session status.
func cmdStatus(args []string) error {
	state, err := loadState()
	if err != nil {
		fmt.Println("No active iter session.")
		return nil
	}

	elapsed := time.Since(state.StartedAt).Round(time.Second)

	fmt.Printf(`# Iter Session Status

Task: %s
Mode: %s
Phase: %s
Iteration: %d/%d
Step: %d/%d
Validation Pass: %d
Rejections: %d
Elapsed: %s
Completed: %v
`, state.Task, state.Mode, state.Phase, state.Iteration, state.MaxIterations,
		state.CurrentStep, state.TotalSteps, state.ValidationPass, state.Rejections,
		elapsed, state.Completed)

	if len(state.Verdicts) > 0 {
		fmt.Println("\n## Recent Verdicts")
		start := len(state.Verdicts) - 5
		if start < 0 {
			start = 0
		}
		for _, v := range state.Verdicts[start:] {
			fmt.Printf("- Step %d Pass %d: %s\n", v.Step, v.Pass, v.Status)
			for _, r := range v.Reasons {
				fmt.Printf("  - %s\n", r)
			}
		}
	}

	return nil
}

// cmdStep outputs current or specified step instructions.
func cmdStep(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	stepNum := state.CurrentStep
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid step number: %w", err)
		}
		stepNum = n
	}

	// Try to read step file
	stepFile := filepath.Join(state.Workdir, fmt.Sprintf("step_%d.md", stepNum))
	content, err := os.ReadFile(stepFile)

	fmt.Printf(`# STEP %d IMPLEMENTATION

%s

---

`, stepNum, prompts.WorkerRole)

	if err != nil {
		fmt.Printf(`## Step Document
No step_%d.md found in .iter/workdir/

If architect phase is not complete, create step documents first.

## Current Task
%s

Iteration: %d/%d
`, stepNum, state.Task, state.Iteration, state.MaxIterations)
	} else {
		fmt.Printf("## Step Document\n\n%s\n", string(content))
	}

	fmt.Printf(`
---

## After Implementation
1. Verify build passes
2. Create step_%d_impl.md documenting what you did
3. Validation will run automatically
`, stepNum)

	return nil
}

// cmdPass records a validation pass.
func cmdPass(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	state.LastActivityAt = time.Now()
	state.Verdicts = append(state.Verdicts, Verdict{
		Step:      state.CurrentStep,
		Pass:      state.ValidationPass,
		Status:    "pass",
		Reasons:   []string{"All validation checks passed"},
		Timestamp: time.Now(),
	})

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("PASS: Step %d validated successfully.\n", state.CurrentStep)

	if state.CurrentStep >= state.TotalSteps && state.TotalSteps > 0 {
		fmt.Println("\nAll steps completed. Run 'iter complete' to finalize.")
	} else {
		fmt.Println("\nRun 'iter next' to proceed to next step.")
	}

	return nil
}

// cmdReject records a validation rejection.
func cmdReject(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter reject \"<reason>\"")
	}

	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	reason := strings.Join(args, " ")
	state.Rejections++
	state.LastActivityAt = time.Now()
	state.Verdicts = append(state.Verdicts, Verdict{
		Step:      state.CurrentStep,
		Pass:      state.ValidationPass,
		Status:    "reject",
		Reasons:   []string{reason},
		Timestamp: time.Now(),
	})
	state.Phase = "worker" // Back to worker

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("REJECT: Step %d (pass %d)\nReason: %s\n", state.CurrentStep, state.ValidationPass, reason)
	fmt.Println("\nFix the issue and validation will run again.")

	return nil
}

// cmdNext moves to the next step.
func cmdNext(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	state.CurrentStep++
	state.ValidationPass = 0
	state.Phase = "worker"
	state.LastActivityAt = time.Now()

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("Moved to step %d.\n", state.CurrentStep)

	if state.CurrentStep > state.TotalSteps && state.TotalSteps > 0 {
		fmt.Println("All planned steps complete. Run 'iter complete' to finalize.")
	}

	return nil
}

// cmdComplete marks the session as complete.
func cmdComplete(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	state.Completed = true
	state.ExitSignal = true
	state.Phase = "complete"
	state.LastActivityAt = time.Now()

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Create summary
	summaryPath := filepath.Join(state.Workdir, "summary.md")
	summary := fmt.Sprintf(`# Iter Session Summary

## Task
%s

## Results
- Mode: %s
- Total Iterations: %d
- Steps Completed: %d/%d
- Rejections: %d

## Verdicts
`, state.Task, state.Mode, state.Iteration, state.CurrentStep, state.TotalSteps, state.Rejections)

	for _, v := range state.Verdicts {
		summary += fmt.Sprintf("\n### Step %d Pass %d: %s\n", v.Step, v.Pass, v.Status)
		for _, r := range v.Reasons {
			summary += fmt.Sprintf("- %s\n", r)
		}
	}

	if err := os.WriteFile(summaryPath, []byte(summary), 0644); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	fmt.Println("Session complete.")
	fmt.Printf("Summary: %s\n", summaryPath)

	return nil
}

// cmdReset clears session state.
func cmdReset(args []string) error {
	if err := os.RemoveAll(stateDir); err != nil {
		return fmt.Errorf("failed to remove state: %w", err)
	}
	fmt.Println("Iter session reset.")
	return nil
}

// cmdHookStop handles the stop hook for Claude Code.
func cmdHookStop(args []string) error {
	state, err := loadState()
	if err != nil {
		// No session - allow exit
		return outputJSON(HookResponse{Continue: true})
	}

	if state.Completed {
		return outputJSON(HookResponse{
			Continue:      true,
			SystemMessage: "Iter session completed.",
		})
	}

	if state.Iteration >= state.MaxIterations {
		return outputJSON(HookResponse{
			Continue:      true,
			SystemMessage: fmt.Sprintf("Max iterations reached (%d).", state.MaxIterations),
		})
	}

	// Continue loop
	state.Iteration++
	state.LastActivityAt = time.Now()
	if err := saveState(state); err != nil {
		return outputJSON(HookResponse{
			Continue:      true,
			SystemMessage: fmt.Sprintf("Warning: state save failed: %v", err),
		})
	}

	var nextPrompt string
	switch state.Phase {
	case "architect":
		state.Phase = "worker"
		_ = saveState(state)
		nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Architect phase complete. Begin implementation.

%s

Read .iter/workdir/step_%d.md and implement exactly as specified.
After implementation, validation runs automatically.`,
			state.Iteration, state.MaxIterations,
			prompts.WorkerRole,
			state.CurrentStep)

	case "worker":
		state.Phase = "validator"
		state.ValidationPass++
		_ = saveState(state)
		nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Implementation complete. Running validation.

%s

Review step %d implementation now.
Call 'iter pass' or 'iter reject "reason"' when done.`,
			state.Iteration, state.MaxIterations,
			prompts.ValidatorRole,
			state.CurrentStep)

	case "validator":
		if len(state.Verdicts) > 0 {
			last := state.Verdicts[len(state.Verdicts)-1]
			if last.Status == "reject" {
				nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Step %d was REJECTED:
%s

%s

Fix the issues and validation will run again.`,
					state.Iteration, state.MaxIterations,
					state.CurrentStep,
					strings.Join(last.Reasons, "\n"),
					prompts.WorkerRole)
			} else {
				nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Step %d PASSED. Run 'iter next' to proceed.

Current progress: Step %d/%d`,
					state.Iteration, state.MaxIterations,
					state.CurrentStep,
					state.CurrentStep, state.TotalSteps)
			}
		} else {
			nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Run validation for step %d.

%s`,
				state.Iteration, state.MaxIterations,
				state.CurrentStep,
				prompts.ValidatorRole)
		}

	default:
		nextPrompt = fmt.Sprintf(`# CONTINUE: Iteration %d/%d

Phase: %s | Step: %d/%d

Run 'iter status' for session details.`,
			state.Iteration, state.MaxIterations,
			state.Phase, state.CurrentStep, state.TotalSteps)
	}

	return outputJSON(HookResponse{
		Continue:      false,
		SystemMessage: nextPrompt,
	})
}

// Code Index Commands

// cmdIndex handles the index subcommand.
func cmdIndex(args []string) error {
	// Get current working directory as repo root
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := index.DefaultConfig(repoRoot)

	// Determine subcommand
	subCmd := ""
	if len(args) > 0 {
		subCmd = args[0]
	}

	switch subCmd {
	case "build":
		return cmdIndexBuild(cfg)
	case "clear":
		return cmdIndexClear(cfg)
	case "watch":
		return cmdIndexWatch(cfg)
	default:
		return cmdIndexStatus(cfg)
	}
}

// cmdIndexStatus shows index statistics.
func cmdIndexStatus(cfg index.Config) error {
	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	stats := idx.Stats()

	fmt.Printf(`# Code Index Status

Documents: %d
Files indexed: %d
Current branch: %s
Last updated: %s
Watcher running: %v

Index path: %s
`, stats.DocumentCount, stats.FileCount, stats.CurrentBranch,
		formatTime(stats.LastUpdated), stats.WatcherRunning,
		filepath.Join(cfg.RepoRoot, cfg.IndexPath))

	return nil
}

// cmdIndexBuild performs a full repository index.
func cmdIndexBuild(cfg index.Config) error {
	fmt.Println("Building code index...")

	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	start := time.Now()
	if err := idx.IndexAll(); err != nil {
		return fmt.Errorf("index all: %w", err)
	}

	stats := idx.Stats()
	fmt.Printf("Indexed %d symbols from %d files in %s\n",
		stats.DocumentCount, stats.FileCount, time.Since(start).Round(time.Millisecond))

	return nil
}

// cmdIndexClear clears and rebuilds the index.
func cmdIndexClear(cfg index.Config) error {
	fmt.Println("Clearing index...")

	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	if err := idx.Clear(); err != nil {
		return fmt.Errorf("clear index: %w", err)
	}

	fmt.Println("Index cleared. Run 'iter index build' to rebuild.")
	return nil
}

// cmdIndexWatch starts the file watcher.
func cmdIndexWatch(cfg index.Config) error {
	fmt.Println("Starting file watcher...")

	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	watcher, err := index.NewWatcher(idx)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	if err := watcher.Start(); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	fmt.Printf("Watching for changes in %s\n", cfg.RepoRoot)
	fmt.Println("Press Ctrl+C to stop.")

	// Block until interrupted
	select {}
}

// cmdSearch searches the code index.
func cmdSearch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter search \"<query>\" [--kind=<type>] [--path=<prefix>] [--branch=<branch>] [--limit=<n>]")
	}

	query := args[0]
	opts := index.SearchOptions{
		Query: query,
		Limit: 10,
	}

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--kind=") {
			opts.SymbolKind = strings.TrimPrefix(arg, "--kind=")
		} else if strings.HasPrefix(arg, "--path=") {
			opts.FilePath = strings.TrimPrefix(arg, "--path=")
		} else if strings.HasPrefix(arg, "--branch=") {
			opts.Branch = strings.TrimPrefix(arg, "--branch=")
		} else if strings.HasPrefix(arg, "--limit=") {
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit="))
			if err == nil && n > 0 {
				opts.Limit = n
			}
		}
	}

	// Get current working directory as repo root
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := index.DefaultConfig(repoRoot)

	idx, err := index.NewIndexer(cfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	searcher := index.NewSearcher(idx)
	results, err := searcher.Search(context.Background(), opts)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	// Output formatted results
	fmt.Println(index.FormatResults(results))

	return nil
}

// formatTime formats a time for display.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}

// State persistence

func loadState() (*State, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, stateFile))
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func saveState(state *State) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, stateFile), data, 0644)
}

func outputJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
