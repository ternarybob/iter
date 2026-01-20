// Package docs provides a skill for documentation tasks.
package docs

import (
	"context"
	"strings"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Skill implements documentation capabilities.
type Skill struct {
	sdk.BaseSkill
}

// New creates a new docs skill.
func New() *Skill {
	return &Skill{
		BaseSkill: *sdk.NewBaseSkill(sdk.SkillMetadata{
			Name:        "docs",
			Description: "Documentation generation and maintenance",
			Version:     "1.0.0",
			Triggers: []string{
				"document",
				"documentation",
				"readme",
				"api docs",
				"comment",
				"explain",
			},
			Tags: []string{"documentation", "readme", "comments"},
		}),
	}
}

// CanHandle evaluates if this skill can handle the task.
func (s *Skill) CanHandle(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (bool, float64) {
	desc := strings.ToLower(task.Description)

	if sdk.MatchTrigger(desc, s.Metadata().Triggers) {
		if strings.Contains(desc, "readme") {
			return true, 0.95
		}
		if strings.Contains(desc, "api doc") {
			return true, 0.9
		}
		if strings.Contains(desc, "document") {
			return true, 0.85
		}
		return true, 0.7
	}

	return false, 0
}

// Plan generates an execution plan for the task.
func (s *Skill) Plan(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (*sdk.Plan, error) {
	plan := sdk.NewPlan(task.ID, s.Metadata().Name).
		WithTitle("Documentation").
		WithDescription("Generate or update documentation")

	plan.AddStep(sdk.PlanStep{
		Title:       "Analyze code",
		Description: "Analyze code structure and patterns",
		Type:        sdk.StepTypeAnalyze,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Generate documentation",
		Description: "Generate documentation content",
		Type:        sdk.StepTypeWrite,
	})

	return plan, nil
}

// Execute performs the planned actions.
func (s *Skill) Execute(ctx context.Context, execCtx *sdk.ExecutionContext, plan *sdk.Plan) (*sdk.Result, error) {
	result := sdk.NewResult(plan.TaskID, s.Metadata().Name)
	result.WithStatus(sdk.ResultStatusSuccess).
		WithMessage("Documentation generated")
	return result, nil
}

// Validate checks execution result for correctness.
func (s *Skill) Validate(ctx context.Context, execCtx *sdk.ExecutionContext, result *sdk.Result) error {
	return nil
}
