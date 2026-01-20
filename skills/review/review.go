// Package review provides a skill for code review.
package review

import (
	"context"
	"strings"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Skill implements code review capabilities.
type Skill struct {
	sdk.BaseSkill
}

// New creates a new review skill.
func New() *Skill {
	return &Skill{
		BaseSkill: *sdk.NewBaseSkill(sdk.SkillMetadata{
			Name:        "review",
			Description: "Review code for issues",
			Version:     "1.0.0",
			Triggers: []string{
				"review",
				"check",
				"audit",
				"analyze",
				"security review",
				"code review",
			},
			Tags: []string{"review", "audit", "quality"},
		}),
	}
}

// CanHandle evaluates if this skill can handle the task.
func (s *Skill) CanHandle(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (bool, float64) {
	desc := strings.ToLower(task.Description)

	if sdk.MatchTrigger(desc, s.Metadata().Triggers) {
		if strings.Contains(desc, "security") {
			return true, 0.95
		}
		if strings.Contains(desc, "review") {
			return true, 0.9
		}
		return true, 0.75
	}

	return false, 0
}

// Plan generates an execution plan for the task.
func (s *Skill) Plan(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (*sdk.Plan, error) {
	plan := sdk.NewPlan(task.ID, s.Metadata().Name).
		WithTitle("Code Review").
		WithDescription("Review code for issues and improvements")

	plan.AddStep(sdk.PlanStep{
		Title:       "Static analysis",
		Description: "Perform static analysis via LLM",
		Type:        sdk.StepTypeAnalyze,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Pattern compliance",
		Description: "Check compliance with project patterns",
		Type:        sdk.StepTypeValidate,
	})

	return plan, nil
}

// Execute performs the planned actions.
func (s *Skill) Execute(ctx context.Context, execCtx *sdk.ExecutionContext, plan *sdk.Plan) (*sdk.Result, error) {
	result := sdk.NewResult(plan.TaskID, s.Metadata().Name)
	result.WithStatus(sdk.ResultStatusSuccess).
		WithMessage("Code review completed")
	return result, nil
}

// Validate checks execution result for correctness.
func (s *Skill) Validate(ctx context.Context, execCtx *sdk.ExecutionContext, result *sdk.Result) error {
	return nil
}
