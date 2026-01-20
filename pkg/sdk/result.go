package sdk

import (
	"time"
)

// ResultStatus indicates the outcome of skill execution.
type ResultStatus string

const (
	ResultStatusSuccess    ResultStatus = "success"
	ResultStatusPartial    ResultStatus = "partial"
	ResultStatusFailed     ResultStatus = "failed"
	ResultStatusSkipped    ResultStatus = "skipped"
	ResultStatusNeedsInput ResultStatus = "needs_input"
)

// ChangeType indicates the type of file modification.
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeModify ChangeType = "modify"
	ChangeTypeDelete ChangeType = "delete"
	ChangeTypeRename ChangeType = "rename"
)

// Change represents a single file modification.
type Change struct {
	// Type indicates the modification type.
	Type ChangeType `json:"type"`

	// Path is the file path.
	Path string `json:"path"`

	// Before is the previous content (for modify/delete).
	Before string `json:"before,omitempty"`

	// After is the new content (for create/modify).
	After string `json:"after,omitempty"`

	// Diff is the unified diff format.
	Diff string `json:"diff,omitempty"`

	// LineNum is the starting line for partial changes.
	LineNum int `json:"line_num,omitempty"`

	// OldPath is the previous path (for rename).
	OldPath string `json:"old_path,omitempty"`
}

// ResultMetrics captures execution statistics.
type ResultMetrics struct {
	// Duration is how long execution took.
	Duration time.Duration `json:"duration"`

	// TokensUsed is the total token consumption.
	TokensUsed int `json:"tokens_used"`

	// FilesModified is the number of files changed.
	FilesModified int `json:"files_modified"`

	// LinesChanged is the number of lines added/removed.
	LinesChanged int `json:"lines_changed"`

	// TestsRun is the number of tests executed.
	TestsRun int `json:"tests_run,omitempty"`

	// TestsPassed is the number of tests that passed.
	TestsPassed int `json:"tests_passed,omitempty"`

	// LLMCalls is the number of LLM API calls made.
	LLMCalls int `json:"llm_calls"`
}

// CommandOutput captures output from command execution.
type CommandOutput struct {
	// Command is the executed command.
	Command string `json:"command"`

	// Stdout is the standard output.
	Stdout string `json:"stdout,omitempty"`

	// Stderr is the standard error.
	Stderr string `json:"stderr,omitempty"`

	// ExitCode is the command exit code.
	ExitCode int `json:"exit_code"`

	// Duration is how long the command took.
	Duration time.Duration `json:"duration"`

	// LogFile is the path to the full output log.
	LogFile string `json:"log_file,omitempty"`
}

// Result captures the outcome of skill execution.
type Result struct {
	// TaskID links to the source task.
	TaskID string `json:"task_id"`

	// SkillName indicates which skill executed.
	SkillName string `json:"skill_name"`

	// Status indicates the execution outcome.
	Status ResultStatus `json:"status"`

	// Message is a human-readable summary.
	Message string `json:"message"`

	// Changes lists all file modifications made.
	Changes []Change `json:"changes,omitempty"`

	// Outputs captures command execution results.
	Outputs []CommandOutput `json:"outputs,omitempty"`

	// Metrics contains execution statistics.
	Metrics ResultMetrics `json:"metrics"`

	// Error contains error details if execution failed.
	Error error `json:"-"`

	// ErrorMessage is the serialized error message.
	ErrorMessage string `json:"error,omitempty"`

	// ExitSignal indicates the agent should stop iterating.
	ExitSignal bool `json:"exit_signal,omitempty"`

	// NextTasks suggests follow-up tasks.
	NextTasks []*Task `json:"next_tasks,omitempty"`

	// Artifacts contains paths to generated artifacts.
	Artifacts map[string]string `json:"artifacts,omitempty"`
}

// NewResult creates a new result for a task and skill.
func NewResult(taskID, skillName string) *Result {
	return &Result{
		TaskID:    taskID,
		SkillName: skillName,
		Status:    ResultStatusSuccess,
		Artifacts: make(map[string]string),
	}
}

// WithStatus sets the result status.
func (r *Result) WithStatus(status ResultStatus) *Result {
	r.Status = status
	return r
}

// WithMessage sets the result message.
func (r *Result) WithMessage(message string) *Result {
	r.Message = message
	return r
}

// WithError sets the result as failed with an error.
func (r *Result) WithError(err error) *Result {
	r.Status = ResultStatusFailed
	r.Error = err
	if err != nil {
		r.ErrorMessage = err.Error()
	}
	return r
}

// WithExitSignal marks that the agent should stop.
func (r *Result) WithExitSignal() *Result {
	r.ExitSignal = true
	return r
}

// AddChange adds a file change to the result.
func (r *Result) AddChange(change Change) *Result {
	r.Changes = append(r.Changes, change)
	r.Metrics.FilesModified++
	return r
}

// AddOutput adds a command output to the result.
func (r *Result) AddOutput(output CommandOutput) *Result {
	r.Outputs = append(r.Outputs, output)
	return r
}

// AddNextTask suggests a follow-up task.
func (r *Result) AddNextTask(task *Task) *Result {
	r.NextTasks = append(r.NextTasks, task)
	return r
}

// SetArtifact sets an artifact path.
func (r *Result) SetArtifact(name, path string) *Result {
	if r.Artifacts == nil {
		r.Artifacts = make(map[string]string)
	}
	r.Artifacts[name] = path
	return r
}

// IsSuccess returns true if the result indicates success.
func (r *Result) IsSuccess() bool {
	return r.Status == ResultStatusSuccess
}

// IsFailure returns true if the result indicates failure.
func (r *Result) IsFailure() bool {
	return r.Status == ResultStatusFailed
}
