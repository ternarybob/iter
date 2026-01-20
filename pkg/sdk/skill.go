package sdk

import (
	"context"
)

// SkillMetadata provides identification and documentation for a skill.
type SkillMetadata struct {
	// Name is the unique identifier for this skill.
	Name string `json:"name"`

	// Description explains what this skill does.
	Description string `json:"description"`

	// Version is the semantic version.
	Version string `json:"version"`

	// Triggers are patterns that activate this skill.
	// Supports glob patterns and regex (prefix with "re:").
	Triggers []string `json:"triggers"`

	// RequiredTools lists external tools needed.
	RequiredTools []string `json:"required_tools,omitempty"`

	// Tags provide categorization.
	Tags []string `json:"tags,omitempty"`

	// Author is the skill author.
	Author string `json:"author,omitempty"`
}

// Skill defines the contract for all Iter skills.
// Skills are the primary extension point for adding capabilities.
type Skill interface {
	// Metadata returns skill identification and documentation.
	Metadata() SkillMetadata

	// CanHandle evaluates if this skill can handle the task.
	// Returns confidence score 0.0-1.0.
	// The highest confidence skill is selected.
	CanHandle(ctx context.Context, execCtx *ExecutionContext, task *Task) (bool, float64)

	// Plan generates an execution plan for the task.
	// Called only if CanHandle returns sufficient confidence.
	Plan(ctx context.Context, execCtx *ExecutionContext, task *Task) (*Plan, error)

	// Execute performs the planned actions.
	// Should be idempotent where possible.
	Execute(ctx context.Context, execCtx *ExecutionContext, plan *Plan) (*Result, error)

	// Validate checks execution result for correctness.
	// Return nil to skip validation.
	Validate(ctx context.Context, execCtx *ExecutionContext, result *Result) error
}

// SkillFunc is a simplified skill using functions.
type SkillFunc struct {
	meta     SkillMetadata
	canFunc  func(ctx context.Context, execCtx *ExecutionContext, task *Task) (bool, float64)
	planFunc func(ctx context.Context, execCtx *ExecutionContext, task *Task) (*Plan, error)
	execFunc func(ctx context.Context, execCtx *ExecutionContext, plan *Plan) (*Result, error)
	valFunc  func(ctx context.Context, execCtx *ExecutionContext, result *Result) error
}

// NewSkillFunc creates a functional skill.
func NewSkillFunc(meta SkillMetadata) *SkillFunc {
	return &SkillFunc{meta: meta}
}

// Metadata returns the skill metadata.
func (s *SkillFunc) Metadata() SkillMetadata {
	return s.meta
}

// OnCanHandle sets the can-handle function.
func (s *SkillFunc) OnCanHandle(fn func(ctx context.Context, execCtx *ExecutionContext, task *Task) (bool, float64)) *SkillFunc {
	s.canFunc = fn
	return s
}

// OnPlan sets the planning function.
func (s *SkillFunc) OnPlan(fn func(ctx context.Context, execCtx *ExecutionContext, task *Task) (*Plan, error)) *SkillFunc {
	s.planFunc = fn
	return s
}

// OnExecute sets the execution function.
func (s *SkillFunc) OnExecute(fn func(ctx context.Context, execCtx *ExecutionContext, plan *Plan) (*Result, error)) *SkillFunc {
	s.execFunc = fn
	return s
}

// OnValidate sets the validation function.
func (s *SkillFunc) OnValidate(fn func(ctx context.Context, execCtx *ExecutionContext, result *Result) error) *SkillFunc {
	s.valFunc = fn
	return s
}

// CanHandle evaluates if this skill can handle the task.
func (s *SkillFunc) CanHandle(ctx context.Context, execCtx *ExecutionContext, task *Task) (bool, float64) {
	if s.canFunc == nil {
		return false, 0
	}
	return s.canFunc(ctx, execCtx, task)
}

// Plan generates an execution plan for the task.
func (s *SkillFunc) Plan(ctx context.Context, execCtx *ExecutionContext, task *Task) (*Plan, error) {
	if s.planFunc == nil {
		// Return a simple plan with no steps
		return NewPlan(task.ID, s.meta.Name).WithTitle("No-op plan"), nil
	}
	return s.planFunc(ctx, execCtx, task)
}

// Execute performs the planned actions.
func (s *SkillFunc) Execute(ctx context.Context, execCtx *ExecutionContext, plan *Plan) (*Result, error) {
	if s.execFunc == nil {
		return NewResult(plan.TaskID, s.meta.Name).
			WithStatus(ResultStatusSkipped).
			WithMessage("No execution function defined"), nil
	}
	return s.execFunc(ctx, execCtx, plan)
}

// Validate checks execution result for correctness.
func (s *SkillFunc) Validate(ctx context.Context, execCtx *ExecutionContext, result *Result) error {
	if s.valFunc == nil {
		return nil
	}
	return s.valFunc(ctx, execCtx, result)
}

// BaseSkill provides a base implementation for skills.
type BaseSkill struct {
	meta SkillMetadata
}

// NewBaseSkill creates a new base skill.
func NewBaseSkill(meta SkillMetadata) *BaseSkill {
	return &BaseSkill{meta: meta}
}

// Metadata returns the skill metadata.
func (s *BaseSkill) Metadata() SkillMetadata {
	return s.meta
}

// CanHandle returns false by default.
func (s *BaseSkill) CanHandle(ctx context.Context, execCtx *ExecutionContext, task *Task) (bool, float64) {
	return false, 0
}

// Plan returns an empty plan by default.
func (s *BaseSkill) Plan(ctx context.Context, execCtx *ExecutionContext, task *Task) (*Plan, error) {
	return NewPlan(task.ID, s.meta.Name), nil
}

// Execute returns a skipped result by default.
func (s *BaseSkill) Execute(ctx context.Context, execCtx *ExecutionContext, plan *Plan) (*Result, error) {
	return NewResult(plan.TaskID, s.meta.Name).
		WithStatus(ResultStatusSkipped).
		WithMessage("Not implemented"), nil
}

// Validate returns nil by default.
func (s *BaseSkill) Validate(ctx context.Context, execCtx *ExecutionContext, result *Result) error {
	return nil
}

// MatchTrigger checks if a text matches skill triggers.
func MatchTrigger(text string, triggers []string) bool {
	textLower := toLower(text)
	for _, trigger := range triggers {
		if len(trigger) > 3 && trigger[:3] == "re:" {
			// Regex trigger
			if matchRegex(trigger[3:], textLower) {
				return true
			}
		} else {
			// Simple substring match
			if contains(textLower, toLower(trigger)) {
				return true
			}
		}
	}
	return false
}

// toLower converts string to lowercase.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// matchRegex performs a simple regex match (basic implementation).
// For full regex support, use the regexp package in actual implementation.
func matchRegex(pattern, text string) bool {
	// Simplified: treat as substring match for now
	// Real implementation would use regexp.MatchString
	return contains(text, pattern)
}
