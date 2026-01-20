package sdk

import (
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	task := NewTask("test task")

	if task.Description != "test task" {
		t.Errorf("expected description 'test task', got %q", task.Description)
	}

	if task.ID == "" {
		t.Error("expected non-empty ID")
	}

	if task.Type != TaskTypeGeneric {
		t.Errorf("expected type %q, got %q", TaskTypeGeneric, task.Type)
	}

	if task.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestTaskWithType(t *testing.T) {
	task := NewTask("fix bug").WithType(TaskTypeFix)

	if task.Type != TaskTypeFix {
		t.Errorf("expected type %q, got %q", TaskTypeFix, task.Type)
	}
}

func TestTaskWithPriority(t *testing.T) {
	task := NewTask("urgent task").WithPriority(1)

	if task.Priority != 1 {
		t.Errorf("expected priority 1, got %d", task.Priority)
	}
}

func TestTaskWithFiles(t *testing.T) {
	task := NewTask("modify files").WithFiles("file1.go", "file2.go")

	if len(task.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(task.Files))
	}

	if task.Files[0] != "file1.go" || task.Files[1] != "file2.go" {
		t.Errorf("unexpected files: %v", task.Files)
	}
}

func TestTaskWithContext(t *testing.T) {
	task := NewTask("context task").
		WithContext("key1", "value1").
		WithContext("key2", 42)

	if v, ok := task.Context["key1"]; !ok || v != "value1" {
		t.Errorf("expected key1='value1', got %v", v)
	}

	if v, ok := task.Context["key2"]; !ok || v != 42 {
		t.Errorf("expected key2=42, got %v", v)
	}
}

func TestTaskWithDeadline(t *testing.T) {
	deadline := time.Now().Add(24 * time.Hour)
	task := NewTask("deadline task").WithDeadline(deadline)

	if task.Deadline == nil {
		t.Error("expected non-nil deadline")
	}

	if !task.Deadline.Equal(deadline) {
		t.Errorf("expected deadline %v, got %v", deadline, *task.Deadline)
	}
}

func TestTaskWithConstraints(t *testing.T) {
	constraints := TaskConstraints{
		MaxTokens:    1000,
		MaxFiles:     5,
		RequireTests: true,
	}
	task := NewTask("constrained task").WithConstraints(constraints)

	if task.Constraints.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens=1000, got %d", task.Constraints.MaxTokens)
	}

	if task.Constraints.MaxFiles != 5 {
		t.Errorf("expected MaxFiles=5, got %d", task.Constraints.MaxFiles)
	}

	if !task.Constraints.RequireTests {
		t.Error("expected RequireTests=true")
	}
}

func TestTaskChaining(t *testing.T) {
	task := NewTask("chained task").
		WithType(TaskTypeFeature).
		WithPriority(2).
		WithFiles("main.go").
		WithContext("author", "test")

	if task.Type != TaskTypeFeature {
		t.Errorf("expected type %q", TaskTypeFeature)
	}

	if task.Priority != 2 {
		t.Errorf("expected priority 2")
	}

	if len(task.Files) != 1 {
		t.Errorf("expected 1 file")
	}

	if v, _ := task.Context["author"]; v != "test" {
		t.Errorf("expected author='test'")
	}
}
