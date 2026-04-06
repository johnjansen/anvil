package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/forecast"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/tools"
	"gopkg.in/yaml.v3"
)

func taskCreateCmd(args []string) {
	priority := 1
	schedule := ""
	preCheck := ""
	allowedTools := ""
	maxConcurrent := 1
	skipPermissions := false
	filePath := ""
	readStdin := false
	onceFlag := false
	dryRun := false
	strict := false
	noOverlapCheck := false
	dryRunJSON := false
	costOnly := false
	templateName := ""

	// Track which flags were explicitly set on the CLI so they take precedence over frontmatter/template.
	prioritySet := false
	scheduleSet := false
	preCheckSet := false
	allowedToolsSet := false
	maxConcurrentSet := false

	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t", "--template":
			if i+1 >= len(args) {
				log.Fatal("missing value for -t/--template")
			}
			i++
			templateName = args[i]
		case "-p", "--priority":
			if i+1 >= len(args) {
				log.Fatal("missing value for -p/--priority")
			}
			i++
			n := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					log.Fatalf("invalid priority: %s (must be 0-9)", args[i])
				}
				n = n*10 + int(c-'0')
			}
			if n > 9 {
				log.Fatalf("priority must be 0-9, got %d", n)
			}
			priority = n
			prioritySet = true
		case "-s", "--schedule":
			if i+1 >= len(args) {
				log.Fatal("missing value for -s/--schedule")
			}
			i++
			schedule = args[i]
			scheduleSet = true
		case "-o", "--once":
			onceFlag = true
		case "-n", "--dry-run":
			dryRun = true
		case "--cost-only":
			costOnly = true
		case "--json":
			dryRunJSON = true
		case "--pre-check":
			if i+1 >= len(args) {
				log.Fatal("missing value for --pre-check")
			}
			i++
			preCheck = args[i]
			preCheckSet = true
		case "--allowed-tools":
			if i+1 >= len(args) {
				log.Fatal("missing value for --allowed-tools")
			}
			i++
			allowedTools = args[i]
			allowedToolsSet = true
		case "--max-concurrent":
			if i+1 >= len(args) {
				log.Fatal("missing value for --max-concurrent")
			}
			i++
			n := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					log.Fatalf("invalid max-concurrent: %s (must be a number)", args[i])
				}
				n = n*10 + int(c-'0')
			}
			maxConcurrent = n
			maxConcurrentSet = true
		case "--skip-permissions":
			skipPermissions = true
		case "-f", "--file":
			if i+1 >= len(args) {
				log.Fatal("missing value for -f/--file")
			}
			i++
			filePath = args[i]
		case "--strict":
			strict = true
		case "--no-overlap-check":
			noOverlapCheck = true
		case "-":
			readStdin = true
		default:
			rest = append(rest, args[i])
		}
	}

	// Validate --once and --schedule are not both set.
	if onceFlag && scheduleSet {
		log.Fatal("cannot use both --once and --schedule")
	}

	// --once explicitly sets an empty schedule (one-shot task).
	if onceFlag {
		schedule = ""
		scheduleSet = true
	}

	// Handle --dry-run or --cost-only: show impact analysis without creating task.
	if dryRun || costOnly {
		abs, err := filepath.Abs(".")
		if err != nil {
			log.Fatalf("bad path: %v", err)
		}
		var todos []project.Todo
		if schedule != "" {
			if proj, err := project.Load(abs); err == nil {
				todos, _ = proj.LoadTodos()
			}
		}

		// Get task text (either from args or file/stdin)
		var taskText string
		switch {
		case filePath != "":
			// Read task content from file.
			data, err := os.ReadFile(filePath)
			if err != nil {
				log.Fatalf("reading file %s: %v", filePath, err)
			}
			taskText = string(data)
		case readStdin:
			// Read task content from stdin.
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatalf("reading stdin: %v", err)
			}
			taskText = string(data)
		default:
			if len(rest) == 0 && !dryRun && !costOnly {
				log.Fatal("usage: anvil add [-p priority] [-s schedule | --once] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [-f file | -] <task text>")
			}
			taskText = strings.Join(rest, " ")
		}

		// If --cost-only, show cost estimation
		if costOnly {
			// Estimate tokens and cost
			printCostEstimate(taskText, schedule, abs)
			return
		}

		// Regular dry-run behavior
		report := analyzeImpactForDryRun(schedule, todos, taskText)
		if dryRunJSON {
			printImpactJSON(report)
		} else {
			printImpactReport(&report, false)
		}

		// Add forecast impact analysis
		if schedule != "" && len(todos) > 0 {
			printForecastImpact(abs, schedule, taskText, todos, dryRunJSON)
		}
		return
	}

	// Load template if specified and apply its values (CLI flags take precedence).
	if templateName != "" {
		abs, err := filepath.Abs(".")
		if err != nil {
			log.Fatalf("bad path: %v", err)
		}
		tmpl, err := project.LoadTemplate(abs, templateName)
		if err != nil {
			log.Fatalf("failed to load template: %v", err)
		}
		// Apply template values only if not explicitly set via CLI flags
		if !prioritySet && tmpl.Spec.Priority > 0 {
			priority = tmpl.Spec.Priority
		}
		if !scheduleSet && tmpl.Spec.Schedule != "" {
			schedule = tmpl.Spec.Schedule
		}
		if !preCheckSet && tmpl.Spec.PreCheck != "" {
			preCheck = tmpl.Spec.PreCheck
		}
		if !allowedToolsSet && len(tmpl.Spec.AllowedTools) > 0 {
			allowedTools = strings.Join(tmpl.Spec.AllowedTools, ",")
		}
		if !maxConcurrentSet && tmpl.Spec.MaxConcurrent > 0 {
			maxConcurrent = tmpl.Spec.MaxConcurrent
		}
		if !skipPermissions && tmpl.Spec.SkipPermissions {
			skipPermissions = true
		}
		// Store template labels for later use (in AddTodo)
		// These will be used when creating the task
	}

	var taskText string

	switch {
	case filePath != "":
		// Read task content from file.
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("reading file %s: %v", filePath, err)
		}
		taskText = string(data)
	case readStdin:
		// Read task content from stdin.
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("reading stdin: %v", err)
		}
		taskText = string(data)
	default:
		if len(rest) == 0 {
			log.Fatal("usage: anvil add [-p priority] [-s schedule | --once] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [-f file | -] <task text>")
		}
		taskText = strings.Join(rest, " ")
	}

	// If file/stdin content has positional args too, that's an error.
	if (filePath != "" || readStdin) && len(rest) > 0 {
		log.Fatal("cannot combine file/stdin input with positional task text")
	}

	// Parse frontmatter from file/stdin content and merge with CLI flags.
	// CLI flags take precedence over frontmatter values.
	if filePath != "" || readStdin {
		taskText, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions = parseFrontmatterAndMerge(
			taskText, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions,
			prioritySet, scheduleSet, preCheckSet, allowedToolsSet, maxConcurrentSet,
		)
	}

	if strings.TrimSpace(taskText) == "" {
		log.Fatal("task content must not be empty")
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	if _, err := os.Stat(filepath.Join(abs, ".anvil", "todos")); os.IsNotExist(err) {
		if _, err := project.Init(abs, tools.FS, false); err != nil {
			log.Fatalf("failed to init project: %v", err)
		}
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	// Check for schedule overlaps with existing tasks using shared conflict detection.
	if schedule != "" && !noOverlapCheck {
		if _, parseErr := cron.Parse(schedule); parseErr == nil {
			todos, _ := proj.LoadTodos()
			conflicts := computeConflicts(schedule, todos)
			if len(conflicts) > 0 {
				var overlapping []string
				for _, c := range conflicts {
					overlapping = append(overlapping, fmt.Sprintf("%s (%s)", c.TaskName, c.Schedule))
				}
				if strict {
					fmt.Fprintf(os.Stderr, "Error: Schedule overlaps with %d existing task(s):\n", len(overlapping))
					for _, o := range overlapping {
						fmt.Fprintf(os.Stderr, "  - %s\n", o)
					}
					fmt.Fprintf(os.Stderr, "\nRun 'anvil task overlaps' for full conflict report.\n")
					suggestStagger(schedule, len(overlapping))
					os.Exit(1)
				}
				// Non-strict: warn but continue
				severity := "Note"
				if len(conflicts) >= 3 {
					severity = "Warning"
				}
				fmt.Fprintf(os.Stderr, "%s: This schedule overlaps with %d existing task(s) that run at similar times:\n", severity, len(conflicts))
				for _, o := range overlapping {
					fmt.Fprintf(os.Stderr, "  - %s\n", o)
				}
				suggestStagger(schedule, len(conflicts))
				fmt.Fprintln(os.Stderr)
			}
		}
	}


	relPath, err := proj.AddTodo(priority, schedule, taskText, preCheck, allowedTools, maxConcurrent, skipPermissions, "")
	if err != nil {
		log.Fatalf("failed to add todo: %v", err)
	}

	fmt.Printf("added %s\n", relPath)

	// Show next run time for scheduled tasks.
	if schedule != "" {
		if p, err := cron.Parse(schedule); err == nil {
			if next, err := p.Next(time.Now()); err == nil {
				until := time.Until(next).Round(time.Minute)
				fmt.Fprintf(os.Stderr, "Next run: %s (%s from now)\n", next.Format("Mon 15:04"), until)
			}
		}
	}

	// Warn if daemon is not running (only for scheduled tasks)
	if schedule != "" && !daemon.IsDaemonRunning() {
		fmt.Fprintf(os.Stderr, "⚠ Daemon is not running. Run 'anvil watch' to start executing tasks.\n")
	}
}

func dispatchCmd(args []string) {
	// Handle -h/--help before creating task
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintf(os.Stderr, `usage: anvil dispatch [-p priority] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [--timeout DURATION] [--json] [--quiet] [--no-wait] [-f file | -] <task text>

Add a one-shot task and wait for completion, returning the result.

Options:
  -p, --priority n           Priority 0-9 (default: 1)
  -o, --once                 Create a one-shot task (default for dispatch)
  --pre-check cmd            Command to run before task execution
  --allowed-tools tools      Comma-separated list of allowed tools (e.g. "Bash,Read" or scoped "Bash(gh:*)", "Read(.claude/commands/*)")
  --max-concurrent n         Max concurrent runs (default: 1)
  --skip-permissions         Skip permission checks
  --timeout DURATION         Max wait time before exit code 2 (default: 30m)
  --json                     Output full RunRecord JSON instead of just summary
  --quiet                    Suppress progress/status on stderr
  --no-wait                  Just add the task and print the UUID (for async use)
  -f, --file path            Read task content from a file
  -                          Read task content from stdin

Examples:
  anvil dispatch --skip-permissions "Review PR #42"
  anvil dispatch --timeout 10m --skip-permissions "Run the test suite"
  anvil dispatch --no-wait --skip-permissions "Long running analysis"
  anvil dispatch --json --skip-permissions "Analyze codebase" | jq '.output_summary'
  anvil dispatch --skip-permissions -f complex-prompt.md
  echo "prompt" | anvil dispatch --skip-permissions -
`)
			os.Exit(0)
		}
	}

	// Parse command line arguments
	priority := 1
	preCheck := ""
	allowedTools := ""
	maxConcurrent := 1
	skipPermissions := false
	filePath := ""
	readStdin := false
	timeoutDur := 30 * time.Minute
	jsonOutput := false
	quiet := false
	noWait := false

	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p", "--priority":
			if i+1 >= len(args) {
				log.Fatal("missing value for -p/--priority")
			}
			i++
			n := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					log.Fatalf("invalid priority: %s (must be 0-9)", args[i])
				}
				n = n*10 + int(c-'0')
			}
			if n > 9 {
				log.Fatalf("priority must be 0-9, got %d", n)
			}
			priority = n
		case "--pre-check":
			if i+1 >= len(args) {
				log.Fatal("missing value for --pre-check")
			}
			i++
			preCheck = args[i]
		case "--allowed-tools":
			if i+1 >= len(args) {
				log.Fatal("missing value for --allowed-tools")
			}
			i++
			allowedTools = args[i]
		case "--max-concurrent":
			if i+1 >= len(args) {
				log.Fatal("missing value for --max-concurrent")
			}
			i++
			n := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					log.Fatalf("invalid max-concurrent: %s (must be a number)", args[i])
				}
				n = n*10 + int(c-'0')
			}
			maxConcurrent = n
		case "--skip-permissions":
			skipPermissions = true
		case "--timeout":
			if i+1 >= len(args) {
				log.Fatal("missing value for --timeout")
			}
			i++
			var err error
			timeoutDur, err = time.ParseDuration(args[i])
			if err != nil {
				log.Fatalf("invalid timeout duration: %v", err)
			}
		case "--json":
			jsonOutput = true
		case "--quiet":
			quiet = true
		case "--no-wait":
			noWait = true
		case "-f", "--file":
			if i+1 >= len(args) {
				log.Fatal("missing value for -f/--file")
			}
			i++
			filePath = args[i]
		case "-":
			readStdin = true
		default:
			rest = append(rest, args[i])
		}
	}

	var taskText string

	switch {
	case filePath != "":
		// Read task content from file.
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("reading file %s: %v", filePath, err)
		}
		taskText = string(data)
	case readStdin:
		// Read task content from stdin.
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("reading stdin: %v", err)
		}
		taskText = string(data)
	default:
		if len(rest) == 0 {
			log.Fatal("usage: anvil dispatch [-p priority] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [--timeout DURATION] [--json] [--quiet] [--no-wait] [-f file | -] <task text>")
		}
		taskText = strings.Join(rest, " ")
	}

	// If file/stdin content has positional args too, that's an error.
	if (filePath != "" || readStdin) && len(rest) > 0 {
		log.Fatal("cannot combine file/stdin input with positional task text")
	}

	if strings.TrimSpace(taskText) == "" {
		log.Fatal("task content must not be empty")
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	if _, err := os.Stat(filepath.Join(abs, ".anvil", "todos")); os.IsNotExist(err) {
		if _, err := project.Init(abs, tools.FS, false); err != nil {
			log.Fatalf("failed to init project: %v", err)
		}
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	// Create the one-shot task using AddTodoWithID to get both path and ID
	relPath, taskID, err := proj.AddTodoWithID(priority, "", taskText, preCheck, allowedTools, maxConcurrent, skipPermissions, "")
	if err != nil {
		log.Fatalf("failed to add todo: %v", err)
	}

	// Print the task ID to stderr immediately
	if !quiet {
		fmt.Fprintf(os.Stderr, "added task %s with ID %s\n", relPath, taskID)
	}

	// If --no-wait is specified, just print the task ID and exit
	if noWait {
		fmt.Println(taskID)
		return
	}

	// Wait for the task to complete
	if !quiet {
		fmt.Fprintf(os.Stderr, "waiting for task completion...\n")
	}

	// Set up timeout if specified
	var deadline <-chan time.Time
	if timeoutDur > 0 {
		deadline = time.After(timeoutDur)
	}

	// Poll until the task is no longer running or timeout occurs
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			if !quiet {
				fmt.Fprintf(os.Stderr, "timed out after %s\n", timeoutDur)
			}
			os.Exit(2)
		case <-ticker.C:
			// Check if the task is still running
			tasks, err := daemon.SendPsRequest()
			if err != nil {
				// Daemon may have gone away — treat as task completed
				if !quiet {
					fmt.Fprintf(os.Stderr, "daemon unreachable, assuming task completed\n")
				}
				// Try to get the result anyway
				exitCode := checkTaskResult(relPath)
				if exitCode == 0 {
					if !quiet {
						fmt.Fprintf(os.Stderr, "task completed successfully\n")
					}
				} else {
					if !quiet {
						fmt.Fprintf(os.Stderr, "task failed\n")
					}
				}
				os.Exit(exitCode)
			}

			// Check if our task is still running
			stillRunning := false
			for _, t := range tasks {
				if t.Name == filepath.Base(relPath) {
					stillRunning = true
					break
				}
			}

			if !stillRunning {
				// Task is no longer running — check last run record for exit status and output
				rec, err := project.ReadCurrentRunRecord(abs, taskID)
				if err != nil {
					// No record found — assume success
					if jsonOutput {
						fmt.Printf("{\"success\":true,\"output_summary\":\"Task completed successfully but no detailed record available\"}\n")
					} else {
						fmt.Println("Task completed successfully but no detailed record available")
					}
					os.Exit(0)
				}

				if jsonOutput {
					// Output the full RunRecord as JSON
					jsonData, err := json.Marshal(rec)
					if err != nil {
						log.Fatalf("failed to marshal run record: %v", err)
					}
					fmt.Println(string(jsonData))
				} else {
					// Output just the summary
					fmt.Println(rec.OutputSummary)
				}

				if rec.Success {
					os.Exit(0)
				} else {
					os.Exit(1)
				}
			}
		}
	}
}

