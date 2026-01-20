package sdk

import (
	"time"
)

// StepType indicates the nature of a plan step.
type StepType string

const (
	StepTypeRead    StepType = "read"
	StepTypeWrite   StepType = "write"
	StepTypeDelete  StepType = "delete"
	StepTypeExecute StepType = "execute"
	StepTypeValidate StepType = "validate"
	StepTypeAnalyze StepType = "analyze"
)

// PlanStep represents a single step in an execution plan.
type PlanStep struct {
	// ID is a unique identifier for this step.
	ID string `json:"id"`

	// Number is the step sequence number.
	Number int `json:"number"`

	// Title is a short description of the step.
	Title string `json:"title"`

	// Description provides detailed information about the step.
	Description string `json:"description"`

	// Type indicates the step nature.
	Type StepType `json:"type"`

	// Dependencies lists step IDs that must complete first.
	Dependencies []string `json:"dependencies,omitempty"`

	// Files lists files this step will touch.
	Files []string `json:"files,omitempty"`

	// Commands lists shell commands to execute.
	Commands []string `json:"commands,omitempty"`

	// Inputs contains data needed for this step.
	Inputs map[string]any `json:"inputs,omitempty"`

	// ExpectedOutputs describes what this step should produce.
	ExpectedOutputs []string `json:"expected_outputs,omitempty"`

	// Validation contains criteria for step success.
	Validation []string `json:"validation,omitempty"`

	// EstimatedTokens is the expected token consumption.
	EstimatedTokens int `json:"estimated_tokens,omitempty"`

	// Parallel indicates if this step can run in parallel with others.
	Parallel bool `json:"parallel,omitempty"`
}

// Plan represents an execution plan for a task.
type Plan struct {
	// ID is a unique identifier for this plan.
	ID string `json:"id"`

	// TaskID links to the source task.
	TaskID string `json:"task_id"`

	// SkillName indicates which skill created this plan.
	SkillName string `json:"skill_name"`

	// Title is a short description of the plan.
	Title string `json:"title"`

	// Description provides an overview of the approach.
	Description string `json:"description"`

	// Steps are the ordered execution steps.
	Steps []PlanStep `json:"steps"`

	// Requirements lists extracted requirements.
	Requirements []Requirement `json:"requirements,omitempty"`

	// Context contains additional planning data.
	Context map[string]any `json:"context,omitempty"`

	// CreatedAt is when the plan was created.
	CreatedAt time.Time `json:"created_at"`

	// EstimatedDuration is the expected execution time.
	EstimatedDuration time.Duration `json:"estimated_duration,omitempty"`

	// EstimatedTokens is the total expected token consumption.
	EstimatedTokens int `json:"estimated_tokens,omitempty"`
}

// Requirement represents a single requirement extracted from a task.
type Requirement struct {
	// ID is a unique identifier (e.g., REQ-1).
	ID string `json:"id"`

	// Description describes what must be accomplished.
	Description string `json:"description"`

	// Priority indicates importance.
	Priority int `json:"priority,omitempty"`

	// Source is where this requirement came from.
	Source string `json:"source,omitempty"`

	// AcceptanceCriteria lists verifiable conditions.
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
}

// NewPlan creates a new execution plan.
func NewPlan(taskID, skillName string) *Plan {
	return &Plan{
		ID:        generateID(),
		TaskID:    taskID,
		SkillName: skillName,
		CreatedAt: time.Now(),
		Context:   make(map[string]any),
	}
}

// WithTitle sets the plan title.
func (p *Plan) WithTitle(title string) *Plan {
	p.Title = title
	return p
}

// WithDescription sets the plan description.
func (p *Plan) WithDescription(description string) *Plan {
	p.Description = description
	return p
}

// AddStep adds a step to the plan.
func (p *Plan) AddStep(step PlanStep) *Plan {
	step.Number = len(p.Steps) + 1
	if step.ID == "" {
		step.ID = generateID()
	}
	p.Steps = append(p.Steps, step)
	return p
}

// AddRequirement adds a requirement to the plan.
func (p *Plan) AddRequirement(req Requirement) *Plan {
	if req.ID == "" {
		req.ID = generateRequirementID(len(p.Requirements) + 1)
	}
	p.Requirements = append(p.Requirements, req)
	return p
}

// GetStep returns a step by ID.
func (p *Plan) GetStep(id string) *PlanStep {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			return &p.Steps[i]
		}
	}
	return nil
}

// GetStepByNumber returns a step by number.
func (p *Plan) GetStepByNumber(number int) *PlanStep {
	for i := range p.Steps {
		if p.Steps[i].Number == number {
			return &p.Steps[i]
		}
	}
	return nil
}

// GetParallelGroups returns groups of steps that can execute in parallel.
func (p *Plan) GetParallelGroups() [][]PlanStep {
	if len(p.Steps) == 0 {
		return nil
	}

	// Build dependency graph
	completed := make(map[string]bool)
	remaining := make(map[string]PlanStep)
	for _, step := range p.Steps {
		remaining[step.ID] = step
	}

	var groups [][]PlanStep

	for len(remaining) > 0 {
		var group []PlanStep

		// Find all steps whose dependencies are satisfied
		for id, step := range remaining {
			canRun := true
			for _, depID := range step.Dependencies {
				if !completed[depID] {
					canRun = false
					break
				}
			}
			if canRun {
				group = append(group, step)
				delete(remaining, id)
			}
		}

		if len(group) == 0 {
			// Circular dependency or error - just add remaining
			for _, step := range remaining {
				group = append(group, step)
			}
			remaining = make(map[string]PlanStep)
		}

		// Mark group as completed
		for _, step := range group {
			completed[step.ID] = true
		}

		groups = append(groups, group)
	}

	return groups
}

// generateRequirementID creates a requirement ID.
func generateRequirementID(n int) string {
	return "REQ-" + itoa(n)
}

// itoa converts int to string without fmt package.
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
