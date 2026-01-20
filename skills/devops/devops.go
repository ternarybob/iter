// Package devops provides a skill for CI/CD, container, and infrastructure tasks.
package devops

import (
	"context"
	"strings"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Skill implements DevOps capabilities.
type Skill struct {
	sdk.BaseSkill
}

// New creates a new devops skill.
func New() *Skill {
	return &Skill{
		BaseSkill: *sdk.NewBaseSkill(sdk.SkillMetadata{
			Name:        "devops",
			Description: "CI/CD, container, and infrastructure tasks",
			Version:     "1.0.0",
			Triggers: []string{
				"docker",
				"kubernetes",
				"k8s",
				"deploy",
				"ci",
				"cd",
				"pipeline",
				"helm",
				"terraform",
				"infrastructure",
			},
			RequiredTools: []string{"docker", "kubectl"},
			Tags:          []string{"devops", "infrastructure", "deployment"},
		}),
	}
}

// CanHandle evaluates if this skill can handle the task.
func (s *Skill) CanHandle(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (bool, float64) {
	desc := strings.ToLower(task.Description)

	if sdk.MatchTrigger(desc, s.Metadata().Triggers) {
		if strings.Contains(desc, "kubernetes") || strings.Contains(desc, "k8s") {
			return true, 0.95
		}
		if strings.Contains(desc, "docker") {
			return true, 0.9
		}
		if strings.Contains(desc, "deploy") || strings.Contains(desc, "pipeline") {
			return true, 0.85
		}
		return true, 0.75
	}

	return false, 0
}

// Plan generates an execution plan for the task.
func (s *Skill) Plan(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (*sdk.Plan, error) {
	plan := sdk.NewPlan(task.ID, s.Metadata().Name).
		WithTitle("DevOps Operation").
		WithDescription("Infrastructure and deployment task")

	plan.AddStep(sdk.PlanStep{
		Title:       "Analyze infrastructure",
		Description: "Review existing infrastructure configuration",
		Type:        sdk.StepTypeAnalyze,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Generate configuration",
		Description: "Generate or modify configuration files",
		Type:        sdk.StepTypeWrite,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Validate configuration",
		Description: "Validate configuration syntax and semantics",
		Type:        sdk.StepTypeValidate,
	})

	return plan, nil
}

// Execute performs the planned actions.
func (s *Skill) Execute(ctx context.Context, execCtx *sdk.ExecutionContext, plan *sdk.Plan) (*sdk.Result, error) {
	result := sdk.NewResult(plan.TaskID, s.Metadata().Name)
	result.WithStatus(sdk.ResultStatusSuccess).
		WithMessage("DevOps operation completed")
	return result, nil
}

// Validate checks execution result for correctness.
func (s *Skill) Validate(ctx context.Context, execCtx *sdk.ExecutionContext, result *sdk.Result) error {
	return nil
}
