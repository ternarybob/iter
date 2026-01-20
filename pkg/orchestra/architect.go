package orchestra

import (
	"context"
	"fmt"

	"github.com/ternarybob/iter/pkg/llm"
)

// Architect is the planning agent that analyzes requirements and creates step documents.
type Architect struct {
	provider llm.Provider
	workdir  *WorkdirManager
}

// NewArchitect creates a new architect agent.
func NewArchitect(provider llm.Provider, workdir *WorkdirManager) *Architect {
	return &Architect{
		provider: provider,
		workdir:  workdir,
	}
}

// Analyze extracts requirements from a task.
func (a *Architect) Analyze(ctx context.Context, task *Task) (*Requirements, error) {
	// Build the analysis prompt
	prompt := buildAnalysisPrompt(task)

	// Call LLM
	req := &llm.CompletionRequest{
		System: architectSystemPrompt,
		Messages: []llm.Message{
			llm.UserMessage(prompt),
		},
		MaxTokens: 4096,
	}

	resp, err := a.provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	// Parse response into requirements
	reqs := parseRequirements(resp.Content, task)

	// Write requirements.md
	if a.workdir != nil {
		if err := a.workdir.WriteRequirements(reqs.Document); err != nil {
			return nil, fmt.Errorf("write requirements: %w", err)
		}
	}

	return reqs, nil
}

// Plan creates step documents from requirements.
func (a *Architect) Plan(ctx context.Context, reqs *Requirements) ([]Step, error) {
	// Build the planning prompt
	prompt := buildPlanningPrompt(reqs)

	// Call LLM
	req := &llm.CompletionRequest{
		System: architectSystemPrompt,
		Messages: []llm.Message{
			llm.UserMessage(prompt),
		},
		MaxTokens: 8192,
	}

	resp, err := a.provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	// Parse response into steps
	steps := parseSteps(resp.Content)

	// Write step documents
	if a.workdir != nil {
		for _, step := range steps {
			if err := a.workdir.WriteStep(step.Number, step.Document); err != nil {
				return nil, fmt.Errorf("write step %d: %w", step.Number, err)
			}
		}

		// Write architect analysis
		if err := a.workdir.WriteArchitectAnalysis(resp.Content); err != nil {
			return nil, fmt.Errorf("write analysis: %w", err)
		}
	}

	return steps, nil
}

// architectSystemPrompt is the system prompt for the architect agent.
const architectSystemPrompt = `You are an expert software architect responsible for analyzing requirements and creating detailed implementation plans.

Your responsibilities:
1. Extract clear, verifiable requirements from task descriptions
2. Analyze existing codebase patterns and conventions
3. Create detailed step-by-step implementation plans
4. Identify cleanup targets (dead code, deprecated functions)
5. Specify acceptance criteria for each step

Rules:
- Requirements must be specific and verifiable
- Each step must have clear dependencies
- Steps should be atomic and independently verifiable
- Always identify patterns to follow from existing code
- Always specify what needs to be cleaned up
- Use REQ-N format for requirements
- Use AC-N format for acceptance criteria

Output Format for Requirements:
# Requirements

## REQ-1: [title]
[description]

### Acceptance Criteria
- AC-1: [criterion]
- AC-2: [criterion]

---

Output Format for Steps:
# Step N: [title]

## Dependencies
[none | step_1, step_2]

## Requirements Addressed
- REQ-N

## Approach
[detailed implementation approach]

## Files to Modify
- path/to/file.go

## Cleanup Required
- Remove: [item] (reason)

## Acceptance Criteria
- AC-1: [criterion]

## Verification Commands
` + "```bash" + `
go build ./...
go test ./path/...
` + "```"

