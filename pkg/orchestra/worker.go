package orchestra

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/ternarybob/iter/pkg/llm"
)

// Worker is the implementation agent that executes steps.
type Worker struct {
	provider llm.Provider
	workdir  *WorkdirManager
}

// NewWorker creates a new worker agent.
func NewWorker(provider llm.Provider, workdir *WorkdirManager) *Worker {
	return &Worker{
		provider: provider,
		workdir:  workdir,
	}
}

// Execute implements a single step.
func (w *Worker) Execute(ctx context.Context, step *Step) (*StepResult, error) {
	result := &StepResult{
		StepNumber: step.Number,
		Iteration:  1,
	}

	// Build the execution prompt
	prompt := buildExecutionPrompt(step)

	// Call LLM for implementation
	req := &llm.CompletionRequest{
		System: workerSystemPrompt,
		Messages: []llm.Message{
			llm.UserMessage(prompt),
		},
		MaxTokens: 8192,
	}

	resp, err := w.provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	// Parse response for changes
	result.Changes = parseChanges(resp.Content)
	result.Notes = resp.Content

	// Run verification commands
	for _, cmd := range step.VerificationCommands {
		passed, logPath := w.runVerification(ctx, cmd, step.Number, result.Iteration)
		if !passed {
			result.BuildPassed = false
			result.BuildLog = logPath
		} else {
			result.BuildPassed = true
		}
	}

	// Default to passed if no verification commands
	if len(step.VerificationCommands) == 0 {
		result.BuildPassed = true
	}

	// Write implementation notes
	if w.workdir != nil {
		implDoc := w.generateImplDoc(step, result, resp.Content)
		if err := w.workdir.WriteStepImpl(step.Number, implDoc); err != nil {
			// Non-fatal error
			_ = err
		}
	}

	result.Document = resp.Content
	return result, nil
}

// Iterate fixes issues from rejection.
func (w *Worker) Iterate(ctx context.Context, step *Step, verdict *Verdict) (*StepResult, error) {
	result := &StepResult{
		StepNumber: step.Number,
	}

	// Build the iteration prompt
	prompt := buildIterationPrompt(step, verdict)

	// Call LLM for fixes
	req := &llm.CompletionRequest{
		System: workerSystemPrompt,
		Messages: []llm.Message{
			llm.UserMessage(prompt),
		},
		MaxTokens: 8192,
	}

	resp, err := w.provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	// Parse response for changes
	result.Changes = parseChanges(resp.Content)
	result.Notes = resp.Content

	// Run verification commands
	for _, cmd := range step.VerificationCommands {
		passed, logPath := w.runVerification(ctx, cmd, step.Number, result.Iteration)
		if !passed {
			result.BuildPassed = false
			result.BuildLog = logPath
		} else {
			result.BuildPassed = true
		}
	}

	if len(step.VerificationCommands) == 0 {
		result.BuildPassed = true
	}

	// Write implementation notes
	if w.workdir != nil {
		implDoc := w.generateImplDoc(step, result, resp.Content)
		if err := w.workdir.WriteStepImpl(step.Number, implDoc); err != nil {
			_ = err
		}
	}

	result.Document = resp.Content
	return result, nil
}

// workerSystemPrompt is the system prompt for the worker agent.
const workerSystemPrompt = `You are an expert software engineer responsible for implementing code changes.

Your responsibilities:
1. Follow step documents EXACTLY - no interpretation or deviation
2. Match existing codebase patterns precisely
3. Implement all specified changes
4. Perform all cleanup as specified
5. Verify build passes before completion

Rules:
- CORRECTNESS over SPEED - never rush
- Requirements are LAW - implement exactly as specified
- EXISTING PATTERNS ARE LAW - match codebase style exactly
- CLEANUP IS MANDATORY - remove all dead code specified
- BUILD VERIFICATION IS MANDATORY - verify after each change

Output Format:
# Implementation: Step N

## Changes Made

### File: path/to/file.go
` + "```go" + `
[code changes]
` + "```" + `

## Cleanup Performed
- Removed: [item] from [file]

## Verification
- Build: PASS/FAIL
- Reason: [if failed]

## Notes
[any implementation notes]`

