package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/forecast"
	"github.com/johnjansen/anvil/internal/project"
)

func taskForecastCmd(args []string) {
	fs := flag.NewFlagSet("anvil task forecast", flag.ExitOnError)
	days := fs.Int("days", 7, "Forecast horizon in days")
	taskFilter := fs.String("task", "", "Filter to a specific task by name")
	contention := fs.Bool("contention", false, "Show resource contention windows")
	cost := fs.Bool("cost", false, "Show cost projections")
	allProjects := fs.Bool("all", false, "Include all watched projects")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	verbose := fs.Bool("verbose", false, "Show individual runs for high-frequency tasks")
	fs.Parse(args)

	if *days <= 0 {
		fmt.Fprintf(os.Stderr, "error: --days must be a positive integer (got: %d)\n", *days)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	now := time.Now()
	end := now.Add(time.Duration(*days) * 24 * time.Hour)

	type projectTodos struct {
		path  string
		todos []project.Todo
	}

	var projects []projectTodos

	if *allProjects {
		paths := loadWatchedProjectPaths()
		for _, p := range paths {
			proj, err := project.Load(p)
			if err != nil {
				log.Printf("warning: skipping project %s: %v", p, err)
				continue
			}
			todos, err := proj.LoadTodos()
			if err != nil {
				log.Printf("warning: skipping project %s: %v", p, err)
				continue
			}
			projects = append(projects, projectTodos{path: p, todos: todos})
		}
	} else {
		abs, err := filepath.Abs(".")
		if err != nil {
			log.Fatalf("bad path: %v", err)
		}
		proj, err := project.Load(abs)
		if err != nil {
			log.Fatalf("failed to load project: %v", err)
		}
		todos, err := proj.LoadTodos()
		if err != nil {
			log.Fatalf("failed to load todos: %v", err)
		}
		projects = append(projects, projectTodos{path: abs, todos: todos})
	}

	// Collect all todos and compute stats
	var allTodos []project.Todo
	allStats := make(map[string]forecast.TaskStats)

	for _, p := range projects {
		if *taskFilter != "" {
			var filtered []project.Todo
			filterName := *taskFilter
			if !strings.HasSuffix(filterName, ".md") {
				filterName += ".md"
			}
			for _, t := range p.todos {
				if t.Name == filterName {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) == 0 {
				continue
			}
			allTodos = append(allTodos, filtered...)
		} else {
			allTodos = append(allTodos, p.todos...)
		}

		stats := forecast.ComputeAllStats(p.path, p.todos, 10)
		for k, v := range stats {
			allStats[k] = v
		}
	}

	if *taskFilter != "" && len(allTodos) == 0 {
		fmt.Fprintf(os.Stderr, "error: task %q not found\n", *taskFilter)
		os.Exit(1)
	}

	if len(allTodos) == 0 {
		fmt.Println("No scheduled tasks found in project.")
		return
	}

	summary := forecast.ProjectRuns(allTodos, allStats, cfg, now, end)

	if summary.TotalRuns == 0 {
		fmt.Println("No runs in forecast period.")
		return
	}

	var contentionWindows []forecast.ContentionWindow
	if *contention {
		contentionWindows = forecast.DetectContention(summary, cfg.MaxWorkers)
	}

	if *jsonOutput {
		printForecastJSON(summary, contentionWindows, *cost, *contention)
		return
	}

	printForecastHuman(summary, contentionWindows, *cost, *contention, *verbose)
}

func printForecastHuman(summary forecast.ForecastSummary, contentionWindows []forecast.ContentionWindow, showCost, showContention, verbose bool) {
	fmt.Printf("FORECAST: %s to %s (%d days)\n\n",
		summary.StartTime.Format("2006-01-02"),
		summary.EndTime.Format("2006-01-02"),
		int(summary.EndTime.Sub(summary.StartTime).Hours()/24))

	// Check for high-frequency tasks
	highFreq := forecast.HighFrequencyTasks(summary, 50)

	// Print high-frequency task summaries first
	if len(highFreq) > 0 && !verbose {
		for name := range highFreq {
			ds := forecast.SummarizeHighFrequency(summary, name)
			costStr := ""
			if showCost && ds.DailyCostUSD > 0 {
				costStr = fmt.Sprintf("   $%.2f/day", ds.DailyCostUSD)
			}
			fmt.Printf("TASK             RUNS/DAY   DAILY DURATION%s\n", func() string {
				if showCost {
					return "   DAILY COST"
				}
				return ""
			}())
			fmt.Printf("%-16s %-10d %-16s%s\n", ds.TaskName, ds.RunsPerDay, ds.DailyDuration.Round(time.Second), costStr)
			fmt.Printf("  (use --task %s --verbose for details)\n\n", strings.TrimSuffix(name, ".md"))
		}
	}

	// Print regular runs
	if showCost {
		fmt.Printf("%-20s %-18s %-10s %s\n", "TIME", "TASK", "DURATION", "COST")
	} else {
		fmt.Printf("%-20s %-18s %s\n", "TIME", "TASK", "DURATION")
	}

	for _, r := range summary.Runs {
		if highFreq[r.TaskName] && !verbose {
			continue
		}

		marker := ""
		if r.IsHypothetical {
			marker = "* "
		}

		durationStr := "-"
		if r.EstimatedDuration > 0 {
			durationStr = r.EstimatedDuration.Round(time.Second).String()
		}

		timeStr := r.ScheduledTime.Format("Mon 01/02 15:04")
		taskName := strings.TrimSuffix(r.TaskName, ".md")

		if showCost {
			costStr := "-"
			if r.HasHistoricalData && r.EstimatedCostUSD > 0 {
				costStr = fmt.Sprintf("$%.2f", r.EstimatedCostUSD)
			} else if !r.HasHistoricalData {
				costStr = "no data"
			}
			fmt.Printf("%-20s %s%-18s %-10s %s\n", timeStr, marker, taskName, durationStr, costStr)
		} else {
			fmt.Printf("%-20s %s%-18s %s\n", timeStr, marker, taskName, durationStr)
		}
	}

	// Summary line
	fmt.Println()
	summaryParts := []string{
		fmt.Sprintf("%d runs", summary.TotalRuns),
		fmt.Sprintf("%s estimated runtime", summary.TotalDuration.Round(time.Second)),
	}
	if showCost && summary.TotalCostUSD > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("$%.2f estimated cost", summary.TotalCostUSD))
	}
	fmt.Printf("SUMMARY: %s\n", strings.Join(summaryParts, " | "))

	// Contention output
	if showContention {
		fmt.Println()
		if len(contentionWindows) == 0 {
			fmt.Println("No contention detected.")
		} else {
			fmt.Printf("CONTENTION WINDOWS (next %d days):\n\n", int(summary.EndTime.Sub(summary.StartTime).Hours()/24))
			fmt.Printf("%-20s %-12s %-10s %-10s %s\n", "TIME", "CONCURRENT", "WORKERS", "OVERFLOW", "TASKS")
			for _, w := range contentionWindows {
				taskList := strings.Join(w.Tasks, ", ")
				fmt.Printf("%-20s %-12d %-10d +%-9d %s\n",
					w.Start.Format("Mon 01/02 15:04"),
					w.PeakConcurrent,
					w.WorkerCount,
					w.Overflow,
					taskList)
			}
			fmt.Printf("\n%d contention windows found. Consider staggering schedules or increasing workers.\n", len(contentionWindows))
		}
	}
}

