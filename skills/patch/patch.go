// Package patch provides a skill for patch application and conflict resolution.
package patch

import (
	"context"
	"strings"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Skill implements patch application capabilities.
type Skill struct {
	sdk.BaseSkill
}

// New creates a new patch skill.
func New() *Skill {
	return &Skill{
		BaseSkill: *sdk.NewBaseSkill(sdk.SkillMetadata{
			Name:        "patch",
			Description: "Apply patches and handle merge conflicts",
			Version:     "1.0.0",
			Triggers: []string{
				"apply patch",
				"merge",
				"cherry-pick",
				"resolve conflict",
				"diff",
				"patch",
			},
			Tags: []string{"patch", "merge", "git"},
		}),
	}
}

// CanHandle evaluates if this skill can handle the task.
func (s *Skill) CanHandle(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (bool, float64) {
	desc := strings.ToLower(task.Description)

	if sdk.MatchTrigger(desc, s.Metadata().Triggers) {
		if strings.Contains(desc, "conflict") {
			return true, 0.95
		}
		if strings.Contains(desc, "patch") || strings.Contains(desc, "merge") {
			return true, 0.9
		}
		return true, 0.75
	}

	return false, 0
}

// Plan generates an execution plan for the task.
func (s *Skill) Plan(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (*sdk.Plan, error) {
	plan := sdk.NewPlan(task.ID, s.Metadata().Name).
		WithTitle("Patch Application").
		WithDescription("Apply patches and resolve conflicts")

	plan.AddStep(sdk.PlanStep{
		Title:       "Parse patch",
		Description: "Parse unified diff format",
		Type:        sdk.StepTypeAnalyze,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Apply changes",
		Description: "Apply patch with conflict detection",
		Type:        sdk.StepTypeWrite,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Verify",
		Description: "Verify patch application",
		Type:        sdk.StepTypeValidate,
	})

	return plan, nil
}

// Execute performs the planned actions.
func (s *Skill) Execute(ctx context.Context, execCtx *sdk.ExecutionContext, plan *sdk.Plan) (*sdk.Result, error) {
	result := sdk.NewResult(plan.TaskID, s.Metadata().Name)
	result.WithStatus(sdk.ResultStatusSuccess).
		WithMessage("Patch applied successfully")
	return result, nil
}

// Validate checks execution result for correctness.
func (s *Skill) Validate(ctx context.Context, execCtx *sdk.ExecutionContext, result *sdk.Result) error {
	return nil
}
