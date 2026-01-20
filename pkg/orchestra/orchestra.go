// Package orchestra provides multi-agent orchestration for Iter.
// It coordinates Architect, Worker, and Validator agents in an
// adversarial validation workflow.
package orchestra

import (
	"context"
	"fmt"
	"sync"

	"github.com/ternarybob/iter/pkg/llm"
)

// Orchestrator coordinates multi-agent workflows.
type Orchestrator interface {
	// Analyze extracts requirements from a task.
	Analyze(ctx context.Context, task *Task) (*Requirements, error)

	// Plan creates step documents from requirements.
	Plan(ctx context.Context, reqs *Requirements) ([]Step, error)

	// Execute implements a single step.
	Execute(ctx context.Context, step *Step) (*StepResult, error)

	// Validate reviews an implementation (adversarial).
	Validate(ctx context.Context, step *Step, result *StepResult) (*Verdict, error)

	// FinalValidate reviews all changes together.
	FinalValidate(ctx context.Context, results []*StepResult) (*Verdict, error)

	// Iterate fixes issues from rejection.
	Iterate(ctx context.Context, step *Step, verdict *Verdict) (*StepResult, error)
}

// Task represents a unit of work for the orchestrator.
type Task struct {
	ID          string
	Description string
	Files       []string
	Context     map[string]any
}

// Requirements represents extracted requirements.
type Requirements struct {
	Items      []Requirement
	SourceTask *Task
	Analysis   string
	Document   string // Raw markdown content
}

// Requirement represents a single requirement.
type Requirement struct {
	ID                 string
	Description        string
	Priority           int
	Source             string
	AcceptanceCriteria []string
}

// DefaultOrchestrator implements Orchestrator with LLM-based agents.
type DefaultOrchestrator struct {
	mu sync.Mutex

	router  *llm.Router
	workdir *WorkdirManager
	config  OrchestratorConfig

	// Current execution state
	requirements *Requirements
	steps        []Step
	results      []*StepResult
}

// OrchestratorConfig configures the orchestrator.
type OrchestratorConfig struct {
	// MaxValidationRetries is the maximum retries per step.
	MaxValidationRetries int

	// ParallelSteps enables parallel execution of independent steps.
	ParallelSteps bool

	// WorkDir is the working directory path.
	WorkDir string

	// PlanningModel is the model for architect.
	PlanningModel string

	// ExecutionModel is the model for worker.
	ExecutionModel string

	// ValidationModel is the model for validator.
	ValidationModel string
}

// NewOrchestrator creates a new orchestrator.
func NewOrchestrator(router *llm.Router, config OrchestratorConfig) (*DefaultOrchestrator, error) {
	workdir, err := NewWorkdirManager(config.WorkDir)
	if err != nil {
		return nil, err
	}

	if config.MaxValidationRetries == 0 {
		config.MaxValidationRetries = 5
	}

	return &DefaultOrchestrator{
		router:  router,
		workdir: workdir,
		config:  config,
	}, nil
}

// Analyze implements Orchestrator.Analyze.
func (o *DefaultOrchestrator) Analyze(ctx context.Context, task *Task) (*Requirements, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create workdir for this task
	taskName := sanitizeTaskName(task.Description)
	if _, err := o.workdir.Create(taskName); err != nil {
		return nil, fmt.Errorf("create workdir: %w", err)
	}

	// Use Architect agent to analyze requirements
	architect := NewArchitect(o.router.ForPlanning(), o.workdir)
	reqs, err := architect.Analyze(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("architect analyze: %w", err)
	}

	o.requirements = reqs
	return reqs, nil
}

// Plan implements Orchestrator.Plan.
func (o *DefaultOrchestrator) Plan(ctx context.Context, reqs *Requirements) ([]Step, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if reqs == nil {
		return nil, fmt.Errorf("requirements is nil")
	}

	// Use Architect agent to create step documents
	architect := NewArchitect(o.router.ForPlanning(), o.workdir)
	steps, err := architect.Plan(ctx, reqs)
	if err != nil {
		return nil, fmt.Errorf("architect plan: %w", err)
	}

	o.steps = steps
	o.results = make([]*StepResult, len(steps))

	return steps, nil
}

// Execute implements Orchestrator.Execute.
func (o *DefaultOrchestrator) Execute(ctx context.Context, step *Step) (*StepResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Use Worker agent to implement step
	worker := NewWorker(o.router.ForExecution(), o.workdir)
	result, err := worker.Execute(ctx, step)
	if err != nil {
		return nil, fmt.Errorf("worker execute: %w", err)
	}

	// Store result
	if step.Number > 0 && step.Number <= len(o.results) {
		o.results[step.Number-1] = result
	}

	return result, nil
}

// Validate implements Orchestrator.Validate.
func (o *DefaultOrchestrator) Validate(ctx context.Context, step *Step, result *StepResult) (*Verdict, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Use Validator agent with adversarial stance
	validator := NewValidator(o.router.ForValidation(), o.workdir)
	verdict, err := validator.Validate(ctx, step, result, o.requirements)
	if err != nil {
		return nil, fmt.Errorf("validator: %w", err)
	}

	return verdict, nil
}

// FinalValidate implements Orchestrator.FinalValidate.
func (o *DefaultOrchestrator) FinalValidate(ctx context.Context, results []*StepResult) (*Verdict, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Use Validator agent to review all changes
	validator := NewValidator(o.router.ForValidation(), o.workdir)
	verdict, err := validator.FinalValidate(ctx, results, o.requirements)
	if err != nil {
		return nil, fmt.Errorf("final validator: %w", err)
	}

	return verdict, nil
}