type forecastJSONOutput struct {
	Start             string                     `json:"start"`
	End               string                     `json:"end"`
	TotalRuns         int                        `json:"total_runs"`
	TotalDurationSecs float64                    `json:"total_duration_seconds"`
	TotalCostUSD      float64                    `json:"total_cost_usd,omitempty"`
	TotalInputTokens  int                        `json:"total_input_tokens,omitempty"`
	TotalOutputTokens int                        `json:"total_output_tokens,omitempty"`
	Runs              []forecastRunJSON           `json:"runs"`
	ContentionWindows []forecast.ContentionWindow `json:"contention_windows,omitempty"`
}

type forecastRunJSON struct {
	TaskID                 string  `json:"task_id"`
	TaskName               string  `json:"task_name"`
	ScheduledTime          string  `json:"scheduled_time"`
	EstimatedDurationSecs  float64 `json:"estimated_duration_seconds"`
	EstimatedCostUSD       float64 `json:"estimated_cost_usd,omitempty"`
	InputTokens            int     `json:"input_tokens,omitempty"`
	OutputTokens           int     `json:"output_tokens,omitempty"`
	HasHistoricalData      bool    `json:"has_historical_data"`
	IsHypothetical         bool    `json:"is_hypothetical"`
}

func printForecastJSON(summary forecast.ForecastSummary, contentionWindows []forecast.ContentionWindow, showCost, showContention bool) {
	out := forecastJSONOutput{
		Start:             summary.StartTime.Format(time.RFC3339),
		End:               summary.EndTime.Format(time.RFC3339),
		TotalRuns:         summary.TotalRuns,
		TotalDurationSecs: summary.TotalDuration.Seconds(),
	}

	if showCost {
		out.TotalCostUSD = summary.TotalCostUSD
		out.TotalInputTokens = summary.TotalInputTokens
		out.TotalOutputTokens = summary.TotalOutputTokens
	}

	for _, r := range summary.Runs {
		jr := forecastRunJSON{
			TaskID:                r.TaskID,
			TaskName:              r.TaskName,
			ScheduledTime:         r.ScheduledTime.Format(time.RFC3339),
			EstimatedDurationSecs: r.EstimatedDuration.Seconds(),
			HasHistoricalData:     r.HasHistoricalData,
			IsHypothetical:        r.IsHypothetical,
		}
		if showCost {
			jr.EstimatedCostUSD = r.EstimatedCostUSD
			jr.InputTokens = r.InputTokens
			jr.OutputTokens = r.OutputTokens
		}
		out.Runs = append(out.Runs, jr)
	}

	if showContention {
		out.ContentionWindows = contentionWindows
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal JSON: %v", err)
	}
	fmt.Println(string(data))
}

// loadWatchedProjectPaths reads watched project paths from ~/.anvil/watched/
func loadWatchedProjectPaths() []string {
	watchedDir := config.WatchedDir()
	dirs, err := os.ReadDir(watchedDir)
	if err != nil {
		return nil
	}

	var paths []string
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		// Each watched dir contains markdown files with frontmatter "path: /some/project"
		dirPath := filepath.Join(watchedDir, d.Name())
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
			if err != nil {
				continue
			}
			content := string(data)
			// Parse simple frontmatter to extract path
			if idx := strings.Index(content, "path:"); idx >= 0 {
				line := content[idx:]
				if nl := strings.Index(line, "\n"); nl >= 0 {
					line = line[:nl]
				}
				p := strings.TrimSpace(strings.TrimPrefix(line, "path:"))
				if p != "" {
					paths = append(paths, p)
				}
			}
			break // only first file per dir
		}
	}
	return paths
}