// buildAnalysisPrompt creates the prompt for requirement analysis.
func buildAnalysisPrompt(task *Task) string {
	var sb stringBuilder

	sb.WriteString("Analyze the following task and extract clear, verifiable requirements.\n\n")
	sb.WriteString("## Task Description\n")
	sb.WriteString(task.Description + "\n\n")

	if len(task.Files) > 0 {
		sb.WriteString("## Relevant Files\n")
		for _, f := range task.Files {
			sb.WriteString("- " + f + "\n")
		}
		sb.WriteString("\n")
	}

	if len(task.Context) > 0 {
		sb.WriteString("## Additional Context\n")
		for k, v := range task.Context {
			sb.WriteString("- " + k + ": " + fmt.Sprint(v) + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Please extract requirements in the specified format.\n")
	sb.WriteString("Each requirement should have clear acceptance criteria.\n")

	return sb.String()
}

// buildPlanningPrompt creates the prompt for step planning.
func buildPlanningPrompt(reqs *Requirements) string {
	var sb stringBuilder

	sb.WriteString("Create a detailed implementation plan for the following requirements.\n\n")
	sb.WriteString("## Requirements\n")
	sb.WriteString(reqs.Document + "\n\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Break down the implementation into atomic steps\n")
	sb.WriteString("2. Identify dependencies between steps\n")
	sb.WriteString("3. For each step, specify files to modify and patterns to follow\n")
	sb.WriteString("4. Identify cleanup targets (dead code, deprecated code)\n")
	sb.WriteString("5. Provide verification commands for each step\n\n")

	sb.WriteString("Please create step documents in the specified format.\n")

	return sb.String()
}

// parseRequirements parses the LLM response into Requirements.
func parseRequirements(content string, task *Task) *Requirements {
	reqs := &Requirements{
		SourceTask: task,
		Document:   content,
		Analysis:   content,
	}

	// Parse requirement items from content
	// This is a simplified parser - a real implementation would be more robust
	lines := splitLines(content)
	var currentReq *Requirement

	for _, line := range lines {
		line = trimSpace(line)

		if hasPrefix(line, "## REQ-") || hasPrefix(line, "# REQ-") {
			if currentReq != nil {
				reqs.Items = append(reqs.Items, *currentReq)
			}
			// Extract ID and title
			colonIdx := indexOf(line, ":")
			if colonIdx > 0 {
				id := extractReqID(line[:colonIdx])
				title := trimSpace(line[colonIdx+1:])
				currentReq = &Requirement{
					ID:          id,
					Description: title,
				}
			}
		} else if hasPrefix(line, "- AC-") && currentReq != nil {
			ac := trimSpace(trimPrefix(line, "- "))
			currentReq.AcceptanceCriteria = append(currentReq.AcceptanceCriteria, ac)
		}
	}

	if currentReq != nil {
		reqs.Items = append(reqs.Items, *currentReq)
	}

	return reqs
}

// extractReqID extracts REQ-N from a line.
func extractReqID(s string) string {
	// Find REQ- pattern
	idx := indexOf(s, "REQ-")
	if idx < 0 {
		return "REQ-1"
	}
	// Extract REQ-N
	start := idx
	end := start + 4 // len("REQ-")
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	return s[start:end]
}

// parseSteps parses the LLM response into Steps.
func parseSteps(content string) []Step {
	var steps []Step

	// Split by step markers
	parts := splitByStepMarker(content)

	for _, part := range parts {
		step, err := ParseStep(part)
		if err != nil || step.Number == 0 {
			continue
		}
		step.Document = part
		steps = append(steps, *step)
	}

	// Ensure sequential numbering
	for i := range steps {
		if steps[i].Number == 0 {
			steps[i].Number = i + 1
		}
	}

	return steps
}

// splitByStepMarker splits content into step sections.
func splitByStepMarker(content string) []string {
	var parts []string
	lines := splitLines(content)

	var current stringBuilder
	inStep := false

	for _, line := range lines {
		if hasPrefix(line, "# Step ") {
			if inStep && current.String() != "" {
				parts = append(parts, current.String())
			}
			current = stringBuilder{}
			inStep = true
		}
		if inStep {
			current.WriteString(line + "\n")
		}
	}

	if inStep && current.String() != "" {
		parts = append(parts, current.String())
	}

	return parts
}