// parseFrontmatterAndMerge extracts YAML frontmatter from content and merges
// it with CLI flags. CLI flags take precedence: if a flag was explicitly set on
// the command line, the frontmatter value for that field is ignored.
// Returns the body (without frontmatter) and the merged configuration values.
func parseFrontmatterAndMerge(
	content string,
	priority int, schedule, preCheck, allowedTools string, maxConcurrent int, skipPermissions bool,
	prioritySet, scheduleSet, preCheckSet, allowedToolsSet, maxConcurrentSet bool,
) (string, int, string, string, string, int, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return content, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions
	}

	parts := strings.SplitN(content[4:], "\n---\n", 2)
	if len(parts) != 2 {
		// No closing delimiter found; treat entire content as body.
		return content, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions
	}

	fm := parts[0]
	body := parts[1]

	var fmData struct {
		Priority        *int   `yaml:"priority"`
		Schedule        string `yaml:"schedule"`
		PreCheck        string `yaml:"pre_check"`
		AllowedTools    string `yaml:"allowed_tools"`
		MaxConcurrent   *int   `yaml:"max_concurrent"`
		SkipPermissions bool   `yaml:"skip_permissions"`
	}
	if err := yaml.Unmarshal([]byte(fm), &fmData); err != nil {
		// If frontmatter is invalid YAML, treat the whole thing as body.
		return content, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions
	}

	// Merge: CLI flags take precedence over frontmatter.
	if !prioritySet && fmData.Priority != nil {
		p := *fmData.Priority
		if p >= 0 && p <= 9 {
			priority = p
		}
	}
	if !scheduleSet && fmData.Schedule != "" {
		schedule = fmData.Schedule
	}
	if !preCheckSet && fmData.PreCheck != "" {
		preCheck = fmData.PreCheck
	}
	if !allowedToolsSet && fmData.AllowedTools != "" {
		allowedTools = fmData.AllowedTools
	}
	if !maxConcurrentSet && fmData.MaxConcurrent != nil {
		maxConcurrent = *fmData.MaxConcurrent
	}
	if fmData.SkipPermissions {
		skipPermissions = true
	}

	return body, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions
}

