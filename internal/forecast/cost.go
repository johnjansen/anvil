package forecast

import (
	"sort"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

// TaskStats holds historical statistics for a single task.
type TaskStats struct {
	TaskID          string
	RunCount        int
	AvgDuration     time.Duration
	AvgCostUSD      float64
	AvgInputTokens  int
	AvgOutputTokens int
}

// ComputeTaskStats calculates average statistics from the last maxSamples successful runs.
func ComputeTaskStats(projectPath, taskID string, maxSamples int) (TaskStats, error) {
	records, err := project.ReadAllRunRecords(projectPath, taskID)
	if err != nil {
		return TaskStats{TaskID: taskID}, err
	}

	// Filter to successful runs only
	var successful []project.RunRecord
	for _, r := range records {
		if r.Success && !r.Finished.IsZero() {
			successful = append(successful, r)
		}
	}

	if len(successful) == 0 {
		return TaskStats{TaskID: taskID}, nil
	}

	// Sort by start time descending to get most recent
	sort.Slice(successful, func(i, j int) bool {
		return successful[i].Started.After(successful[j].Started)
	})

	// Take up to maxSamples
	if len(successful) > maxSamples {
		successful = successful[:maxSamples]
	}

	stats := TaskStats{
		TaskID:   taskID,
		RunCount: len(successful),
	}

	var totalDuration time.Duration
	var totalCost float64
	var totalInput, totalOutput int

	for _, r := range successful {
		totalDuration += r.Finished.Sub(r.Started)
		totalCost += r.EstimatedCostUSD
		totalInput += r.InputTokens
		totalOutput += r.OutputTokens
	}

	n := len(successful)
	stats.AvgDuration = totalDuration / time.Duration(n)
	stats.AvgCostUSD = totalCost / float64(n)
	stats.AvgInputTokens = totalInput / n
	stats.AvgOutputTokens = totalOutput / n

	return stats, nil
}

// ComputeAllStats computes stats for all tasks in a project.
func ComputeAllStats(projectPath string, todos []project.Todo, maxSamples int) map[string]TaskStats {
	stats := make(map[string]TaskStats)
	for _, todo := range todos {
		if todo.ID == "" {
			continue
		}
		s, err := ComputeTaskStats(projectPath, todo.ID, maxSamples)
		if err != nil {
			continue
		}
		stats[todo.ID] = s
	}
	return stats
}
