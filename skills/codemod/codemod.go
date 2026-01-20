// Package codemod provides a skill for modifying existing code.
package codemod

import (
	"context"
	"strings"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Skill implements code modification capabilities.
type Skill struct {
	sdk.BaseSkill
}

// New creates a new codemod skill.
func New() *Skill {
	return &Skill{
		BaseSkill: *sdk.NewBaseSkill(sdk.SkillMetadata{
			Name:        "codemod",
			Description: "Modify existing code based on requirements",
			Version:     "1.0.0",
			Triggers: []string{
				"fix",
				"refactor",
				"modify",
				"update",
				"change",
				"implement",
				"add feature",
				"patch",
				"edit",
				"rewrite",
			},
			Tags: []string{"code", "modification", "refactor"},
		}),
	}
}

// CanHandle evaluates if this skill can handle the task.
func (s *Skill) CanHandle(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (bool, float64) {
	desc := strings.ToLower(task.Description)

	// Check triggers
	if sdk.MatchTrigger(desc, s.Metadata().Triggers) {
		// Higher confidence for specific keywords
		if strings.Contains(desc, "fix") || strings.Contains(desc, "bug") {
			return true, 0.9
		}
		if strings.Contains(desc, "implement") || strings.Contains(desc, "add") {
			return true, 0.85
		}
		if strings.Contains(desc, "refactor") {
			return true, 0.8
		}
		return true, 0.7
	}

	// Generic code-related tasks
	if strings.Contains(desc, "code") || strings.Contains(desc, "function") ||
		strings.Contains(desc, "method") || strings.Contains(desc, "class") {
		return true, 0.5
	}

	return false, 0
}

// Plan generates an execution plan for the task.
func (s *Skill) Plan(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (*sdk.Plan, error) {
	plan := sdk.NewPlan(task.ID, s.Metadata().Name).
		WithTitle("Code Modification").
		WithDescription("Modify existing code based on requirements")

	// Add requirements
	plan.AddRequirement(sdk.Requirement{
		ID:          "REQ-1",
		Description: task.Description,
	})

	// Add steps
	plan.AddStep(sdk.PlanStep{
		Title:       "Analyze context",
		Description: "Search codebase for relevant context",
		Type:        sdk.StepTypeAnalyze,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Generate changes",
		Description: "Generate code modifications via LLM",
		Type:        sdk.StepTypeWrite,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Verify build",
		Description: "Verify the build passes",
		Type:        sdk.StepTypeValidate,
	})

	return plan, nil
}

// Execute performs the planned actions.
func (s *Skill) Execute(ctx context.Context, execCtx *sdk.ExecutionContext, plan *sdk.Plan) (*sdk.Result, error) {
	result := sdk.NewResult(plan.TaskID, s.Metadata().Name)

	// In a real implementation, this would:
	// 1. Search the codebase for relevant context
	// 2. Call the LLM to generate code changes
	// 3. Apply the changes to files
	// 4. Run build verification

	// For now, we return a placeholder result
	result.WithStatus(sdk.ResultStatusSuccess).
		WithMessage("Code modification skill executed successfully")

	return result, nil
}

// Validate checks execution result for correctness.
func (s *Skill) Validate(ctx context.Context, execCtx *sdk.ExecutionContext, result *sdk.Result) error {
	// In a real implementation, validate that:
	// - Build passes
	// - Tests pass
	// - Changes are syntactically valid
	return nil
}