func printForecastImpact(projectPath, schedule, taskName string, existingTodos []project.Todo, jsonOutput bool) {
	cfg, err := config.Load()
	if err != nil {
		return
	}

	now := time.Now()
	end := now.Add(7 * 24 * time.Hour)

	stats := forecast.ComputeAllStats(projectPath, existingTodos, 10)

	// Forecast without the new task
	before := forecast.ProjectRuns(existingTodos, stats, cfg, now, end)

	// Create hypothetical todo
	hypo := project.Todo{
		ID:       "hypothetical",
		Name:     taskName,
		Schedule: schedule,
	}

	withHypo := make([]project.Todo, len(existingTodos)+1)
	copy(withHypo, existingTodos)
	withHypo[len(existingTodos)] = hypo
	after := forecast.ProjectRuns(withHypo, stats, cfg, now, end)

	// Mark hypothetical runs
	for i := range after.Runs {
		if after.Runs[i].TaskID == "hypothetical" {
			after.Runs[i].IsHypothetical = true
		}
	}

	contentionBefore := forecast.DetectContention(before, cfg.MaxWorkers)
	contentionAfter := forecast.DetectContention(after, cfg.MaxWorkers)

	if jsonOutput {
		type impactJSON struct {
			BeforeRuns      int `json:"before_runs"`
			AfterRuns       int `json:"after_runs"`
			NewContentions  int `json:"new_contention_windows"`
		}
		data, _ := json.MarshalIndent(impactJSON{
			BeforeRuns:     before.TotalRuns,
			AfterRuns:      after.TotalRuns,
			NewContentions: len(contentionAfter) - len(contentionBefore),
		}, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("\nIMPACT: %d → %d runs (+%d) | %s → %s runtime\n",
		before.TotalRuns, after.TotalRuns, after.TotalRuns-before.TotalRuns,
		before.TotalDuration.Round(time.Second), after.TotalDuration.Round(time.Second))

	newContentions := len(contentionAfter) - len(contentionBefore)
	if newContentions > 0 {
		fmt.Printf("CONTENTION: %d new contention window(s) introduced\n", newContentions)
		for _, w := range contentionAfter[len(contentionBefore):] {
			fmt.Printf("  %s (%d concurrent, %d workers)\n",
				w.Start.Format("Mon 01/02 15:04"),
				w.PeakConcurrent, w.WorkerCount)
		}
	}
}
