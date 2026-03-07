package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadRunRecord(t *testing.T) {
	dir := t.TempDir()
	taskID := "test-task-id"
	runID := "run-abc123"

	// Create runs directory and write a run record
	runsPath := filepath.Join(dir, ".anvil", "runs", taskID)
	os.MkdirAll(runsPath, 0755)

	rec := RunRecord{
		RunID:         runID,
		TaskID:        taskID,
		Started:       time.Now().Add(-5 * time.Minute),
		Finished:      time.Now(),
		Success:       true,
		OutputSummary: "test output",
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runsPath, runID+".json"), data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read it back
	got, err := ReadRunRecord(dir, taskID, runID)
	if err != nil {
		t.Fatalf("ReadRunRecord: %v", err)
	}
	if got.RunID != runID {
		t.Errorf("expected RunID=%q, got %q", runID, got.RunID)
	}
	if !got.Success {
		t.Error("expected Success=true")
	}
	if got.OutputSummary != "test output" {
		t.Errorf("expected OutputSummary=%q, got %q", "test output", got.OutputSummary)
	}
}

func TestReadRunRecord_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadRunRecord(dir, "nonexistent", "no-run")
	if err == nil {
		t.Error("expected error for nonexistent run")
	}
}

func TestReplayAndPinnedRunFrontmatter(t *testing.T) {
	dir := t.TempDir()
	anvilDir := filepath.Join(dir, ".anvil", "todos", "p2")
	os.MkdirAll(anvilDir, 0755)

	content := `---
schedule: "0 9 * * *"
replay: true
pinned_run: abc123
---
Check for updates
`
	taskPath := filepath.Join(anvilDir, "check-updates.md")
	os.WriteFile(taskPath, []byte(content), 0644)

	proj := &Project{Path: dir}
	todos, err := proj.LoadTodos()
	if err != nil {
		t.Fatalf("LoadTodos: %v", err)
	}

	if len(todos) == 0 {
		t.Fatal("expected at least 1 todo")
	}

	var found *Todo
	for i := range todos {
		if todos[i].Name == "check-updates.md" {
			found = &todos[i]
			break
		}
	}

	if found == nil {
		t.Fatal("expected to find check-updates task")
	}

	if !found.Replay {
		t.Error("expected Replay=true")
	}
	if found.PinnedRun != "abc123" {
		t.Errorf("expected PinnedRun=%q, got %q", "abc123", found.PinnedRun)
	}
}

func TestWriteAndReadSnapshot(t *testing.T) {
	dir := t.TempDir()
	taskID := "snap-task"
	runID := "snap-run-1"

	rec := RunRecord{
		RunID:         runID,
		TaskID:        taskID,
		Success:       true,
		OutputSummary: "snapshot output",
	}

	err := WriteSnapshot(dir, taskID, runID, map[string]string{"key": "val"}, map[string]string{"ENV": "test"}, "do the thing", dir, rec)
	if err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}

	// Read snapshot metadata
	snap, err := ReadSnapshot(dir, taskID, runID)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	if snap.TaskID != taskID {
		t.Errorf("expected TaskID=%q, got %q", taskID, snap.TaskID)
	}

	// Read prompt
	promptData, err := ReadSnapshotFile(dir, taskID, runID, "prompt.txt")
	if err != nil {
		t.Fatalf("ReadSnapshotFile(prompt): %v", err)
	}
	if string(promptData) != "do the thing" {
		t.Errorf("expected prompt=%q, got %q", "do the thing", string(promptData))
	}

	// Read run record from snapshot
	recData, err := ReadSnapshotFile(dir, taskID, runID, "run_record.json")
	if err != nil {
		t.Fatalf("ReadSnapshotFile(run_record): %v", err)
	}
	var gotRec RunRecord
	if err := json.Unmarshal(recData, &gotRec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if gotRec.OutputSummary != "snapshot output" {
		t.Errorf("expected OutputSummary=%q, got %q", "snapshot output", gotRec.OutputSummary)
	}
}
