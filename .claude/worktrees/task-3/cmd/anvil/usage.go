package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func usageCmd(args []string) {
	var projectFilter string
	var taskFilter string
	var sinceStr string
	var showMetrics bool
	var topN int
	var jsonOut bool

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				projectFilter = args[i+1]
				i += 2
			} else {
				fmt.Fprintln(os.Stderr, "--project requires a value")
				os.Exit(1)
			}
		case "--task":
			if i+1 < len(args) {
				taskFilter = args[i+1]
				i += 2
			} else {
				fmt.Fprintln(os.Stderr, "--task requires a value")
				os.Exit(1)
			}
		case "--since":
			if i+1 < len(args) {
				sinceStr = args[i+1]
				i += 2
			} else {
				fmt.Fprintln(os.Stderr, "--since requires a value")
				os.Exit(1)
			}
		case "--metrics":
			showMetrics = true
			i++
		case "--top":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err != nil || n <= 0 {
					fmt.Fprintln(os.Stderr, "--top requires a positive number")
					os.Exit(1)
				}
				topN = n
				i += 2
			} else {
				fmt.Fprintln(os.Stderr, "--top requires a value")
				os.Exit(1)
			}
		case "--json":
			jsonOut = true
			i++
		case "-h", "--help":
			if showMetrics {
				fmt.Fprintf(os.Stderr, `Usage: anvil usage --metrics [options]

Show task runtime metrics including total runtime, average execution time,
run count, and success rates.

Options:
  --project <path>   Filter to a specific project (default: all watched projects)
  --task <name>     Filter to a specific task name
  --since <date>    Show metrics since date (YYYY-MM-DD, default: 30 days ago)
  --top <N>         Show only top N tasks by runtime
  --json            Output as JSON
`)
			} else {
				fmt.Fprintf(os.Stderr, `Usage: anvil usage [options]

Show LLM token usage and estimated costs across tasks and projects.

Options:
  --project <path>   Filter to a specific project (default: all watched projects)
  --task <name>      Filter to a specific task name
  --since <date>     Show usage since date (YYYY-MM-DD, default: 7 days ago)
  --metrics          Show task runtime metrics (total runtime, success rate, etc.)
  --top <N>          Limit output to top N tasks (use with --metrics)
  --json             Output as JSON
`)
			}
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	// Default: last 7 days for usage, 30 days for metrics
	sinceDays := 7
	if showMetrics {
		sinceDays = 30
	}
	since := time.Now().AddDate(0, 0, -sinceDays)
	if sinceStr != "" {
		parsed, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid date %q (expected YYYY-MM-DD): %v\n", sinceStr, err)
			os.Exit(1)
		}
		since = parsed
	}

	// Resolve project filter to absolute path
	if projectFilter != "" {
		abs, err := filepath.Abs(projectFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad project path: %v\n", err)
			os.Exit(1)
		}
		projectFilter = abs
	}

	// Discover projects
	var projectPaths []string
	if projectFilter != "" {
		projectPaths = []string{projectFilter}
	} else {
		watched, err := loadAllWatched()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load watched projects: %v\n", err)
			os.Exit(1)
		}
		for _, w := range watched {
			projectPaths = append(projectPaths, w.Path)
		}
	}

	if len(projectPaths) == 0 {
		fmt.Println("No watched projects found.")
		return
	}

	// Load global config for cost rates
	cfg, _ := config.Load()
	inputRate := cfg.InputTokenRate
	outputRate := cfg.OutputTokenRate
	if inputRate <= 0 {
		inputRate = 3.0
	}
	if outputRate <= 0 {
		outputRate = 15.0
	}

	type taskUsage struct {
		Project      string
		TaskName     string
		Runs         int
		InputTokens  int
		OutputTokens int
		Cost         float64
	}

	var allUsage []taskUsage
	var totalRuns int
	var totalInput, totalOutput int
	var totalCost float64

	for _, projPath := range projectPaths {
		proj, err := project.Load(projPath)
		if err != nil {
			continue
		}
		todos, err := proj.LoadTodos()
		if err != nil {
			continue
		}
		projName := filepath.Base(projPath)

		for _, todo := range todos {
			if taskFilter != "" && todo.Name != taskFilter {
				continue
			}
			records, err := project.ReadAllRunRecords(projPath, todo.ID)
			if err != nil {
				continue
			}

			var tu taskUsage
			tu.Project = projName
			tu.TaskName = todo.Name

			for _, rec := range records {
				if rec.Started.Before(since) {
					continue
				}
				tu.Runs++
				tu.InputTokens += rec.InputTokens
				tu.OutputTokens += rec.OutputTokens
				tu.Cost += rec.EstimatedCostUSD
			}

			// Recalculate cost if records had zero cost but had tokens
			// (handles old records that may have tokens but no cost)
			if tu.Cost == 0 && (tu.InputTokens > 0 || tu.OutputTokens > 0) {
				tu.Cost = float64(tu.InputTokens)/1_000_000*inputRate +
					float64(tu.OutputTokens)/1_000_000*outputRate
			}

			if tu.Runs > 0 {
				allUsage = append(allUsage, tu)
				totalRuns += tu.Runs
				totalInput += tu.InputTokens
				totalOutput += tu.OutputTokens
				totalCost += tu.Cost
			}
		}
	}

	if totalRuns == 0 {
		fmt.Printf("No runs found since %s.\n", since.Format("2006-01-02"))
		return
	}

	// Sort by cost descending
	sort.Slice(allUsage, func(i, j int) bool {
		return allUsage[i].Cost > allUsage[j].Cost
	})

	// Print summary
	fmt.Printf("Token usage since %s\n", since.Format("2006-01-02"))
	fmt.Printf("Rates: $%.2f/1M input, $%.2f/1M output\n\n", inputRate, outputRate)

	fmt.Printf("%-30s %-6s %12s %12s %10s\n", "TASK", "RUNS", "INPUT", "OUTPUT", "COST")
	fmt.Printf("%-30s %-6s %12s %12s %10s\n", "----", "----", "-----", "------", "----")

	limit := len(allUsage)
	if limit > 15 {
		limit = 15
	}
	for _, tu := range allUsage[:limit] {
		name := tu.TaskName
		if len(name) > 27 {
			name = name[:27] + "..."
		}
		if len(projectPaths) > 1 {
			label := tu.Project + "/" + name
			if len(label) > 30 {
				label = label[:27] + "..."
			}
			name = label
		}
		fmt.Printf("%-30s %-6d %12s %12s %10s\n",
			name,
			tu.Runs,
			formatTokens(tu.InputTokens),
			formatTokens(tu.OutputTokens),
			formatCost(tu.Cost))
	}
	if len(allUsage) > limit {
		fmt.Printf("  ... and %d more tasks\n", len(allUsage)-limit)
	}

	fmt.Printf("\n%-30s %-6d %12s %12s %10s\n",
		"TOTAL",
		totalRuns,
		formatTokens(totalInput),
		formatTokens(totalOutput),
		formatCost(totalCost))

	// If --metrics requested, show runtime metrics
	if showMetrics {
		type taskMetrics struct {
			Project      string
			TaskName     string
			TotalRuntime time.Duration
			AvgRuntime   time.Duration
			Runs         int
			Successes    int
			SuccessRate  float64
			InputTokens  int
			OutputTokens int
			Cost         float64
		}

		var allMetrics []taskMetrics
		var projectTotals map[string]struct {
			runtime time.Duration
			runs    int
			cost    float64
		}
		projectTotals = make(map[string]struct {
			runtime time.Duration
			runs    int
			cost    float64
		})
		var grandTotalRuntime time.Duration

		// Reload data for metrics
		for _, projPath := range projectPaths {
			proj, err := project.Load(projPath)
			if err != nil {
				continue
			}
			todos, err := proj.LoadTodos()
			if err != nil {
				continue
			}
			projName := filepath.Base(projPath)

			for _, todo := range todos {
				if taskFilter != "" && todo.Name != taskFilter {
					continue
				}
				records, err := project.ReadAllRunRecords(projPath, todo.ID)
				if err != nil {
					continue
				}

				var tm taskMetrics
				tm.Project = projName
				tm.TaskName = todo.Name

				for _, rec := range records {
					if rec.Started.Before(since) {
						continue
					}
					if rec.Finished.IsZero() {
						continue // skip incomplete runs
					}
					tm.Runs++
					runtime := rec.Finished.Sub(rec.Started)
					tm.TotalRuntime += runtime
					if rec.Success {
						tm.Successes++
					}
					tm.InputTokens += rec.InputTokens
					tm.OutputTokens += rec.OutputTokens
					tm.Cost += rec.EstimatedCostUSD
				}

				if tm.Runs > 0 {
					tm.AvgRuntime = tm.TotalRuntime / time.Duration(tm.Runs)
					tm.SuccessRate = float64(tm.Successes) / float64(tm.Runs)

					// Recalculate cost if needed
					if tm.Cost == 0 && (tm.InputTokens > 0 || tm.OutputTokens > 0) {
						tm.Cost = float64(tm.InputTokens)/1_000_000*inputRate +
							float64(tm.OutputTokens)/1_000_000*outputRate
					}

					allMetrics = append(allMetrics, tm)
					grandTotalRuntime += tm.TotalRuntime

					// Accumulate project totals
					if pt, ok := projectTotals[projName]; ok {
						pt.runtime += tm.TotalRuntime
						pt.runs += tm.Runs
						pt.cost += tm.Cost
						projectTotals[projName] = pt
					} else {
						projectTotals[projName] = struct {
							runtime time.Duration
							runs    int
							cost    float64
						}{runtime: tm.TotalRuntime, runs: tm.Runs, cost: tm.Cost}
					}
				}
			}
		}

		if len(allMetrics) == 0 {
			fmt.Printf("\nNo completed runs found since %s.\n", since.Format("2006-01-02"))
			return
		}

		// Sort by total runtime descending
		sort.Slice(allMetrics, func(i, j int) bool {
			return allMetrics[i].TotalRuntime > allMetrics[j].TotalRuntime
		})

		// Apply topN limit
		displayLimit := len(allMetrics)
		if topN > 0 && topN < displayLimit {
			displayLimit = topN
		}

		// Output JSON if requested
		if jsonOut {
			type jsonOutput struct {
				Period    string        `json:"period"`
				Since     string        `json:"since"`
				Tasks     []taskMetrics `json:"tasks"`
				TotalCost float64       `json:"total_cost"`
				TotalRuns int           `json:"total_runs"`
			}
			var totalCost float64
			var totalRuns int
			for _, tm := range allMetrics {
				totalCost += tm.Cost
				totalRuns += tm.Runs
			}
			jo := jsonOutput{
				Period:    fmt.Sprintf("%dd", sinceDays),
				Since:     since.Format("2006-01-02"),
				Tasks:     allMetrics[:displayLimit],
				TotalCost: totalCost,
				TotalRuns: totalRuns,
			}
			data, err := json.MarshalIndent(jo, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal JSON: %v", err)
			}
			fmt.Println(string(data))
			return
		}

		// Print human-readable metrics
		fmt.Printf("\n")
		fmt.Printf("TASK RUNTIME SUMMARY\n")
		fmt.Printf("====================\n")
		fmt.Printf("Period: %s to %s\n\n", since.Format("2006-01-02"), time.Now().Format("2006-01-02"))

		fmt.Printf("%-30s %10s %10s %6s %8s\n", "TASK", "TOTAL", "AVG", "RUNS", "SUCCESS")
		fmt.Printf("%-30s %10s %10s %6s %8s\n", "----", "-----", "---", "----", "-------")

		for _, tm := range allMetrics[:displayLimit] {
			name := tm.TaskName
			if len(name) > 27 {
				name = name[:27] + "..."
			}
			if len(projectPaths) > 1 {
				label := tm.Project + "/" + name
				if len(label) > 30 {
					label = label[:27] + "..."
				}
				name = label
			}
			successPct := int(tm.SuccessRate * 100)
			fmt.Printf("%-30s %10s %10s %6d %7d%%\n",
				name,
				formatDuration(tm.TotalRuntime),
				formatDuration(tm.AvgRuntime),
				tm.Runs,
				successPct)
		}

		if len(allMetrics) > displayLimit {
			fmt.Printf("  ... and %d more tasks\n", len(allMetrics)-displayLimit)
		}

		// Project totals
		if len(projectTotals) > 1 {
			fmt.Printf("\nBY PROJECT\n")
			fmt.Printf("==========\n")
			var sortedProjects []string
			for p := range projectTotals {
				sortedProjects = append(sortedProjects, p)
			}
			sort.Slice(sortedProjects, func(i, j int) bool {
				return projectTotals[sortedProjects[i]].runtime > projectTotals[sortedProjects[j]].runtime
			})

			for _, p := range sortedProjects {
				pt := projectTotals[p]
				fmt.Printf("%-40s %10s %6d runs %10s\n",
					p,
					formatDuration(pt.runtime),
					pt.runs,
					formatCost(pt.cost))
			}
		}

		// Failure analysis
		fmt.Printf("\nRUNTIME\n")
		fmt.Printf("=======\n")
		fmt.Printf("Total: %s across %d runs\n", formatDuration(grandTotalRuntime), totalRuns)
		fmt.Printf("Cost:  %s\n", formatCost(totalCost))
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "N/A"
	}
	if d >= time.Hour {
		hours := d.Hours()
		return fmt.Sprintf("%.1fh", hours)
	}
	if d >= time.Minute {
		mins := d.Minutes()
		return fmt.Sprintf("%.1fm", mins)
	}
	secs := d.Seconds()
	return fmt.Sprintf("%.0fs", secs)
}

func formatTokens(n int) string {
	if n == 0 {
		return "N/A"
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCost(c float64) string {
	if c == 0 {
		return "N/A"
	}
	if c < 0.01 {
		return fmt.Sprintf("$%.4f", c)
	}
	return fmt.Sprintf("$%.2f", c)
}

// --- helpers ---
