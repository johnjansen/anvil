package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadTodos_TimeoutWarning(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	taskContent := "---\nid: \"tw-1\"\ntimeout: \"30m\"\ntimeout_warning: \"5m\"\non_timeout_warning: \"echo warning\"\non_timeout: \"echo timed out\"\n---\nDo something\n"
	os.WriteFile(filepath.Join(todosDir, "task.md"), []byte(taskContent), 0644)

	proj, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	todo := todos[0]
	if todo.Timeout != 30*time.Minute {
		t.Errorf("expected timeout=30m, got %v", todo.Timeout)
	}
	if todo.TimeoutWarning != 5*time.Minute {
		t.Errorf("expected timeout_warning=5m, got %v", todo.TimeoutWarning)
	}
	if todo.OnTimeoutWarning != "echo warning" {
		t.Errorf("expected on_timeout_warning='echo warning', got %q", todo.OnTimeoutWarning)
	}
	if todo.OnTimeout != "echo timed out" {
		t.Errorf("expected on_timeout='echo timed out', got %q", todo.OnTimeout)
	}
}

func TestLoadTodos_AdaptiveTimeout(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	taskContent := `---
id: "at-1"
timeout: "30m"
adaptive_timeout:
  enabled: true
  extend_if: "checkpoint_exists"
  max_extensions: 3
  extension_duration: "10m"
---
Do something
`
	os.WriteFile(filepath.Join(todosDir, "task.md"), []byte(taskContent), 0644)

	proj, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	todo := todos[0]
	if todo.AdaptiveTimeout == nil {
		t.Fatal("expected adaptive_timeout to be set")
	}
	if !todo.AdaptiveTimeout.Enabled {
		t.Error("expected adaptive_timeout.enabled=true")
	}
	if todo.AdaptiveTimeout.ExtendIf != "checkpoint_exists" {
		t.Errorf("expected extend_if='checkpoint_exists', got %q", todo.AdaptiveTimeout.ExtendIf)
	}
	if todo.AdaptiveTimeout.MaxExtensions != 3 {
		t.Errorf("expected max_extensions=3, got %d", todo.AdaptiveTimeout.MaxExtensions)
	}
	if todo.AdaptiveTimeout.ExtensionDuration != 10*time.Minute {
		t.Errorf("expected extension_duration=10m, got %v", todo.AdaptiveTimeout.ExtensionDuration)
	}
}

func TestLoadTodos_AdaptiveTimeoutDisabled(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	taskContent := `---
id: "at-2"
timeout: "30m"
adaptive_timeout:
  enabled: false
  extend_if: "checkpoint_exists"
---
Do something
`
	os.WriteFile(filepath.Join(todosDir, "task.md"), []byte(taskContent), 0644)

	proj, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	todo := todos[0]
	if todo.AdaptiveTimeout != nil {
		t.Error("expected adaptive_timeout to be nil when disabled")
	}
}

func TestLoadTodos_NoTimeoutEscalation(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	taskContent := "---\nid: \"basic-1\"\ntimeout: \"5m\"\n---\nDo something\n"
	os.WriteFile(filepath.Join(todosDir, "task.md"), []byte(taskContent), 0644)

	proj, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	todo := todos[0]
	if todo.TimeoutWarning != 0 {
		t.Errorf("expected zero timeout_warning, got %v", todo.TimeoutWarning)
	}
	if todo.OnTimeoutWarning != "" {
		t.Errorf("expected empty on_timeout_warning, got %q", todo.OnTimeoutWarning)
	}
	if todo.OnTimeout != "" {
		t.Errorf("expected empty on_timeout, got %q", todo.OnTimeout)
	}
	if todo.AdaptiveTimeout != nil {
		t.Error("expected nil adaptive_timeout")
	}
}

func TestLoadTodos_AdaptiveTimeoutDefaultExtensionDuration(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	taskContent := `---
id: "at-3"
timeout: "30m"
adaptive_timeout:
  enabled: true
  extend_if: "checkpoint_exists"
  max_extensions: 2
---
Do something
`
	os.WriteFile(filepath.Join(todosDir, "task.md"), []byte(taskContent), 0644)

	proj, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}

	todo := todos[0]
	if todo.AdaptiveTimeout == nil {
		t.Fatal("expected adaptive_timeout to be set")
	}
	if todo.AdaptiveTimeout.ExtensionDuration != 0 {
		t.Errorf("expected zero extension_duration (defaults to original timeout at runtime), got %v", todo.AdaptiveTimeout.ExtensionDuration)
	}
}
