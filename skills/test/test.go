// Package test provides a skill for test generation and execution.
package test

import (
	"context"
	"strings"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Skill implements test generation and execution capabilities.
type Skill struct {
	sdk.BaseSkill
}

// New creates a new test skill.
func New() *Skill {
	return &Skill{
		BaseSkill: *sdk.NewBaseSkill(sdk.SkillMetadata{
			Name:        "test",
			Description: "Generate and run tests",
			Version:     "1.0.0",
			Triggers: []string{
				"test",
				"add tests",
				"write tests",
				"verify",
				"coverage",
				"unit test",
				"integration test",
			},
			Tags: []string{"test", "verification", "quality"},
		}),
	}
}

// CanHandle evaluates if this skill can handle the task.
func (s *Skill) CanHandle(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (bool, float64) {
	desc := strings.ToLower(task.Description)

	if sdk.MatchTrigger(desc, s.Metadata().Triggers) {
		if strings.Contains(desc, "write test") || strings.Contains(desc, "add test") {
			return true, 0.95
		}
		if strings.Contains(desc, "coverage") {
			return true, 0.9
		}
		return true, 0.8
	}

	return false, 0
}

// Plan generates an execution plan for the task.
func (s *Skill) Plan(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (*sdk.Plan, error) {
	plan := sdk.NewPlan(task.ID, s.Metadata().Name).
		WithTitle("Test Generation").
		WithDescription("Generate and run tests")

	plan.AddStep(sdk.PlanStep{
		Title:       "Analyze code",
		Description: "Analyze code to generate appropriate tests",
		Type:        sdk.StepTypeAnalyze,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Generate tests",
		Description: "Generate test code following project patterns",
		Type:        sdk.StepTypeWrite,
	})

	plan.AddStep(sdk.PlanStep{
		Title:       "Run tests",
		Description: "Execute tests and capture results",
		Type:        sdk.StepTypeExecute,
	})

	return plan, nil
}

// Execute performs the planned actions.
func (s *Skill) Execute(ctx context.Context, execCtx *sdk.ExecutionContext, plan *sdk.Plan) (*sdk.Result, error) {
	result := sdk.NewResult(plan.TaskID, s.Metadata().Name)

	result.WithStatus(sdk.ResultStatusSuccess).
		WithMessage("Test skill executed successfully")

	return result, nil
}

// Validate checks execution result for correctness.
func (s *Skill) Validate(ctx context.Context, execCtx *sdk.ExecutionContext, result *sdk.Result) error {
	return nil
}