// buildExecutionPrompt creates the prompt for step execution.
func buildExecutionPrompt(step *Step) string {
	var sb stringBuilder

	sb.WriteString("Implement the following step exactly as specified.\n\n")
	sb.WriteString("## Step Document\n")
	sb.WriteString(step.Document + "\n\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Implement ALL changes specified in the step document\n")
	sb.WriteString("2. Follow existing codebase patterns EXACTLY\n")
	sb.WriteString("3. Perform ALL cleanup specified\n")
	sb.WriteString("4. Verify build passes\n\n")

	sb.WriteString("Provide your implementation in the specified format.\n")
	sb.WriteString("Include complete code for any files you modify.\n")

	return sb.String()
}

// buildIterationPrompt creates the prompt for fixing rejected changes.
func buildIterationPrompt(step *Step, verdict *Verdict) string {
	var sb stringBuilder

	sb.WriteString("Fix the issues identified in the validation.\n\n")
	sb.WriteString("## Original Step Document\n")
	sb.WriteString(step.Document + "\n\n")

	sb.WriteString("## Validation Verdict: REJECT\n\n")
	sb.WriteString("## Issues to Fix\n")
	for i, reason := range verdict.Reasons {
		sb.WriteString(itoa(i+1) + ". " + reason + "\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Address EACH issue listed above\n")
	sb.WriteString("2. Maintain all previous correct changes\n")
	sb.WriteString("3. Verify build passes after fixes\n\n")

	sb.WriteString("Provide your fixed implementation in the specified format.\n")

	return sb.String()
}

// parseChanges extracts file changes from the LLM response.
func parseChanges(content string) []Change {
	var changes []Change

	lines := splitLines(content)
	var currentFile string
	var currentCode stringBuilder
	inCodeBlock := false

	for _, line := range lines {
		if hasPrefix(line, "### File:") || hasPrefix(line, "## File:") {
			// Save previous file
			if currentFile != "" && currentCode.String() != "" {
				changes = append(changes, Change{
					Type:  ChangeModify,
					Path:  currentFile,
					After: trimSpace(currentCode.String()),
				})
			}
			// Start new file
			currentFile = trimSpace(trimPrefix(trimPrefix(line, "### File:"), "## File:"))
			currentCode = stringBuilder{}
			inCodeBlock = false
		} else if hasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				inCodeBlock = false
			} else {
				// Start of code block
				inCodeBlock = true
			}
		} else if inCodeBlock {
			currentCode.WriteString(line + "\n")
		}
	}

	// Save last file
	if currentFile != "" && currentCode.String() != "" {
		changes = append(changes, Change{
			Type:  ChangeModify,
			Path:  currentFile,
			After: trimSpace(currentCode.String()),
		})
	}

	return changes
}

// runVerification runs a verification command.
func (w *Worker) runVerification(ctx context.Context, cmd string, stepNum, iteration int) (bool, string) {
	// Create a shell command
	shellCmd := exec.CommandContext(ctx, "sh", "-c", cmd)

	// Capture output
	output, err := shellCmd.CombinedOutput()

	// Write to log file
	logName := fmt.Sprintf("build_step%d_iter%d.log", stepNum, iteration)
	if w.workdir != nil {
		_ = w.workdir.WriteLog(logName, output)
	}

	logPath := ""
	if w.workdir != nil {
		logPath = w.workdir.GetLogPath(logName)
	}

	return err == nil, logPath
}

// generateImplDoc generates the step_N_impl.md content.
func (w *Worker) generateImplDoc(step *Step, result *StepResult, notes string) string {
	var sb stringBuilder

	sb.WriteString("# Implementation: Step " + itoa(step.Number) + "\n\n")

	sb.WriteString("## Iteration: " + itoa(result.Iteration) + "\n\n")

	sb.WriteString("## Changes Made\n")
	for _, change := range result.Changes {
		sb.WriteString("- " + string(change.Type) + ": " + change.Path + "\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Build Status\n")
	if result.BuildPassed {
		sb.WriteString("- Result: PASS\n")
	} else {
		sb.WriteString("- Result: FAIL\n")
		sb.WriteString("- Log: " + result.BuildLog + "\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Notes\n")
	sb.WriteString(notes + "\n")

	return sb.String()
}
