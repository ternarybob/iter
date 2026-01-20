package sdk

import (
	"errors"
	"testing"
)

func TestNewResult(t *testing.T) {
	result := NewResult("task-123", "codemod")

	if result.TaskID != "task-123" {
		t.Errorf("expected TaskID 'task-123', got %q", result.TaskID)
	}

	if result.SkillName != "codemod" {
		t.Errorf("expected SkillName 'codemod', got %q", result.SkillName)
	}

	if result.Status != ResultStatusSuccess {
		t.Errorf("expected default status %q, got %q", ResultStatusSuccess, result.Status)
	}
}

func TestResultWithStatus(t *testing.T) {
	result := NewResult("task", "skill").WithStatus(ResultStatusPartial)

	if result.Status != ResultStatusPartial {
		t.Errorf("expected status %q, got %q", ResultStatusPartial, result.Status)
	}
}

func TestResultWithMessage(t *testing.T) {
	result := NewResult("task", "skill").WithMessage("completed successfully")

	if result.Message != "completed successfully" {
		t.Errorf("expected message 'completed successfully', got %q", result.Message)
	}
}

func TestResultWithError(t *testing.T) {
	err := errors.New("something went wrong")
	result := NewResult("task", "skill").WithError(err)

	if result.Status != ResultStatusFailed {
		t.Errorf("expected status %q, got %q", ResultStatusFailed, result.Status)
	}

	if result.Error != err {
		t.Errorf("expected error %v, got %v", err, result.Error)
	}

	if result.ErrorMessage != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got %q", result.ErrorMessage)
	}
}

func TestResultWithExitSignal(t *testing.T) {
	result := NewResult("task", "skill").WithExitSignal()

	if !result.ExitSignal {
		t.Error("expected ExitSignal=true")
	}
}

func TestResultAddChange(t *testing.T) {
	result := NewResult("task", "skill").
		AddChange(Change{Type: ChangeTypeCreate, Path: "file1.go"}).
		AddChange(Change{Type: ChangeTypeModify, Path: "file2.go"})

	if len(result.Changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(result.Changes))
	}

	if result.Metrics.FilesModified != 2 {
		t.Errorf("expected FilesModified=2, got %d", result.Metrics.FilesModified)
	}
}

func TestResultAddOutput(t *testing.T) {
	output := CommandOutput{
		Command:  "go build ./...",
		ExitCode: 0,
	}
	result := NewResult("task", "skill").AddOutput(output)

	if len(result.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(result.Outputs))
	}

	if result.Outputs[0].Command != "go build ./..." {
		t.Errorf("expected command 'go build ./...', got %q", result.Outputs[0].Command)
	}
}

func TestResultAddNextTask(t *testing.T) {
	nextTask := NewTask("follow-up task")
	result := NewResult("task", "skill").AddNextTask(nextTask)

	if len(result.NextTasks) != 1 {
		t.Errorf("expected 1 next task, got %d", len(result.NextTasks))
	}

	if result.NextTasks[0].Description != "follow-up task" {
		t.Errorf("expected next task 'follow-up task', got %q", result.NextTasks[0].Description)
	}
}

func TestResultSetArtifact(t *testing.T) {
	result := NewResult("task", "skill").
		SetArtifact("summary", "/path/to/summary.md").
		SetArtifact("log", "/path/to/build.log")

	if len(result.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(result.Artifacts))
	}

	if result.Artifacts["summary"] != "/path/to/summary.md" {
		t.Errorf("expected summary path, got %q", result.Artifacts["summary"])
	}
}

func TestResultIsSuccess(t *testing.T) {
	success := NewResult("task", "skill").WithStatus(ResultStatusSuccess)
	failed := NewResult("task", "skill").WithStatus(ResultStatusFailed)

	if !success.IsSuccess() {
		t.Error("expected IsSuccess()=true for success status")
	}

	if success.IsFailure() {
		t.Error("expected IsFailure()=false for success status")
	}

	if failed.IsSuccess() {
		t.Error("expected IsSuccess()=false for failed status")
	}

	if !failed.IsFailure() {
		t.Error("expected IsFailure()=true for failed status")
	}
}
