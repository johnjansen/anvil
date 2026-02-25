package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestProject(t *testing.T, numRuns int, fileAge time.Duration) string {
	t.Helper()
	dir := t.TempDir()

	logsDir := filepath.Join(dir, ".anvil", "logs", "task-1")
	runsDir := filepath.Join(dir, ".anvil", "runs", "task-1")
	os.MkdirAll(logsDir, 0755)
	os.MkdirAll(runsDir, 0755)

	now := time.Now()
	for i := 0; i < numRuns; i++ {
		runID := now.Add(-fileAge * time.Duration(i)).Format("20060102-150405")
		logPath := filepath.Join(logsDir, runID+".log")
		runPath := filepath.Join(runsDir, runID+".json")
		os.WriteFile(logPath, []byte("log output"), 0644)
		os.WriteFile(runPath, []byte(`{"run_id":"`+runID+`"}`), 0644)

		modTime := now.Add(-fileAge * time.Duration(i))
		os.Chtimes(logPath, modTime, modTime)
		os.Chtimes(runPath, modTime, modTime)
	}

	// Write "current" pointer
	currentPath := filepath.Join(runsDir, "current")
	latestID := now.Format("20060102-150405")
	os.WriteFile(currentPath, []byte(latestID), 0644)

	return dir
}

func TestPruneProject_MaxRuns(t *testing.T) {
	dir := setupTestProject(t, 10, time.Hour)

	result := PruneProject(dir, PruneOptions{
		MaxRuns: 3,
		Now:     time.Now(),
	})

	if result.RunsDeleted != 7 {
		t.Errorf("expected 7 runs deleted, got %d", result.RunsDeleted)
	}
	if result.LogsDeleted != 7 {
		t.Errorf("expected 7 logs deleted, got %d", result.LogsDeleted)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	// Verify 3 run files remain
	remaining := listDataFiles(filepath.Join(dir, ".anvil", "runs", "task-1"), ".json")
	if len(remaining) != 3 {
		t.Errorf("expected 3 remaining run files, got %d", len(remaining))
	}
}

func TestPruneProject_MaxAge(t *testing.T) {
	dir := setupTestProject(t, 10, 24*time.Hour)
	now := time.Now()

	result := PruneProject(dir, PruneOptions{
		MaxAge: 5 * 24 * time.Hour, // 5 days
		Now:    now,
	})

	// Files aged 0d, 1d, 2d, 3d, 4d should survive; 5d, 6d, 7d, 8d, 9d should be pruned
	if result.RunsDeleted != 5 {
		t.Errorf("expected 5 runs deleted, got %d", result.RunsDeleted)
	}
	if result.LogsDeleted != 5 {
		t.Errorf("expected 5 logs deleted, got %d", result.LogsDeleted)
	}
}

func TestPruneProject_BothLimits(t *testing.T) {
	dir := setupTestProject(t, 10, 24*time.Hour)
	now := time.Now()

	// MaxRuns=8 would keep 8, MaxAge=3d would keep 4. More aggressive wins.
	result := PruneProject(dir, PruneOptions{
		MaxRuns: 8,
		MaxAge:  3 * 24 * time.Hour,
		Now:     now,
	})

	// Items 0-2 are within both limits, items 3-7 exceed age, items 8-9 exceed both
	// Total pruned: anything that violates either limit
	// MaxRuns prunes items 8,9 (2 items). MaxAge prunes items 3-9 (7 items).
	// Union: items 3-9 = 7 items pruned
	if result.RunsDeleted != 7 {
		t.Errorf("expected 7 runs deleted, got %d", result.RunsDeleted)
	}
}

func TestPruneProject_DryRun(t *testing.T) {
	dir := setupTestProject(t, 10, time.Hour)

	result := PruneProject(dir, PruneOptions{
		MaxRuns: 3,
		DryRun:  true,
		Now:     time.Now(),
	})

	if result.RunsDeleted != 7 {
		t.Errorf("expected 7 runs would-delete, got %d", result.RunsDeleted)
	}

	// Verify nothing was actually deleted
	remaining := listDataFiles(filepath.Join(dir, ".anvil", "runs", "task-1"), ".json")
	if len(remaining) != 10 {
		t.Errorf("expected 10 run files to remain in dry run, got %d", len(remaining))
	}
}

func TestPruneProject_NoPolicy(t *testing.T) {
	dir := setupTestProject(t, 10, time.Hour)

	result := PruneProject(dir, PruneOptions{
		Now: time.Now(),
	})

	if result.RunsDeleted != 0 || result.LogsDeleted != 0 {
		t.Errorf("expected nothing pruned with no policy, got %d runs, %d logs", result.RunsDeleted, result.LogsDeleted)
	}
}

func TestPruneProject_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".anvil"), 0755)

	result := PruneProject(dir, PruneOptions{
		MaxRuns: 5,
		Now:     time.Now(),
	})

	if result.RunsDeleted != 0 || result.LogsDeleted != 0 {
		t.Errorf("expected nothing pruned for empty project, got %d runs, %d logs", result.RunsDeleted, result.LogsDeleted)
	}
}

func TestPruneProject_PreservesCurrentPointer(t *testing.T) {
	dir := setupTestProject(t, 10, time.Hour)

	PruneProject(dir, PruneOptions{
		MaxRuns: 3,
		Now:     time.Now(),
	})

	// Verify "current" file still exists
	currentPath := filepath.Join(dir, ".anvil", "runs", "task-1", "current")
	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		t.Error("current pointer file was deleted")
	}
}

func TestPruneProject_MultipleTaskIDs(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	for _, taskID := range []string{"task-a", "task-b"} {
		logsDir := filepath.Join(dir, ".anvil", "logs", taskID)
		runsDir := filepath.Join(dir, ".anvil", "runs", taskID)
		os.MkdirAll(logsDir, 0755)
		os.MkdirAll(runsDir, 0755)

		for i := 0; i < 5; i++ {
			runID := now.Add(-time.Hour * time.Duration(i)).Format("20060102-150405")
			os.WriteFile(filepath.Join(logsDir, runID+".log"), []byte("log"), 0644)
			os.WriteFile(filepath.Join(runsDir, runID+".json"), []byte(`{}`), 0644)
			modTime := now.Add(-time.Hour * time.Duration(i))
			os.Chtimes(filepath.Join(logsDir, runID+".log"), modTime, modTime)
			os.Chtimes(filepath.Join(runsDir, runID+".json"), modTime, modTime)
		}
	}

	result := PruneProject(dir, PruneOptions{
		MaxRuns: 2,
		Now:     now,
	})

	// Each task should have 3 pruned (5-2=3), total 6
	if result.RunsDeleted != 6 {
		t.Errorf("expected 6 runs deleted, got %d", result.RunsDeleted)
	}
	if result.LogsDeleted != 6 {
		t.Errorf("expected 6 logs deleted, got %d", result.LogsDeleted)
	}
}
