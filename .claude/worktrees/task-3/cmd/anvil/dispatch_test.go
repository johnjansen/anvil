package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johnjansen/anvil/internal/project"
)

func TestAddTodoWithID(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos")
	os.MkdirAll(todosDir, 0755)

	proj := &project.Project{Path: dir}

	// Test adding a todo with ID
	relPath, taskID, err := proj.AddTodoWithID(1, "", "Test task", "", "", 1, false, "", nil)
	if err != nil {
		t.Fatalf("AddTodoWithID error: %v", err)
	}

	if relPath == "" {
		t.Error("expected non-empty relative path")
	}

	if taskID == "" {
		t.Error("expected non-empty task ID")
	}

	// Verify the task file was created
	taskFilePath := filepath.Join(dir, ".anvil", "todos", relPath)
	if _, err := os.Stat(taskFilePath); os.IsNotExist(err) {
		t.Error("expected task file to be created")
	}

	// Load todos and verify the task exists
	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	// Verify the task ID matches
	if todos[0].ID != taskID {
		t.Errorf("expected task ID %q, got %q", taskID, todos[0].ID)
	}
}

func TestAddTodoWithID_OneShot(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos")
	os.MkdirAll(todosDir, 0755)

	proj := &project.Project{Path: dir}

	// Test adding a one-shot task (empty schedule)
	relPath, taskID, err := proj.AddTodoWithID(1, "", "One-shot task", "", "", 1, false, "", nil)
	if err != nil {
		t.Fatalf("AddTodoWithID error: %v", err)
	}

	if relPath == "" {
		t.Error("expected non-empty relative path")
	}

	if taskID == "" {
		t.Error("expected non-empty task ID")
	}

	// Load todos and verify the task exists
	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	// Verify it's a one-shot task (should have resume: false)
	content, err := os.ReadFile(filepath.Join(dir, ".anvil", "todos", relPath))
	if err != nil {
		t.Fatalf("failed to read task file: %v", err)
	}

	// Check that the content contains resume: false
	if !strings.Contains(string(content), "resume: false") {
		t.Error("expected one-shot task to have resume: false")
	}
}
