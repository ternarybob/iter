// Package main provides the CLI entry point for the iter Claude Code plugin.
//
// iter is an adversarial multi-agent DevOps loop that runs within Claude Code.
// It implements an Architect -> Worker -> Validator feedback cycle where:
//   - Architect: Analyzes requirements and creates step documents
//   - Worker: Implements steps exactly as specified (Claude Code)
//   - Validator: Adversarially reviews implementations (default REJECT)
//
// Usage:
//
//	iter init "<task>" [--max-iterations N]  - Start a new iter session
//	iter check                               - Check if work is complete (for stop hook)
//	iter analyze                             - Run architect analysis
//	iter validate                            - Run validator on current work
//	iter status                              - Show current iteration status
//	iter step [N]                            - Get current/specific step instructions
//	iter complete                            - Mark session as complete
//	iter reset                               - Reset session state
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	stateDir  = ".iter"
	stateFile = "state.json"
)

// State represents the persistent state of an iter session.
type State struct {
	Task           string    `json:"task"`
	MaxIterations  int       `json:"max_iterations"`
	Iteration      int       `json:"iteration"`
	Phase          string    `json:"phase"` // init, architect, worker, validator, complete
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
	Continue           bool               `json:"continue"`
	SuppressOutput     bool               `json:"suppressOutput,omitempty"`
	SystemMessage      string             `json:"systemMessage,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput provides hook-specific control.
type HookSpecificOutput struct {
	HookEventName     string            `json:"hookEventName,omitempty"`
	PermissionDecision string           `json:"permissionDecision,omitempty"`
	AdditionalContext string            `json:"additionalContext,omitempty"`
	UpdatedInput      map[string]any    `json:"updatedInput,omitempty"`
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
	case "init":
		err = cmdInit(args)
	case "check":
		err = cmdCheck(args)
	case "analyze":
		err = cmdAnalyze(args)
	case "validate":
		err = cmdValidate(args)
	case "status":
		err = cmdStatus(args)
	case "step":
		err = cmdStep(args)
	case "complete":
		err = cmdComplete(args)
	case "reset":
		err = cmdReset(args)
	case "reject":
		err = cmdReject(args)
	case "pass":
		err = cmdPass(args)
	case "next":
		err = cmdNext(args)
	case "hook-stop":
		err = cmdHookStop(args)
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
	fmt.Println(`iter - Adversarial multi-agent DevOps loop for Claude Code

Commands:
  init "<task>" [--max-iterations N]  Start a new iter session
  check                               Check if work is complete (for stop hook)
  analyze                             Output architect analysis prompt
  validate                            Output validator review prompt
  status                              Show current iteration status
  step [N]                            Get current/specific step instructions
  complete                            Mark session as complete
  reset                               Reset session state
  reject "<reason>"                   Record a validation rejection
  pass                                Record a validation pass
  next                                Move to next step
  hook-stop                           Stop hook handler (outputs JSON)
  help                                Show this help

The iter plugin creates an adversarial feedback loop:
  1. /iter-loop starts the session
  2. Architect analyzes and creates step documents
  3. Worker (Claude Code) implements each step
  4. Validator reviews with adversarial stance (default REJECT)
  5. Loop continues until all steps pass or max iterations reached`)
}

// cmdInit initializes a new iter session.
func cmdInit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: iter init \"<task>\" [--max-iterations N]")
	}

	task := args[0]
	maxIterations := 50 // default

	// Parse optional flags
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

	state := &State{
		Task:           task,
		MaxIterations:  maxIterations,
		Iteration:      0,
		Phase:          "architect",
		CurrentStep:    0,
		TotalSteps:     0,
		StartedAt:      time.Now(),
		LastActivityAt: time.Now(),
		Artifacts:      []string{},
		Verdicts:       []Verdict{},
	}

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("Iter session initialized\n")
	fmt.Printf("Task: %s\n", task)
	fmt.Printf("Max iterations: %d\n", maxIterations)
	fmt.Printf("Phase: architect\n")
	fmt.Printf("\nRun /iter-analyze to begin architect analysis.\n")

	return nil
}

// cmdCheck checks if the work is complete (for stop hook).
func cmdCheck(args []string) error {
	state, err := loadState()
	if err != nil {
		// No state = not in an iter session
		fmt.Println("NOT_ITER_SESSION")
		return nil
	}

	if state.Completed {
		fmt.Println("COMPLETE")
		return nil
	}

	if state.Iteration >= state.MaxIterations {
		fmt.Println("MAX_ITERATIONS")
		return nil
	}

	fmt.Println("CONTINUE")
	return nil
}

// cmdAnalyze outputs the architect analysis prompt.
func cmdAnalyze(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session (run 'iter init' first)")
	}

	state.Phase = "architect"
	state.Iteration++
	state.LastActivityAt = time.Now()
	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	prompt := fmt.Sprintf(`# ARCHITECT ANALYSIS

You are the ARCHITECT agent in an adversarial multi-agent system.

## Task
%s

## Your Role
Analyze the requirements and create a detailed implementation plan.

## Instructions
1. **Analyze Existing Patterns**: Before planning, examine the codebase for existing patterns, conventions, and architecture
2. **Create Requirements Document**: List all explicit and implicit requirements
3. **Identify Cleanup Targets**: Find any dead code, redundant patterns, or technical debt to address
4. **Create Step Documents**: Break the work into discrete, verifiable steps

## Output Format
Create the following files in .iter/workdir/:

### requirements.md
- List all requirements with unique IDs (R1, R2, etc.)
- Mark requirements as MUST, SHOULD, or MAY
- Include implicit requirements (error handling, testing, etc.)

### step_1.md, step_2.md, etc.
For each step, include:
- **Title**: Brief description
- **Dependencies**: Which steps must complete first
- **Requirements**: Which requirements this step addresses (R1, R2, etc.)
- **Approach**: Detailed implementation approach
- **Cleanup**: Any dead code or redundancy to remove
- **Acceptance Criteria**: How to verify the step is complete

### architect-analysis.md
- Existing patterns found
- Architectural decisions
- Risk assessment
- Total steps planned

## Constraints
- Steps must be independently verifiable
- Each step must trace to at least one requirement
- Cleanup is MANDATORY - identify and remove dead code
- Build must pass after each step

Iteration: %d/%d`, state.Task, state.Iteration, state.MaxIterations)

	fmt.Println(prompt)
	return nil
}

// cmdValidate outputs the validator review prompt.
func cmdValidate(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session (run 'iter init' first)")
	}

	state.Phase = "validator"
	state.ValidationPass++
	state.LastActivityAt = time.Now()
	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	prompt := fmt.Sprintf(`# VALIDATOR REVIEW

You are the VALIDATOR agent in an adversarial multi-agent system.

## DEFAULT STANCE: REJECT

Your job is to find problems. Assume the implementation is wrong until proven correct.

## Task Being Validated
%s

## Current Step: %d

## Validation Checklist

### 1. Build Verification
- [ ] Run 'go build ./...' - AUTO-REJECT if it fails
- [ ] Run 'go test ./...' - AUTO-REJECT if tests fail
- [ ] Run 'golangci-lint run' - AUTO-REJECT on lint errors

### 2. Requirements Traceability
- [ ] Every change must trace to a requirement in requirements.md
- [ ] AUTO-REJECT if any change lacks requirement traceability
- [ ] Verify each requirement marked for this step is actually implemented

### 3. Code Quality
- [ ] Changes match existing codebase patterns
- [ ] Error handling is complete (no ignored errors)
- [ ] No fmt.Println (use slog)
- [ ] Proper error wrapping with context

### 4. Cleanup Verification
- [ ] All cleanup items from step doc are addressed
- [ ] No new dead code introduced
- [ ] No redundant patterns

### 5. Step Completion
- [ ] All acceptance criteria from step doc are met
- [ ] Changes are minimal and focused
- [ ] No scope creep beyond step requirements

## Output
After review, run one of:
- 'iter pass' - If ALL checks pass
- 'iter reject "reason"' - If ANY check fails (include specific reason)

Validation pass: %d
Iteration: %d/%d`, state.Task, state.CurrentStep, state.ValidationPass, state.Iteration, state.MaxIterations)

	fmt.Println(prompt)
	return nil
}

// cmdStatus shows current session status.
func cmdStatus(args []string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active iter session")
	}

	elapsed := time.Since(state.StartedAt).Round(time.Second)

	fmt.Printf("Iter Session Status\n")
	fmt.Printf("==================\n")
	fmt.Printf("Task: %s\n", state.Task)
	fmt.Printf("Phase: %s\n", state.Phase)
	fmt.Printf("Iteration: %d/%d\n", state.Iteration, state.MaxIterations)
	fmt.Printf("Current Step: %d/%d\n", state.CurrentStep, state.TotalSteps)
	fmt.Printf("Validation Pass: %d\n", state.ValidationPass)
	fmt.Printf("Rejections: %d\n", state.Rejections)
	fmt.Printf("Elapsed: %s\n", elapsed)
	fmt.Printf("Completed: %v\n", state.Completed)

	if len(state.Verdicts) > 0 {
		fmt.Printf("\nRecent Verdicts:\n")
		start := len(state.Verdicts) - 5
		if start < 0 {
			start = 0
		}
		for _, v := range state.Verdicts[start:] {
			fmt.Printf("  Step %d Pass %d: %s\n", v.Step, v.Pass, v.Status)
			for _, r := range v.Reasons {
				fmt.Printf("    - %s\n", r)
			}
		}
	}

	return nil
}

// cmdStep outputs the current or specified step instructions.
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

	// Read step file if it exists
	stepFile := filepath.Join(stateDir, "workdir", fmt.Sprintf("step_%d.md", stepNum))
	content, err := os.ReadFile(stepFile)
	if err != nil {
		// No step file yet, output worker prompt
		fmt.Printf(`# WORKER INSTRUCTIONS

You are the WORKER agent in an adversarial multi-agent system.

## CRITICAL RULES
- Follow step documents EXACTLY - no interpretation
- No changes beyond what the step specifies
- Verify build passes after each change
- Perform all cleanup specified in step doc
- Write step_%d_impl.md when done

## Current Task
%s

## Current Step: %d

Read .iter/workdir/step_%d.md for detailed instructions.

If no step document exists yet, run /iter-analyze first.

Iteration: %d/%d`, stepNum, state.Task, stepNum, stepNum, state.Iteration, state.MaxIterations)
		return nil
	}

	fmt.Printf("# Step %d Instructions\n\n", stepNum)
	fmt.Println(string(content))

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

	// Create summary.md
	summaryPath := filepath.Join(stateDir, "workdir", "summary.md")
	summary := fmt.Sprintf(`# Iter Session Summary

## Task
%s

## Results
- Total Iterations: %d
- Steps Completed: %d/%d
- Rejections: %d

## Verdicts
`, state.Task, state.Iteration, state.CurrentStep, state.TotalSteps, state.Rejections)

	for _, v := range state.Verdicts {
		summary += fmt.Sprintf("\n### Step %d Pass %d: %s\n", v.Step, v.Pass, v.Status)
		for _, r := range v.Reasons {
			summary += fmt.Sprintf("- %s\n", r)
		}
	}

	if err := os.MkdirAll(filepath.Dir(summaryPath), 0755); err != nil {
		return fmt.Errorf("failed to create workdir: %w", err)
	}
	if err := os.WriteFile(summaryPath, []byte(summary), 0644); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	fmt.Println("Session marked as complete.")
	fmt.Printf("Summary written to %s\n", summaryPath)

	return nil
}

// cmdReset resets the session state.
func cmdReset(args []string) error {
	if err := os.RemoveAll(stateDir); err != nil {
		return fmt.Errorf("failed to remove state directory: %w", err)
	}
	fmt.Println("Iter session reset.")
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

	if err := saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Printf("Rejection recorded for step %d (pass %d): %s\n", state.CurrentStep, state.ValidationPass, reason)
	fmt.Println("\nWorker must address this issue. Run /iter-step to continue.")

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

	fmt.Printf("Step %d passed validation.\n", state.CurrentStep)

	// Check if all steps complete
	if state.CurrentStep >= state.TotalSteps && state.TotalSteps > 0 {
		fmt.Println("\nAll steps completed! Run /iter-complete to finalize.")
	} else {
		fmt.Println("\nRun /iter-next to proceed to the next step.")
	}

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
	fmt.Println("Run /iter-step to get step instructions.")

	return nil
}

// cmdHookStop handles the stop hook (outputs JSON for Claude Code).
func cmdHookStop(args []string) error {
	state, err := loadState()
	if err != nil {
		// No iter session - allow normal exit
		resp := HookResponse{
			Continue: true,
		}
		return outputJSON(resp)
	}

	// Check completion conditions
	if state.Completed {
		resp := HookResponse{
			Continue:       true,
			SystemMessage:  "Iter session completed successfully.",
		}
		return outputJSON(resp)
	}

	if state.Iteration >= state.MaxIterations {
		resp := HookResponse{
			Continue:       true,
			SystemMessage:  fmt.Sprintf("Iter session reached max iterations (%d).", state.MaxIterations),
		}
		return outputJSON(resp)
	}

	// Continue the loop - inject the next prompt
	state.Iteration++
	state.LastActivityAt = time.Now()
	if err := saveState(state); err != nil {
		resp := HookResponse{
			Continue:      true,
			SystemMessage: fmt.Sprintf("Warning: failed to save state: %v", err),
		}
		return outputJSON(resp)
	}

	var nextPrompt string
	switch state.Phase {
	case "architect":
		nextPrompt = fmt.Sprintf(`Continue iter session (iteration %d/%d).

The Architect has created step documents. Now begin implementation.

Run /iter-step to get the current step instructions.
After implementing, run /iter-validate for adversarial review.`, state.Iteration, state.MaxIterations)
	case "worker":
		nextPrompt = fmt.Sprintf(`Continue iter session (iteration %d/%d).

You are the WORKER. Implement step %d exactly as specified.

Run /iter-step to see the instructions.
After implementing, run /iter-validate for review.`, state.Iteration, state.MaxIterations, state.CurrentStep)
	case "validator":
		// Check last verdict
		if len(state.Verdicts) > 0 {
			lastVerdict := state.Verdicts[len(state.Verdicts)-1]
			if lastVerdict.Status == "reject" {
				nextPrompt = fmt.Sprintf(`Continue iter session (iteration %d/%d).

Step %d was REJECTED by the Validator:
%s

You are the WORKER. Fix the issues and run /iter-validate again.`,
					state.Iteration, state.MaxIterations, state.CurrentStep,
					strings.Join(lastVerdict.Reasons, "\n"))
			} else {
				nextPrompt = fmt.Sprintf(`Continue iter session (iteration %d/%d).

Step %d passed validation. Run /iter-next to proceed to the next step.`,
					state.Iteration, state.MaxIterations, state.CurrentStep)
			}
		} else {
			nextPrompt = fmt.Sprintf(`Continue iter session (iteration %d/%d).

Run /iter-validate to review the current implementation.`, state.Iteration, state.MaxIterations)
		}
	default:
		nextPrompt = fmt.Sprintf(`Continue iter session (iteration %d/%d).

Current phase: %s
Current step: %d/%d

Run /iter-status for full session status.`, state.Iteration, state.MaxIterations, state.Phase, state.CurrentStep, state.TotalSteps)
	}

	resp := HookResponse{
		Continue:       false, // Block exit
		SuppressOutput: false,
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:     "Stop",
			AdditionalContext: nextPrompt,
		},
	}

	return outputJSON(resp)
}

// State persistence functions

func loadState() (*State, error) {
	path := filepath.Join(stateDir, stateFile)
	data, err := os.ReadFile(path)
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
