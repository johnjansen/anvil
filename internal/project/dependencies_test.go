package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTestRunRecord creates the proper run record file structure for tests.
func writeTestRunRecord(t *testing.T, projectPath, taskID string, rec RunRecord) {
	t.Helper()
	runsDir := filepath.Join(projectPath, ".anvil", "runs", taskID)
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatal(err)
	}
	recBytes, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(runsDir, rec.RunID+".json"), recBytes, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runsDir, "current"), []byte(rec.RunID), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectDependencyResults_LocalDeps(t *testing.T) {
	tmpDir := t.TempDir()

	todosDir := filepath.Join(tmpDir, ".anvil", "todos", "p0")
	if err := os.MkdirAll(todosDir, 0755); err != nil {
		t.Fatal(err)
	}
	taskContent := "---\nid: task-abc-123\nschedule: \"0 * * * *\"\ncapture_output: true\n---\nDo something\n"
	if err := os.WriteFile(filepath.Join(todosDir, "fetch-data.md"), []byte(taskContent), 0644); err != nil {
		t.Fatal(err)
	}

	writeTestRunRecord(t, tmpDir, "task-abc-123", RunRecord{
		RunID:      "run-001",
		TaskID:     "task-abc-123",
		Started:    time.Now().Add(-time.Minute),
		Finished:   time.Now(),
		Success:    true,
		ResultData: `{"records": 42, "status": "success"}`,
	})

	results, err := CollectDependencyResults(tmpDir, []string{"fetch-data.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	val, ok := results["fetch-data"]
	if !ok {
		t.Fatal("expected key 'fetch-data' in results")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(val, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["records"].(float64) != 42 {
		t.Errorf("expected records=42, got %v", parsed["records"])
	}
}

func TestCollectDependencyResults_MissingResults(t *testing.T) {
	tmpDir := t.TempDir()

	results, err := CollectDependencyResults(tmpDir, []string{"nonexistent-task.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, ok := results["nonexistent-task"]
	if !ok {
		t.Fatal("expected key 'nonexistent-task' in results")
	}

	if string(val) != "null" {
		t.Errorf("expected null for missing task, got %s", string(val))
	}
}

func TestCollectDependencyResults_EmptyResultData(t *testing.T) {
	tmpDir := t.TempDir()

	todosDir := filepath.Join(tmpDir, ".anvil", "todos", "p0")
	if err := os.MkdirAll(todosDir, 0755); err != nil {
		t.Fatal(err)
	}
	taskContent := "---\nid: task-xyz-789\nschedule: \"0 * * * *\"\n---\nDo something\n"
	if err := os.WriteFile(filepath.Join(todosDir, "no-result-task.md"), []byte(taskContent), 0644); err != nil {
		t.Fatal(err)
	}

	writeTestRunRecord(t, tmpDir, "task-xyz-789", RunRecord{
		RunID:    "run-002",
		TaskID:   "task-xyz-789",
		Started:  time.Now().Add(-time.Minute),
		Finished: time.Now(),
		Success:  true,
	})

	results, err := CollectDependencyResults(tmpDir, []string{"no-result-task.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(results["no-result-task"]) != "null" {
		t.Errorf("expected null for empty result, got %s", string(results["no-result-task"]))
	}
}

func TestCollectDependencyResults_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	todosDir := filepath.Join(tmpDir, ".anvil", "todos", "p0")
	if err := os.MkdirAll(todosDir, 0755); err != nil {
		t.Fatal(err)
	}
	taskContent := "---\nid: task-raw-456\nschedule: \"0 * * * *\"\ncapture_output: true\n---\nDo something\n"
	if err := os.WriteFile(filepath.Join(todosDir, "raw-task.md"), []byte(taskContent), 0644); err != nil {
		t.Fatal(err)
	}

	writeTestRunRecord(t, tmpDir, "task-raw-456", RunRecord{
		RunID:      "run-003",
		TaskID:     "task-raw-456",
		Started:    time.Now().Add(-time.Minute),
		Finished:   time.Now(),
		Success:    true,
		ResultData: "not valid json",
	})

	results, err := CollectDependencyResults(tmpDir, []string{"raw-task.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var str string
	if err := json.Unmarshal(results["raw-task"], &str); err != nil {
		t.Fatalf("expected valid JSON string, got parse error: %v", err)
	}
	if str != "not valid json" {
		t.Errorf("expected 'not valid json', got %q", str)
	}
}

func TestParseDependency_Local(t *testing.T) {
	dep := ParseDependency("my-task.md")
	if !dep.IsLocal {
		t.Error("expected local dependency")
	}
	if dep.Task != "my-task.md" {
		t.Errorf("expected task 'my-task.md', got %q", dep.Task)
	}
}

func TestParseDependency_CrossProject(t *testing.T) {
	dep := ParseDependency("other-project:my-task.md")
	if dep.IsLocal {
		t.Error("expected cross-project dependency")
	}
	if dep.Project != "other-project" {
		t.Errorf("expected project 'other-project', got %q", dep.Project)
	}
	if dep.Task != "my-task.md" {
		t.Errorf("expected task 'my-task.md', got %q", dep.Task)
	}
}

func TestResolveTaskName_WithExtension(t *testing.T) {
	dep := Dependency{Task: "my-task.md"}
	if dep.ResolveTaskName() != "my-task.md" {
		t.Errorf("expected 'my-task.md', got %q", dep.ResolveTaskName())
	}
}

func TestResolveTaskName_WithoutExtension(t *testing.T) {
	dep := Dependency{Task: "my-task"}
	if dep.ResolveTaskName() != "my-task.md" {
		t.Errorf("expected 'my-task.md', got %q", dep.ResolveTaskName())
	}
}
