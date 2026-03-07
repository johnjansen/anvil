package forecast

import (
	"testing"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func TestProjectRuns_BasicSchedule(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	end := now.Add(24 * time.Hour)

	todos := []project.Todo{
		{
			ID:       "task-1",
			Name:     "hourly-task.md",
			Schedule: "0 * * * *", // every hour
		},
	}

	cfg := config.Default()
	stats := map[string]TaskStats{}

	summary := ProjectRuns(todos, stats, cfg, now, end)

	// Should have ~24 runs (one per hour for 24h, minus the first one at 00:00 since Next starts after)
	if summary.TotalRuns < 23 || summary.TotalRuns > 24 {
		t.Errorf("expected ~24 runs, got %d", summary.TotalRuns)
	}

	// Runs should be sorted chronologically
	for i := 1; i < len(summary.Runs); i++ {
		if summary.Runs[i].ScheduledTime.Before(summary.Runs[i-1].ScheduledTime) {
			t.Errorf("runs not sorted: run %d (%v) before run %d (%v)",
				i, summary.Runs[i].ScheduledTime, i-1, summary.Runs[i-1].ScheduledTime)
		}
	}
}

func TestProjectRuns_SkipsManualTasks(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	end := now.Add(24 * time.Hour)

	todos := []project.Todo{
		{
			ID:       "task-1",
			Name:     "manual-task.md",
			Schedule: "", // no schedule
		},
		{
			ID:       "task-2",
			Name:     "persistent-task.md",
			Schedule: "persistent",
		},
	}

	cfg := config.Default()
	stats := map[string]TaskStats{}

	summary := ProjectRuns(todos, stats, cfg, now, end)

	if summary.TotalRuns != 0 {
		t.Errorf("expected 0 runs for manual/persistent tasks, got %d", summary.TotalRuns)
	}
}

func TestProjectRuns_SkipsDisabledTasks(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	end := now.Add(24 * time.Hour)

	todos := []project.Todo{
		{
			ID:       "task-1",
			Name:     "disabled-task.md",
			Schedule: "0 * * * *",
			Disabled: true,
		},
	}

	cfg := config.Default()
	summary := ProjectRuns(todos, map[string]TaskStats{}, cfg, now, end)

	if summary.TotalRuns != 0 {
		t.Errorf("expected 0 runs for disabled tasks, got %d", summary.TotalRuns)
	}
}

func TestProjectRuns_WithStats(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	end := now.Add(2 * time.Hour)

	todos := []project.Todo{
		{
			ID:       "task-1",
			Name:     "hourly.md",
			Schedule: "0 * * * *",
		},
	}

	stats := map[string]TaskStats{
		"task-1": {
			TaskID:          "task-1",
			RunCount:        5,
			AvgDuration:     10 * time.Minute,
			AvgCostUSD:      0.50,
			AvgInputTokens:  1000,
			AvgOutputTokens: 500,
		},
	}

	cfg := config.Default()
	summary := ProjectRuns(todos, stats, cfg, now, end)

	if summary.TotalRuns < 1 {
		t.Fatalf("expected at least 1 run, got %d", summary.TotalRuns)
	}

	run := summary.Runs[0]
	if !run.HasHistoricalData {
		t.Error("expected HasHistoricalData to be true")
	}
	if run.EstimatedDuration != 10*time.Minute {
		t.Errorf("expected 10m duration, got %v", run.EstimatedDuration)
	}
	if run.EstimatedCostUSD != 0.50 {
		t.Errorf("expected $0.50 cost, got %f", run.EstimatedCostUSD)
	}
}

func TestProjectRuns_MultipleTasks(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	end := now.Add(2 * time.Hour)

	todos := []project.Todo{
		{
			ID:       "task-1",
			Name:     "task-a.md",
			Schedule: "0 * * * *",
		},
		{
			ID:       "task-2",
			Name:     "task-b.md",
			Schedule: "30 * * * *",
		},
	}

	cfg := config.Default()
	summary := ProjectRuns(todos, map[string]TaskStats{}, cfg, now, end)

	// Each should have ~2 runs in 2 hours, total ~4
	if summary.TotalRuns < 3 || summary.TotalRuns > 4 {
		t.Errorf("expected 3-4 runs, got %d", summary.TotalRuns)
	}

	// Verify chronological order across tasks
	for i := 1; i < len(summary.Runs); i++ {
		if summary.Runs[i].ScheduledTime.Before(summary.Runs[i-1].ScheduledTime) {
			t.Errorf("runs not sorted chronologically")
		}
	}
}

func TestHighFrequencyTasks(t *testing.T) {
	summary := ForecastSummary{
		Runs: make([]ForecastRun, 60),
	}
	for i := range summary.Runs {
		summary.Runs[i].TaskName = "frequent-task.md"
	}

	hf := HighFrequencyTasks(summary, 50)
	if !hf["frequent-task.md"] {
		t.Error("expected frequent-task.md to be high-frequency")
	}

	hf2 := HighFrequencyTasks(summary, 100)
	if hf2["frequent-task.md"] {
		t.Error("expected frequent-task.md to not be high-frequency at threshold 100")
	}
}

func TestSummarizeHighFrequency(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	summary := ForecastSummary{
		StartTime: now,
		EndTime:   now.Add(7 * 24 * time.Hour),
	}

	for i := 0; i < 70; i++ {
		summary.Runs = append(summary.Runs, ForecastRun{
			TaskName:          "heartbeat.md",
			EstimatedDuration: 1 * time.Second,
			EstimatedCostUSD:  0.001,
		})
	}

	ds := SummarizeHighFrequency(summary, "heartbeat.md")
	if ds.RunsPerDay != 10 {
		t.Errorf("expected 10 runs/day, got %d", ds.RunsPerDay)
	}
}
