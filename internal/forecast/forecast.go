package forecast

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

// ForecastRun represents a single projected task execution.
type ForecastRun struct {
	TaskID            string        `json:"task_id"`
	TaskName          string        `json:"task_name"`
	ScheduledTime     time.Time     `json:"scheduled_time"`
	EstimatedDuration time.Duration `json:"estimated_duration_seconds"`
	EstimatedCostUSD  float64       `json:"estimated_cost_usd"`
	InputTokens       int           `json:"input_tokens"`
	OutputTokens      int           `json:"output_tokens"`
	HasHistoricalData bool          `json:"has_historical_data"`
	IsHypothetical    bool          `json:"is_hypothetical"`
}

// ForecastSummary is the aggregate forecast result.
type ForecastSummary struct {
	StartTime         time.Time     `json:"start"`
	EndTime           time.Time     `json:"end"`
	TotalRuns         int           `json:"total_runs"`
	TotalDuration     time.Duration `json:"total_duration_seconds"`
	TotalCostUSD      float64       `json:"total_cost_usd"`
	TotalInputTokens  int           `json:"total_input_tokens"`
	TotalOutputTokens int           `json:"total_output_tokens"`
	Runs              []ForecastRun `json:"runs"`
}

// TaskRunCounts maps task name to its run count (for summarization).
func (s *ForecastSummary) TaskRunCounts() map[string]int {
	counts := make(map[string]int)
	for _, r := range s.Runs {
		counts[r.TaskName]++
	}
	return counts
}

// ProjectRuns generates a forecast of all task executions within the given time window.
func ProjectRuns(todos []project.Todo, stats map[string]TaskStats, cfg *config.Config, start, end time.Time) ForecastSummary {
	summary := ForecastSummary{
		StartTime: start,
		EndTime:   end,
	}

	for _, todo := range todos {
		if todo.Schedule == "" || todo.Schedule == "persistent" || todo.Disabled {
			continue
		}

		runs := projectTaskRuns(todo, stats, cfg, start, end)
		summary.Runs = append(summary.Runs, runs...)
	}

	sort.Slice(summary.Runs, func(i, j int) bool {
		return summary.Runs[i].ScheduledTime.Before(summary.Runs[j].ScheduledTime)
	})

	summary.TotalRuns = len(summary.Runs)
	for _, r := range summary.Runs {
		summary.TotalDuration += r.EstimatedDuration
		summary.TotalCostUSD += r.EstimatedCostUSD
		summary.TotalInputTokens += r.InputTokens
		summary.TotalOutputTokens += r.OutputTokens
	}

	return summary
}

func projectTaskRuns(todo project.Todo, stats map[string]TaskStats, cfg *config.Config, start, end time.Time) []ForecastRun {
	st, hasStats := stats[todo.ID]

	hasWindow := todo.Window.Start != "" || todo.Window.End != ""
	hasQuiet := cfg.QuietHours.Enabled

	// Parse cron expression once outside the loop
	var parser *cron.Parser
	if !hasWindow && !hasQuiet {
		var parseErr error
		parser, parseErr = cron.Parse(todo.Schedule)
		if parseErr != nil {
			log.Printf("warning: skipping task %q: invalid schedule %q", todo.Name, todo.Schedule)
			return nil
		}
	}

	var runs []ForecastRun
	cursor := start

	for {
		var next time.Time
		var err error

		if hasWindow || hasQuiet {
			next, err = daemon.NextAllowedRun(todo.Schedule, todo.Window, cfg.QuietHours, todo.Priority, cursor)
		} else {
			next, err = parser.Next(cursor)
		}

		if err != nil {
			break
		}
		if next.After(end) || next.Equal(end) {
			break
		}

		run := ForecastRun{
			TaskID:         todo.ID,
			TaskName:       todo.Name,
			ScheduledTime:  next,
			IsHypothetical: false,
		}

		if hasStats && st.RunCount > 0 {
			run.HasHistoricalData = true
			run.EstimatedDuration = st.AvgDuration
			run.EstimatedCostUSD = st.AvgCostUSD
			run.InputTokens = st.AvgInputTokens
			run.OutputTokens = st.AvgOutputTokens
		} else if todo.Timeout > 0 {
			run.EstimatedDuration = todo.Timeout
		}

		runs = append(runs, run)
		cursor = next
	}

	return runs
}

// HighFrequencyTasks returns task names that have more than threshold runs in the forecast.
func HighFrequencyTasks(summary ForecastSummary, threshold int) map[string]bool {
	counts := summary.TaskRunCounts()
	hf := make(map[string]bool)
	for name, count := range counts {
		if count > threshold {
			hf[name] = true
		}
	}
	return hf
}

// DailySummary holds aggregated stats for a high-frequency task per day.
type DailySummary struct {
	TaskName      string
	RunsPerDay    int
	DailyDuration time.Duration
	DailyCostUSD  float64
}

// SummarizeHighFrequency computes daily summaries for high-frequency tasks.
func SummarizeHighFrequency(summary ForecastSummary, taskName string) DailySummary {
	var totalRuns int
	var totalDuration time.Duration
	var totalCost float64

	for _, r := range summary.Runs {
		if r.TaskName == taskName {
			totalRuns++
			totalDuration += r.EstimatedDuration
			totalCost += r.EstimatedCostUSD
		}
	}

	days := summary.EndTime.Sub(summary.StartTime).Hours() / 24
	if days < 1 {
		days = 1
	}

	return DailySummary{
		TaskName:      taskName,
		RunsPerDay:    int(float64(totalRuns) / days),
		DailyDuration: time.Duration(float64(totalDuration) / days),
		DailyCostUSD:  totalCost / days,
	}
}

// ValidateSchedule checks if a cron expression is valid.
func ValidateSchedule(schedule string) error {
	_, err := cron.Parse(schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule %q: %w", schedule, err)
	}
	return nil
}
