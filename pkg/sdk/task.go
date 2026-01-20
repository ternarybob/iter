// Package sdk provides the public interfaces and types for the Iter SDK.
// Skills and agents implement these interfaces to extend functionality.
package sdk

import (
	"time"
)

// TaskType categorizes the nature of work.
type TaskType string

const (
	TaskTypeFix      TaskType = "fix"
	TaskTypeFeature  TaskType = "feature"
	TaskTypeRefactor TaskType = "refactor"
	TaskTypeTest     TaskType = "test"
	TaskTypeDocs     TaskType = "docs"
	TaskTypeDevOps   TaskType = "devops"
	TaskTypeReview   TaskType = "review"
	TaskTypeGeneric  TaskType = "generic"
)

// TaskConstraints defines execution limits for a task.
type TaskConstraints struct {
	// MaxTokens is the token budget for this task.
	MaxTokens int `json:"max_tokens,omitempty"`

	// MaxFiles limits the number of files that can be modified.
	MaxFiles int `json:"max_files,omitempty"`

	// AllowedPaths is a whitelist of paths that can be modified.
	// Empty means all paths are allowed.
	AllowedPaths []string `json:"allowed_paths,omitempty"`

	// DisallowedPaths is a blacklist of paths that cannot be modified.
	DisallowedPaths []string `json:"disallowed_paths,omitempty"`

	// RequireTests indicates tests must be present or generated.
	RequireTests bool `json:"require_tests,omitempty"`

	// RequireReview indicates validation must pass.
	RequireReview bool `json:"require_review,omitempty"`
}

// Task represents a unit of work for the agent.
type Task struct {
	// ID is a unique identifier for this task.
	ID string `json:"id"`

	// Description is a human-readable description of what needs to be done.
	Description string `json:"description"`

	// Priority determines execution order (lower = higher priority).
	Priority int `json:"priority,omitempty"`

	// Type categorizes the task.
	Type TaskType `json:"type,omitempty"`

	// Files lists relevant file paths for this task.
	Files []string `json:"files,omitempty"`

	// Context provides additional structured context.
	Context map[string]any `json:"context,omitempty"`

	// Parent links to the parent task if this is a subtask.
	Parent *Task `json:"-"`

	// ParentID is the ID of the parent task (for serialization).
	ParentID string `json:"parent_id,omitempty"`

	// CreatedAt is when the task was created.
	CreatedAt time.Time `json:"created_at"`

	// Deadline is an optional completion deadline.
	Deadline *time.Time `json:"deadline,omitempty"`

	// Constraints defines execution limits.
	Constraints TaskConstraints `json:"constraints,omitempty"`
}

// NewTask creates a new task with the given description.
func NewTask(description string) *Task {
	return &Task{
		ID:          generateID(),
		Description: description,
		Type:        TaskTypeGeneric,
		CreatedAt:   time.Now(),
		Context:     make(map[string]any),
	}
}

// WithType sets the task type.
func (t *Task) WithType(taskType TaskType) *Task {
	t.Type = taskType
	return t
}

// WithPriority sets the task priority.
func (t *Task) WithPriority(priority int) *Task {
	t.Priority = priority
	return t
}

// WithFiles sets the relevant files.
func (t *Task) WithFiles(files ...string) *Task {
	t.Files = files
	return t
}

// WithContext adds context values.
func (t *Task) WithContext(key string, value any) *Task {
	if t.Context == nil {
		t.Context = make(map[string]any)
	}
	t.Context[key] = value
	return t
}

// WithConstraints sets the task constraints.
func (t *Task) WithConstraints(constraints TaskConstraints) *Task {
	t.Constraints = constraints
	return t
}

// WithDeadline sets the task deadline.
func (t *Task) WithDeadline(deadline time.Time) *Task {
	t.Deadline = &deadline
	return t
}
