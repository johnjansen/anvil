package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Defaults.PreCheck != "" {
		t.Errorf("expected empty defaults, got pre_check=%q", cfg.Defaults.PreCheck)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	os.MkdirAll(anvilDir, 0755)

	content := `defaults:
  allowed_tools:
    - Bash(gh:*)
    - Read
    - Write
  skip_permissions: true
  pre_check: "git rev-parse --is-inside-work-tree"
  on_success: "echo done"
  on_failure: "echo failed"
  timeout: "10m"
  retry: 3
  retry_delay: "30s"
  max_concurrent: 2
  persistent_cooldown: "5m"
  persistent_max_runtime: "1h"
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(content), 0644)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := cfg.Defaults
	if !d.SkipPermissions {
		t.Error("expected skip_permissions=true")
	}
	if len(d.AllowedTools) != 3 || d.AllowedTools[0] != "Bash(gh:*)" {
		t.Errorf("unexpected allowed_tools: %v", d.AllowedTools)
	}
	if d.PreCheck != "git rev-parse --is-inside-work-tree" {
		t.Errorf("unexpected pre_check: %q", d.PreCheck)
	}
	if d.Timeout != "10m" {
		t.Errorf("unexpected timeout: %q", d.Timeout)
	}
	if d.Retry != 3 {
		t.Errorf("expected retry=3, got %d", d.Retry)
	}
	if d.MaxConcurrent != 2 {
		t.Errorf("expected max_concurrent=2, got %d", d.MaxConcurrent)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	os.MkdirAll(anvilDir, 0755)
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(":\n  bad: [yaml"), 0644)

	_, err := LoadConfig(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadTodos_ProjectDefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	// Write project config with defaults
	configContent := `defaults:
  allowed_tools:
    - Read
    - Write
  skip_permissions: true
  pre_check: "test -f .git/HEAD"
  timeout: "20m"
  retry: 2
  on_success: "echo ok"
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(configContent), 0644)

	// Write a task with NO frontmatter overrides for these fields
	taskContent := "---\nid: \"test-id-1\"\nschedule: \"*/5 * * * *\"\n---\nDo something\n"
	os.WriteFile(filepath.Join(todosDir, "task1.md"), []byte(taskContent), 0644)

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
	if !todo.SkipPermissions {
		t.Error("expected skip_permissions from project defaults")
	}
	if len(todo.AllowedTools) != 2 || todo.AllowedTools[0] != "Read" {
		t.Errorf("expected allowed_tools from project defaults, got %v", todo.AllowedTools)
	}
	if todo.PreCheck != "test -f .git/HEAD" {
		t.Errorf("expected pre_check from project defaults, got %q", todo.PreCheck)
	}
	if todo.Timeout != 20*time.Minute {
		t.Errorf("expected timeout=20m from project defaults, got %v", todo.Timeout)
	}
	if todo.Retry != 2 {
		t.Errorf("expected retry=2 from project defaults, got %d", todo.Retry)
	}
	if todo.OnSuccess != "echo ok" {
		t.Errorf("expected on_success from project defaults, got %q", todo.OnSuccess)
	}
}

func TestLoadTodos_FrontmatterOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	// Write project config
	configContent := `defaults:
  allowed_tools:
    - Read
    - Write
  pre_check: "default-check"
  timeout: "20m"
  retry: 2
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(configContent), 0644)

	// Write a task that OVERRIDES some fields
	taskContent := "---\nid: \"test-id-2\"\nschedule: \"*/5 * * * *\"\npre_check: \"custom-check\"\nretry: 5\n---\nDo something else\n"
	os.WriteFile(filepath.Join(todosDir, "task2.md"), []byte(taskContent), 0644)

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
	// Task overrides should win
	if todo.PreCheck != "custom-check" {
		t.Errorf("expected task pre_check override, got %q", todo.PreCheck)
	}
	if todo.Retry != 5 {
		t.Errorf("expected task retry override=5, got %d", todo.Retry)
	}
	// Non-overridden fields should still come from defaults
	if len(todo.AllowedTools) != 2 || todo.AllowedTools[0] != "Read" {
		t.Errorf("expected allowed_tools from defaults, got %v", todo.AllowedTools)
	}
	if todo.Timeout != 20*time.Minute {
		t.Errorf("expected timeout=20m from defaults, got %v", todo.Timeout)
	}
}

func TestLoadTodos_NoFrontmatter_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	configContent := `defaults:
  pre_check: "default-check"
  timeout: "5m"
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(configContent), 0644)

	// Write a task with NO frontmatter at all
	os.WriteFile(filepath.Join(todosDir, "bare-task.md"), []byte("Just a bare task\n"), 0644)

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
	if todo.PreCheck != "default-check" {
		t.Errorf("expected pre_check from defaults, got %q", todo.PreCheck)
	}
	if todo.Timeout != 5*time.Minute {
		t.Errorf("expected timeout=5m from defaults, got %v", todo.Timeout)
	}
}

