package forecast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

func TestComputeTaskStats_NoRecords(t *testing.T) {
	dir := t.TempDir()

	stats, err := ComputeTaskStats(dir, "nonexistent", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.RunCount != 0 {
		t.Errorf("expected 0 runs, got %d", stats.RunCount)
	}
}

func TestComputeTaskStats_WithRecords(t *testing.T) {
	dir := t.TempDir()
	taskID := "test-task"
	runsDir := filepath.Join(dir, ".anvil", "runs", taskID)
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	records := []project.RunRecord{
		{
			RunID:            "run-1",
			TaskID:           taskID,
			Started:          now.Add(-2 * time.Hour),
			Finished:         now.Add(-2*time.Hour + 10*time.Minute),
			Success:          true,
			InputTokens:      1000,
			OutputTokens:     500,
			EstimatedCostUSD: 0.10,
		},
		{
			RunID:            "run-2",
			TaskID:           taskID,
			Started:          now.Add(-1 * time.Hour),
			Finished:         now.Add(-1*time.Hour + 20*time.Minute),
			Success:          true,
			InputTokens:      2000,
			OutputTokens:     1000,
			EstimatedCostUSD: 0.20,
		},
		{
			RunID:    "run-3",
			TaskID:   taskID,
			Started:  now.Add(-30 * time.Minute),
			Finished: now.Add(-25 * time.Minute),
			Success:  false, // failed runs should be excluded
		},
	}

	for _, r := range records {
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		filename := r.RunID + ".json"
		if err := os.WriteFile(filepath.Join(runsDir, filename), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := ComputeTaskStats(dir, taskID, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.RunCount != 2 {
		t.Errorf("expected 2 successful runs, got %d", stats.RunCount)
	}

	expectedAvgDuration := (10*time.Minute + 20*time.Minute) / 2
	if stats.AvgDuration != expectedAvgDuration {
		t.Errorf("expected avg duration %v, got %v", expectedAvgDuration, stats.AvgDuration)
	}

	expectedAvgCost := 0.15
	if diff := stats.AvgCostUSD - expectedAvgCost; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected avg cost %f, got %f", expectedAvgCost, stats.AvgCostUSD)
	}

	if stats.AvgInputTokens != 1500 {
		t.Errorf("expected avg input tokens 1500, got %d", stats.AvgInputTokens)
	}

	if stats.AvgOutputTokens != 750 {
		t.Errorf("expected avg output tokens 750, got %d", stats.AvgOutputTokens)
	}
}

func TestComputeTaskStats_MaxSamples(t *testing.T) {
	dir := t.TempDir()
	taskID := "test-task"
	runsDir := filepath.Join(dir, ".anvil", "runs", taskID)
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	// Create 5 records
	for i := 0; i < 5; i++ {
		r := project.RunRecord{
			RunID:            "run-" + string(rune('a'+i)),
			TaskID:           taskID,
			Started:          now.Add(time.Duration(-5+i) * time.Hour),
			Finished:         now.Add(time.Duration(-5+i)*time.Hour + 10*time.Minute),
			Success:          true,
			EstimatedCostUSD: float64(i+1) * 0.10,
		}
		data, _ := json.Marshal(r)
		os.WriteFile(filepath.Join(runsDir, r.RunID+".json"), data, 0644)
	}

	// Only take last 2
	stats, err := ComputeTaskStats(dir, taskID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.RunCount != 2 {
		t.Errorf("expected 2 runs with maxSamples=2, got %d", stats.RunCount)
	}
}