// Iterate implements Orchestrator.Iterate.
func (o *DefaultOrchestrator) Iterate(ctx context.Context, step *Step, verdict *Verdict) (*StepResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Use Worker agent to fix issues
	worker := NewWorker(o.router.ForExecution(), o.workdir)
	result, err := worker.Iterate(ctx, step, verdict)
	if err != nil {
		return nil, fmt.Errorf("worker iterate: %w", err)
	}

	// Store result
	if step.Number > 0 && step.Number <= len(o.results) {
		o.results[step.Number-1] = result
	}

	return result, nil
}

// ExecuteWorkflow runs the complete multi-agent workflow.
func (o *DefaultOrchestrator) ExecuteWorkflow(ctx context.Context, task *Task) (*WorkflowResult, error) {
	// Phase 0: Architect analyzes requirements
	reqs, err := o.Analyze(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}

	// Phase 0b: Architect creates step documents
	steps, err := o.Plan(ctx, reqs)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}

	var allResults []*StepResult

	// Phase 1-3: For each step
	for _, step := range steps {
		stepCopy := step // Avoid closure issues

		// Check dependencies (simplified - assumes sequential)
		// TODO: Implement parallel execution for independent steps

		// Execute step with validation loop
		var result *StepResult
		var verdict *Verdict

		for iteration := 1; iteration <= o.config.MaxValidationRetries; iteration++ {
			// Execute
			if iteration == 1 {
				result, err = o.Execute(ctx, &stepCopy)
			} else {
				result, err = o.Iterate(ctx, &stepCopy, verdict)
			}
			if err != nil {
				return nil, fmt.Errorf("execute step %d: %w", stepCopy.Number, err)
			}

			result.Iteration = iteration

			// Validate
			verdict, err = o.Validate(ctx, &stepCopy, result)
			if err != nil {
				return nil, fmt.Errorf("validate step %d: %w", stepCopy.Number, err)
			}

			if verdict.Status == VerdictPass {
				break
			}

			if iteration == o.config.MaxValidationRetries {
				return nil, fmt.Errorf("step %d failed after %d iterations: %v",
					stepCopy.Number, iteration, verdict.Reasons)
			}
		}

		allResults = append(allResults, result)
	}

	// Phase 4: Final validation
	finalVerdict, err := o.FinalValidate(ctx, allResults)
	if err != nil {
		return nil, fmt.Errorf("final validate: %w", err)
	}

	if finalVerdict.Status == VerdictReject {
		return nil, fmt.Errorf("final validation failed: %v", finalVerdict.Reasons)
	}

	// Phase 5: Write summary
	summary := generateSummary(reqs, steps, allResults, finalVerdict)
	if err := o.workdir.WriteSummary(summary); err != nil {
		return nil, fmt.Errorf("write summary: %w", err)
	}

	return &WorkflowResult{
		Requirements:    reqs,
		Steps:           steps,
		Results:         allResults,
		FinalVerdict:    finalVerdict,
		SummaryPath:     o.workdir.SummaryPath(),
		WorkdirPath:     o.workdir.Path(),
	}, nil
}

// WorkflowResult contains the complete workflow output.
type WorkflowResult struct {
	Requirements    *Requirements
	Steps           []Step
	Results         []*StepResult
	FinalVerdict    *Verdict
	SummaryPath     string
	WorkdirPath     string
}

// sanitizeTaskName creates a valid directory name from task description.
func sanitizeTaskName(desc string) string {
	// Take first 50 chars
	if len(desc) > 50 {
		desc = desc[:50]
	}

	// Replace invalid chars
	var result []byte
	for i := 0; i < len(desc); i++ {
		c := desc[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else if c == ' ' {
			result = append(result, '-')
		}
	}

	if len(result) == 0 {
		return "task"
	}

	return string(result)
}

// generateSummary creates the summary.md content.
func generateSummary(reqs *Requirements, steps []Step, results []*StepResult, verdict *Verdict) string {
	var sb stringBuilder

	sb.WriteString("# Summary\n\n")

	// Build status
	if verdict.BuildPassed {
		sb.WriteString("## Build: PASS\n\n")
	} else {
		sb.WriteString("## Build: FAIL\n\n")
	}

	// Requirements table
	sb.WriteString("## Requirements\n")
	sb.WriteString("| REQ | Status | Implemented In |\n")
	sb.WriteString("|-----|--------|----------------|\n")
	for _, req := range reqs.Items {
		status := "?"
		if verdict.RequirementStatus != nil {
			if passed, ok := verdict.RequirementStatus[req.ID]; ok {
				if passed {
					status = "✓"
				} else {
					status = "✗"
				}
			}
		}
		sb.WriteString("| " + req.ID + " | " + status + " | - |\n")
	}
	sb.WriteString("\n")

	// Steps table
	sb.WriteString("## Steps\n")
	sb.WriteString("| Step | Iterations | Key Decisions |\n")
	sb.WriteString("|------|------------|---------------|\n")
	for i, step := range steps {
		iterations := "1"
		if i < len(results) && results[i] != nil {
			iterations = itoa(results[i].Iteration)
		}
		sb.WriteString("| " + itoa(step.Number) + " | " + iterations + " | " + step.Title + " |\n")
	}
	sb.WriteString("\n")

	// Files changed
	sb.WriteString("## Files Changed\n")
	seen := make(map[string]bool)
	for _, result := range results {
		if result == nil {
			continue
		}
		for _, change := range result.Changes {
			if !seen[change.Path] {
				sb.WriteString("- " + change.Path + "\n")
				seen[change.Path] = true
			}
		}
	}

	return sb.String()
}

// stringBuilder is a simple string builder.
type stringBuilder struct {
	data []byte
}

func (sb *stringBuilder) WriteString(s string) {
	sb.data = append(sb.data, s...)
}

func (sb *stringBuilder) String() string {
	return string(sb.data)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