func TestLoadTodos_NoConfig_NoDefaults(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	// No config.yaml
	taskContent := "---\nid: \"test-id-3\"\nschedule: \"*/5 * * * *\"\n---\nSimple task\n"
	os.WriteFile(filepath.Join(todosDir, "task3.md"), []byte(taskContent), 0644)

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
	if todo.SkipPermissions {
		t.Error("expected skip_permissions=false with no config")
	}
	if len(todo.AllowedTools) != 0 {
		t.Errorf("expected empty allowed_tools with no config, got %v", todo.AllowedTools)
	}
	if todo.PreCheck != "" {
		t.Errorf("expected empty pre_check with no config, got %q", todo.PreCheck)
	}
}

func TestLoadTodos_ExplicitZeroOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	configContent := `defaults:
  skip_permissions: true
  retry: 3
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(configContent), 0644)

	// Task explicitly sets skip_permissions: false and retry: 0
	taskContent := "---\nid: \"test-id-4\"\nschedule: \"*/5 * * * *\"\nskip_permissions: false\nretry: 0\n---\nExplicit zero\n"
	os.WriteFile(filepath.Join(todosDir, "task4.md"), []byte(taskContent), 0644)

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
	// Even though defaults say skip_permissions=true, task explicitly set false
	if todo.SkipPermissions {
		t.Error("expected skip_permissions=false (explicit override of default)")
	}
	// Even though defaults say retry=3, task explicitly set 0
	if todo.Retry != 0 {
		t.Errorf("expected retry=0 (explicit override of default), got %d", todo.Retry)
	}
}

func TestLoadTodos_RunnerFromFrontmatter(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	taskContent := "---\nid: \"test-runner-1\"\nschedule: \"*/30 * * * *\"\nrunner: \"claude --model haiku\"\n---\nTriage issues\n"
	os.WriteFile(filepath.Join(todosDir, "triage.md"), []byte(taskContent), 0644)

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

	if todos[0].Runner != "claude --model haiku" {
		t.Errorf("expected runner='claude --model haiku', got %q", todos[0].Runner)
	}
}

func TestLoadTodos_RunnerFromProjectDefaults(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	configContent := `defaults:
  runner: "claude --model sonnet"
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(configContent), 0644)

	taskContent := "---\nid: \"test-runner-2\"\nschedule: \"*/5 * * * *\"\n---\nDo something\n"
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

	if todos[0].Runner != "claude --model sonnet" {
		t.Errorf("expected runner from defaults, got %q", todos[0].Runner)
	}
}

func TestLoadTodos_RunnerFrontmatterOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos", "p1")
	os.MkdirAll(todosDir, 0755)

	configContent := `defaults:
  runner: "claude --model sonnet"
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(configContent), 0644)

	taskContent := "---\nid: \"test-runner-3\"\nschedule: \"*/30 * * * *\"\nrunner: \"claude --model haiku\"\n---\nCheap triage\n"
	os.WriteFile(filepath.Join(todosDir, "triage.md"), []byte(taskContent), 0644)

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

	if todos[0].Runner != "claude --model haiku" {
		t.Errorf("expected task runner to override default, got %q", todos[0].Runner)
	}
}

func TestAddTodo_WithRunner(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos")
	os.MkdirAll(todosDir, 0755)

	proj := &Project{Path: dir}
	_, err := proj.AddTodo(1, "*/30 * * * *", "Triage issues", "", "", 0, false, "claude --model haiku")
	if err != nil {
		t.Fatalf("AddTodo error: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	if todos[0].Runner != "claude --model haiku" {
		t.Errorf("expected runner='claude --model haiku', got %q", todos[0].Runner)
	}
}

func TestAddTodo_WithoutRunner(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil")
	todosDir := filepath.Join(anvilDir, "todos")
	os.MkdirAll(todosDir, 0755)

	proj := &Project{Path: dir}
	_, err := proj.AddTodo(1, "*/30 * * * *", "Review PRs", "", "", 0, false, "")
	if err != nil {
		t.Fatalf("AddTodo error: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos error: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	if todos[0].Runner != "" {
		t.Errorf("expected empty runner, got %q", todos[0].Runner)
	}
}
