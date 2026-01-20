package sdk

import (
	"context"
)

// HookType identifies lifecycle hook points.
type HookType string

const (
	// HookTypePreIteration runs before each iteration.
	HookTypePreIteration HookType = "pre_iteration"

	// HookTypePostIteration runs after each iteration.
	HookTypePostIteration HookType = "post_iteration"

	// HookTypePrePlan runs before planning.
	HookTypePrePlan HookType = "pre_plan"

	// HookTypePostPlan runs after planning.
	HookTypePostPlan HookType = "post_plan"

	// HookTypePreExecute runs before execution.
	HookTypePreExecute HookType = "pre_execute"

	// HookTypePostExecute runs after execution.
	HookTypePostExecute HookType = "post_execute"

	// HookTypePreValidate runs before validation.
	HookTypePreValidate HookType = "pre_validate"

	// HookTypePostValidate runs after validation.
	HookTypePostValidate HookType = "post_validate"

	// HookTypeOnError runs when an error occurs.
	HookTypeOnError HookType = "on_error"

	// HookTypeOnExit runs when the agent is about to exit.
	HookTypeOnExit HookType = "on_exit"
)

// HookContext provides information to hooks.
type HookContext struct {
	// Type is the hook type.
	Type HookType

	// Iteration is the current iteration number.
	Iteration int

	// Task is the current task (if applicable).
	Task *Task

	// Plan is the current plan (if applicable).
	Plan *Plan

	// Result is the current result (if applicable).
	Result *Result

	// Error is any error that occurred.
	Error error

	// ExecCtx is the execution context.
	ExecCtx *ExecutionContext
}

// Hook is a lifecycle callback function.
type Hook func(ctx context.Context, hctx *HookContext) error

// HookRegistry manages lifecycle hooks.
type HookRegistry struct {
	hooks map[HookType][]Hook
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks: make(map[HookType][]Hook),
	}
}

// Register adds a hook for the specified type.
func (r *HookRegistry) Register(hookType HookType, hook Hook) {
	r.hooks[hookType] = append(r.hooks[hookType], hook)
}

// Run executes all hooks of the specified type.
func (r *HookRegistry) Run(ctx context.Context, hctx *HookContext) error {
	hooks := r.hooks[hctx.Type]
	for _, hook := range hooks {
		if err := hook(ctx, hctx); err != nil {
			return err
		}
	}
	return nil
}

// Clear removes all hooks of the specified type.
func (r *HookRegistry) Clear(hookType HookType) {
	delete(r.hooks, hookType)
}

// ClearAll removes all registered hooks.
func (r *HookRegistry) ClearAll() {
	r.hooks = make(map[HookType][]Hook)
}

// Count returns the number of hooks registered for a type.
func (r *HookRegistry) Count(hookType HookType) int {
	return len(r.hooks[hookType])
}

// Hooks returns a copy of hooks for a type.
func (r *HookRegistry) Hooks(hookType HookType) []Hook {
	hooks := r.hooks[hookType]
	if hooks == nil {
		return nil
	}
	result := make([]Hook, len(hooks))
	copy(result, hooks)
	return result
}
