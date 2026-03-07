package forecast

import (
	"testing"
	"time"
)

func TestDetectContention_NoOverlap(t *testing.T) {
	now := time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC)

	summary := ForecastSummary{
		Runs: []ForecastRun{
			{TaskName: "task-a.md", ScheduledTime: now, EstimatedDuration: 5 * time.Minute},
			{TaskName: "task-b.md", ScheduledTime: now.Add(10 * time.Minute), EstimatedDuration: 5 * time.Minute},
		},
	}

	windows := DetectContention(summary, 1)
	if len(windows) != 0 {
		t.Errorf("expected no contention, got %d windows", len(windows))
	}
}

func TestDetectContention_SimpleOverlap(t *testing.T) {
	now := time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC)

	summary := ForecastSummary{
		EndTime: now.Add(1 * time.Hour),
		Runs: []ForecastRun{
			{TaskName: "task-a.md", ScheduledTime: now, EstimatedDuration: 10 * time.Minute},
			{TaskName: "task-b.md", ScheduledTime: now, EstimatedDuration: 10 * time.Minute},
		},
	}

	windows := DetectContention(summary, 1)
	if len(windows) != 1 {
		t.Fatalf("expected 1 contention window, got %d", len(windows))
	}

	w := windows[0]
	if w.PeakConcurrent != 2 {
		t.Errorf("expected peak concurrent 2, got %d", w.PeakConcurrent)
	}
	if w.Overflow != 1 {
		t.Errorf("expected overflow 1, got %d", w.Overflow)
	}
	if len(w.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(w.Tasks))
	}
}

func TestDetectContention_NoContentionWithEnoughWorkers(t *testing.T) {
	now := time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC)

	summary := ForecastSummary{
		Runs: []ForecastRun{
			{TaskName: "task-a.md", ScheduledTime: now, EstimatedDuration: 10 * time.Minute},
			{TaskName: "task-b.md", ScheduledTime: now, EstimatedDuration: 10 * time.Minute},
			{TaskName: "task-c.md", ScheduledTime: now, EstimatedDuration: 10 * time.Minute},
		},
	}

	windows := DetectContention(summary, 3)
	if len(windows) != 0 {
		t.Errorf("expected no contention with 3 workers, got %d windows", len(windows))
	}
}

func TestDetectContention_PartialOverlap(t *testing.T) {
	now := time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC)

	summary := ForecastSummary{
		EndTime: now.Add(1 * time.Hour),
		Runs: []ForecastRun{
			{TaskName: "task-a.md", ScheduledTime: now, EstimatedDuration: 15 * time.Minute},
			{TaskName: "task-b.md", ScheduledTime: now.Add(5 * time.Minute), EstimatedDuration: 15 * time.Minute},
		},
	}

	windows := DetectContention(summary, 1)
	if len(windows) != 1 {
		t.Fatalf("expected 1 contention window for partial overlap, got %d", len(windows))
	}

	w := windows[0]
	if w.PeakConcurrent != 2 {
		t.Errorf("expected peak concurrent 2, got %d", w.PeakConcurrent)
	}
}

func TestDetectContention_MultipleWindows(t *testing.T) {
	now := time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC)

	summary := ForecastSummary{
		EndTime: now.Add(2 * time.Hour),
		Runs: []ForecastRun{
			{TaskName: "task-a.md", ScheduledTime: now, EstimatedDuration: 5 * time.Minute},
			{TaskName: "task-b.md", ScheduledTime: now, EstimatedDuration: 5 * time.Minute},
			{TaskName: "task-c.md", ScheduledTime: now.Add(1 * time.Hour), EstimatedDuration: 5 * time.Minute},
			{TaskName: "task-d.md", ScheduledTime: now.Add(1 * time.Hour), EstimatedDuration: 5 * time.Minute},
		},
	}

	windows := DetectContention(summary, 1)
	if len(windows) != 2 {
		t.Errorf("expected 2 contention windows, got %d", len(windows))
	}
}
