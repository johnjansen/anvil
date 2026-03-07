package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

func taskLsCmd(args []string) {
	allProjects := false
	jsonOutput := false
	matchPattern := ""
	labelFilter := ""

	// Parse flags: --match/-m for pattern, --all/-a for all projects, --json for JSON output, --label/-l for label filter
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--all" || a == "-a" {
			allProjects = true
		} else if a == "--json" {
			jsonOutput = true
		} else if a == "--match" || a == "-m" {
			if i+1 >= len(args) {
				log.Fatal("missing value for --match/-m")
			}
			i++
			matchPattern = args[i]
		} else if strings.HasPrefix(a, "--match=") || strings.HasPrefix(a, "-m=") {
			matchPattern = strings.TrimPrefix(strings.TrimPrefix(a, "--match="), "-m=")
		} else if a == "--label" || a == "-l" {
			if i+1 >= len(args) {
				log.Fatal("missing value for --label/-l")
			}
			i++
			labelFilter = strings.ToLower(args[i])
		} else if strings.HasPrefix(a, "--label=") || strings.HasPrefix(a, "-l=") {
			labelFilter = strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(a, "--label="), "-l="))
		} else {
			filteredArgs = append(filteredArgs, a)
		}
	}

	// Gather running tasks once
	var runningTasks []daemon.TaskInfo
	if daemon.IsDaemonRunning() {
		runningTasks, _ = daemon.SendPsRequest()
	}
	runningByID := make(map[string]daemon.TaskInfo)
	for _, t := range runningTasks {
		runningByID[fmt.Sprintf("%s/%s", t.Project, t.Name)] = t
	}

	type projectTodos struct {
		path  string
		todos []project.Todo
	}

	var projects []projectTodos

	if allProjects {
		watched, err := loadAllWatched()
		if err != nil {
			log.Fatalf("failed to read watched: %v", err)
		}
		for _, w := range watched {
			proj, err := project.Load(w.Path)
			if err != nil {
				continue
			}
			todos, _ := proj.LoadTodos()
			projects = append(projects, projectTodos{path: w.Path, todos: todos})
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

	// Filter by match pattern if specified (regex or case-insensitive substring match)
	if matchPattern != "" {
		// Check if it's a valid regex
		var regex *regexp.Regexp
		isRegex := false
		if re, err := regexp.Compile("(?i)" + matchPattern); err == nil {
			regex = re
			isRegex = true
		}

		var filtered []projectTodos
		for _, p := range projects {
			var projectFiltered []project.Todo
			for _, t := range p.todos {
				matched := false
				if isRegex {
					matched = regex.MatchString(t.Name)
				} else {
					matched = strings.Contains(strings.ToLower(t.Name), strings.ToLower(matchPattern))
				}
				if matched {
					projectFiltered = append(projectFiltered, t)
				}
			}
			if len(projectFiltered) > 0 {
				filtered = append(filtered, projectTodos{path: p.path, todos: projectFiltered})
			}
		}
		projects = filtered
	}

	// Filter by label if specified (case-insensitive match)
	if labelFilter != "" {
		var filtered []projectTodos
		for _, p := range projects {
			var projectFiltered []project.Todo
			for _, t := range p.todos {
				for _, lbl := range t.Labels {
					if strings.ToLower(lbl) == labelFilter {
						projectFiltered = append(projectFiltered, t)
						break
					}
				}
			}
			if len(projectFiltered) > 0 {
				filtered = append(filtered, projectTodos{path: p.path, todos: projectFiltered})
			}
		}
		projects = filtered
	}

	total := 0
	for _, p := range projects {
		total += len(p.todos)
	}
	if total == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("no tasks")
		}
		if matchPattern != "" || labelFilter != "" {
			os.Exit(1)
		}
		return
	}

	if jsonOutput {
		type taskJSON struct {
			Project  string   `json:"project"`
			Name     string   `json:"name"`
			Priority int      `json:"priority"`
			Schedule string   `json:"schedule"`
			Status   string   `json:"status"`
			Disabled bool     `json:"disabled"`
			Content  string   `json:"content"`
			ID       string   `json:"id,omitempty"`
			Labels   []string `json:"labels,omitempty"`
		}
		var items []taskJSON
		for _, p := range projects {
			for _, t := range p.todos {
				taskKey := fmt.Sprintf("%s/%s", p.path, t.Name)
				status := "idle"
				if t.Disabled {
					status = "disabled"
				} else if t.IsLocked {
					status = "locked"
				} else if rt, ok := runningByID[taskKey]; ok {
					if rt.Status != "" {
						status = rt.Status
					} else {
						status = "running"
					}
				}
				items = append(items, taskJSON{
					Project:  p.path,
					Name:     t.Name,
					Priority: t.Priority,
					Schedule: t.Schedule,
					Status:   status,
					Disabled: t.Disabled,
					Content:  strings.TrimSpace(t.Content),
					ID:       t.ID,
					Labels:   t.Labels,
				})
			}
		}
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Fetch budget data for persistent tasks
	var budgetMap map[string]float64
	if daemon.IsDaemonRunning() {
		budgetMap, _ = daemon.SendBudgetRequest()
	}

	for _, p := range projects {
		if allProjects && len(p.todos) > 0 {
			fmt.Printf("%s\n", p.path)
		}
		for _, t := range p.todos {
			taskKey := fmt.Sprintf("%s/%s", p.path, t.Name)
			status := "idle"
			if t.Disabled {
				status = "disabled"
			} else if t.IsLocked {
				status = "locked"
			} else if rt, ok := runningByID[taskKey]; ok {
				if rt.Status != "" {
					status = rt.Status
				} else {
					status = "running"
				}
			}
			preview := strings.TrimSpace(t.Content)
			preview = strings.Join(strings.Fields(preview), " ")
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			labelStr := ""
			if len(t.Labels) > 0 {
				labelStr = "  [" + strings.Join(t.Labels, ", ") + "]"
			}
			budgetStr := ""
			if t.IsPersistent() && t.PersistentBudget > 0 {
				budgetUsed := time.Duration(0)
				if secs, ok := budgetMap[taskKey]; ok {
					budgetUsed = time.Duration(secs * float64(time.Second))
				}
				pct := float64(budgetUsed) / float64(t.PersistentBudget) * 100
				filled := int(pct / 10)
				if filled > 10 {
					filled = 10
				}
				bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", 10-filled)
				budgetStr = fmt.Sprintf("  %s %.0f%%", bar, pct)
			}
			// Add circuit breaker status indicator
			circuitStr := ""
			if t.CircuitBreaker.Failures > 0 {
				if daemon.IsDaemonRunning() {
					circuitStorage := daemon.NewCircuitBreakerStorage(filepath.Join(config.Dir(), "circuits"))
					record, err := circuitStorage.LoadCircuit(t.ID)
					if err == nil {
						switch record.State {
						case daemon.Open:
							circuitStr = "  🔴"
						case daemon.HalfOpen:
							circuitStr = "  🟡"
						case daemon.Closed:
							if record.FailureCount > 0 {
								circuitStr = fmt.Sprintf("  ⚠️ %d", record.FailureCount)
							} else {
								circuitStr = "  ✅"
							}
						}
					}
				}
			}

			fmt.Printf("p%d  %-14s  %-10s  %-35s  %s%s%s%s\n", t.Priority, t.Schedule, status, t.Name, preview, labelStr, budgetStr, circuitStr)
		}
	}
}

func taskGetCmd(args []string) {
	jsonOutput := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		} else {
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task get <name> [--json]\n")
		os.Exit(1)
	}

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

	todo := findTodo(todos, rest[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", rest[0])
		os.Exit(1)
	}

	// Check if task is running
	runStatus := "idle"
	var runPID int
	var runElapsed string
	if daemon.IsDaemonRunning() {
		runningTasks, err := daemon.SendPsRequest()
		if err == nil {
			taskName := fmt.Sprintf("%s/%s", abs, todo.Name)
			for _, t := range runningTasks {
				if t.Name == taskName {
					runStatus = "running"
					runPID = t.PID
					runElapsed = t.Elapsed
					break
				}
			}
		}
	}

	if jsonOutput {
		type depStatusJSON struct {
			Name    string `json:"name"`
			Status  string `json:"status"` // "success", "failed", "not_run", "stale"
			Error   string `json:"error,omitempty"`
			LastRun string `json:"last_run,omitempty"`
		}
		type taskDetailJSON struct {
			File              string          `json:"file"`
			ID                string          `json:"id"`
			Name              string          `json:"name"`
			Schedule          string          `json:"schedule"`
			Priority          int             `json:"priority"`
			Disabled          bool            `json:"disabled"`
			Status            string          `json:"status"`
			PID               int             `json:"pid,omitempty"`
			Elapsed           string          `json:"elapsed,omitempty"`
			Content           string          `json:"content"`
			PreCheck          string          `json:"pre_check,omitempty"`
			OnSuccess         string          `json:"on_success,omitempty"`
			OnFailure         string          `json:"on_failure,omitempty"`
			AllowedTools      []string        `json:"allowed_tools,omitempty"`
			MaxConcurrent     int             `json:"max_concurrent,omitempty"`
			SkipPermissions   bool            `json:"skip_permissions,omitempty"`
			Runner            string          `json:"runner,omitempty"`
			RunnerChain       []string        `json:"runner_chain,omitempty"`
			RunnerOnTimeout   string          `json:"runner_on_timeout,omitempty"`
			LastRunnerUsed    string          `json:"last_runner_used,omitempty"`
			DependsOn         []string        `json:"depends_on,omitempty"`
			Dependencies      []depStatusJSON `json:"dependencies,omitempty"`
			Retry             int             `json:"retry,omitempty"`
			RetryDelay        string          `json:"retry_delay,omitempty"`
			RetryStrategy     string          `json:"retry_strategy,omitempty"`
			RetryJitter       float64         `json:"retry_jitter,omitempty"`
			RetryMaxTime      string          `json:"retry_max_time,omitempty"`
			LastAttempt       int             `json:"last_attempt,omitempty"`
			LastMaxRetries    int             `json:"last_max_retries,omitempty"`
			LastAttemptStatus string          `json:"last_attempt_status,omitempty"`
			BudgetTotal       string          `json:"budget_total,omitempty"`
			BudgetUsed        string          `json:"budget_used,omitempty"`
			BudgetRemaining   string          `json:"budget_remaining,omitempty"`
			BudgetPercent     float64         `json:"budget_percent,omitempty"`
			BudgetExhausted   bool            `json:"budget_exhausted,omitempty"`
			SLAMaxDelay       string          `json:"sla_max_delay,omitempty"`
			SLAStrict         bool            `json:"sla_strict,omitempty"`
			SLALastDelay      string          `json:"sla_last_delay,omitempty"`
			SLALastViolation  bool            `json:"sla_last_violation,omitempty"`
		}
		detail := taskDetailJSON{
			File:            todo.Path,
			ID:              todo.ID,
			Name:            todo.Name,
			Schedule:        todo.Schedule,
			Priority:        todo.Priority,
			Disabled:        todo.Disabled,
			Status:          runStatus,
			PID:             runPID,
			Elapsed:         runElapsed,
			Content:         strings.TrimSpace(todo.Content),
			PreCheck:        todo.PreCheck,
			OnSuccess:       todo.OnSuccess,
			OnFailure:       todo.OnFailure,
			AllowedTools:    todo.AllowedTools,
			MaxConcurrent:   todo.MaxConcurrent,
			SkipPermissions: todo.SkipPermissions,
			Runner:          todo.Runner,
			RunnerChain:     todo.RunnerChain,
			RunnerOnTimeout: todo.RunnerOnTimeout,
			DependsOn:       todo.DependsOn,
		}
		// Add last runner used from most recent run record
		if lastRec, recErr := project.ReadCurrentRunRecord(abs, todo.ID); recErr == nil && lastRec.RunnerCommand != "" {
			detail.LastRunnerUsed = lastRec.RunnerCommand
		}
		// Add dependency status info
		if len(todo.DependsOn) > 0 {
			for _, dep := range todo.DependsOn {
				ds := depStatusJSON{Name: dep}
				rec, err := project.ReadCurrentRunRecord(abs, dep)
				if err != nil {
					ds.Status = "not_run"
				} else if !rec.Success {
					ds.Status = "failed"
					ds.Error = rec.Error
					if !rec.Finished.IsZero() {
						ds.LastRun = rec.Finished.Format(time.RFC3339)
					}
				} else {
					ds.Status = "success"
					if !rec.Finished.IsZero() {
						ds.LastRun = rec.Finished.Format(time.RFC3339)
					}
				}
				detail.Dependencies = append(detail.Dependencies, ds)
			}
		}
		// Add retry configuration and last run attempt info
		if todo.Retry > 0 {
			detail.Retry = todo.Retry
			delayStr := todo.RetryDelay.String()
			if todo.RetryDelay <= 0 {
				delayStr = "1m0s"
			}
			detail.RetryDelay = delayStr
			if todo.RetryStrategy != "" {
				detail.RetryStrategy = todo.RetryStrategy
			}
			if todo.RetryJitter > 0 {
				detail.RetryJitter = todo.RetryJitter
			}
			if todo.RetryMaxTime > 0 {
				detail.RetryMaxTime = todo.RetryMaxTime.String()
			}
			rec, recErr := project.ReadCurrentRunRecord(abs, todo.ID)
			if recErr == nil && rec.MaxRetries > 0 {
				detail.LastAttempt = rec.Attempt
				detail.LastMaxRetries = rec.MaxRetries
				if !rec.Success {
					if rec.Attempt >= rec.MaxRetries {
						detail.LastAttemptStatus = "failed (retries exhausted)"
					} else {
						detail.LastAttemptStatus = "failed"
					}
				} else if rec.Attempt > 1 {
					detail.LastAttemptStatus = "succeeded (after retry)"
				} else {
					detail.LastAttemptStatus = "succeeded"
				}
			}
		}
		// Add budget info for persistent tasks
		if todo.IsPersistent() && todo.PersistentBudget > 0 {
			budgetUsed := time.Duration(0)
			if daemon.IsDaemonRunning() {
				budgetMap, err := daemon.SendBudgetRequest()
				if err == nil {
					taskKey := fmt.Sprintf("%s/%s", abs, todo.Name)
					if secs, ok := budgetMap[taskKey]; ok {
						budgetUsed = time.Duration(secs * float64(time.Second))
					}
				}
			}
			remaining := todo.PersistentBudget - budgetUsed
			if remaining < 0 {
				remaining = 0
			}
			pct := float64(budgetUsed) / float64(todo.PersistentBudget) * 100
			detail.BudgetTotal = todo.PersistentBudget.String()
			detail.BudgetUsed = budgetUsed.Round(time.Second).String()
			detail.BudgetRemaining = remaining.Round(time.Second).String()
			detail.BudgetPercent = pct
			detail.BudgetExhausted = budgetUsed >= todo.PersistentBudget
		}
		// Add SLA info
		if todo.SLA.MaxDelay > 0 {
			detail.SLAMaxDelay = todo.SLA.MaxDelay.String()
			detail.SLAStrict = todo.SLA.Strict
			rec, recErr := project.ReadCurrentRunRecord(abs, todo.ID)
			if recErr == nil && rec.SLAMaxDelay > 0 {
				detail.SLALastDelay = rec.DispatchDelay.String()
				detail.SLALastViolation = rec.SLAViolation
			}
		}
		data, err := json.MarshalIndent(detail, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	fmt.Printf("File:     %s\n", todo.Path)
	fmt.Printf("ID:       %s\n", todo.ID)
	fmt.Printf("Schedule: %s\n", todo.Schedule)
	fmt.Printf("Priority: %d\n", todo.Priority)
	if todo.Disabled {
		fmt.Printf("Disabled: true\n")
	}
	fmt.Printf("Disabled: %t\n", todo.Disabled)
	if todo.ID != "" {
		sessionPath := project.SessionPath(abs, todo.ID)
		if _, err := os.Stat(sessionPath); err == nil {
			fmt.Printf("Session:  %s\n", sessionPath)
		}
	}
	if runPID > 0 {
		fmt.Printf("Status:   running (PID %d, elapsed %s)\n", runPID, runElapsed)
	} else {
		fmt.Printf("Status:   %s\n", runStatus)
	}
	// Show dependency status
	if len(todo.DependsOn) > 0 {
		fmt.Printf("Deps:     %s\n", strings.Join(todo.DependsOn, ", "))
		for _, dep := range todo.DependsOn {
			rec, err := project.ReadCurrentRunRecord(abs, dep)
			if err != nil {
				fmt.Printf("  %-30s not_run\n", dep)
			} else if !rec.Success {
				lastRun := ""
				if !rec.Finished.IsZero() {
					lastRun = fmt.Sprintf(" (last run: %s)", rec.Finished.Format("15:04"))
				}
				errMsg := ""
				if rec.Error != "" {
					errMsg = fmt.Sprintf(" — %s", rec.Error)
				}
				fmt.Printf("  %-30s failed%s%s\n", dep, lastRun, errMsg)
			} else {
				lastRun := ""
				if !rec.Finished.IsZero() {
					lastRun = fmt.Sprintf(" (last run: %s)", rec.Finished.Format("15:04"))
				}
				fmt.Printf("  %-30s success%s\n", dep, lastRun)
			}
		}
	}
	// Show runner chain configuration
	if len(todo.RunnerChain) > 0 {
		fmt.Printf("Chain:    %d runners\n", len(todo.RunnerChain))
		for i, cmd := range todo.RunnerChain {
			fmt.Printf("  [%d] %s\n", i, cmd)
		}
		if todo.RunnerOnTimeout != "" {
			fmt.Printf("  on_timeout: %s\n", todo.RunnerOnTimeout)
		}
		// Show which runner was last used
		rec, recErr := project.ReadCurrentRunRecord(abs, todo.ID)
		if recErr == nil && rec.RunnerCommand != "" {
			fmt.Printf("          last used: %s\n", rec.RunnerCommand)
		}
	} else if todo.RunnerOnTimeout != "" {
		fmt.Printf("Timeout:  fallback runner: %s\n", todo.RunnerOnTimeout)
	}
	// Show retry configuration and last run attempt info
	if todo.Retry > 0 {
		delayStr := todo.RetryDelay.String()
		if todo.RetryDelay <= 0 {
			delayStr = "1m0s (default)"
		}
		fmt.Printf("Retry:    %d retries, delay %s\n", todo.Retry, delayStr)
		// Show last run attempt info
		rec, err := project.ReadCurrentRunRecord(abs, todo.ID)
		if err == nil && rec.MaxRetries > 0 {
			attemptStatus := "succeeded"
			if !rec.Success {
				if rec.Attempt >= rec.MaxRetries {
					attemptStatus = "failed (retries exhausted)"
				} else {
					attemptStatus = "failed"
				}
			} else if rec.Attempt > 1 {
				attemptStatus = "succeeded (after retry)"
			}
			fmt.Printf("          last run: attempt %d/%d — %s\n", rec.Attempt, rec.MaxRetries, attemptStatus)
		}
	}
	// Show budget info for persistent tasks with a budget
	if todo.IsPersistent() && todo.PersistentBudget > 0 {
		budgetUsed := time.Duration(0)
		if daemon.IsDaemonRunning() {
			budgetMap, err := daemon.SendBudgetRequest()
			if err == nil {
				taskKey := fmt.Sprintf("%s/%s", abs, todo.Name)
				if secs, ok := budgetMap[taskKey]; ok {
					budgetUsed = time.Duration(secs * float64(time.Second))
				}
			}
		}
		remaining := todo.PersistentBudget - budgetUsed
		if remaining < 0 {
			remaining = 0
		}
		pct := float64(budgetUsed) / float64(todo.PersistentBudget) * 100
		fmt.Printf("Budget:   %v / %v used (%.0f%%)\n", budgetUsed.Round(time.Second), todo.PersistentBudget, pct)
		if remaining > 0 {
			fmt.Printf("          %v remaining\n", remaining.Round(time.Second))
		} else {
			fmt.Printf("          EXHAUSTED\n")
		}
	}
	// Show SLA configuration and last run status
	if todo.SLA.MaxDelay > 0 {
		strictStr := ""
		if todo.SLA.Strict {
			strictStr = " (strict)"
		}
		fmt.Printf("SLA:      %v max delay%s\n", todo.SLA.MaxDelay, strictStr)
		rec, err := project.ReadCurrentRunRecord(abs, todo.ID)
		if err == nil && rec.SLAMaxDelay > 0 {
			if rec.SLAViolation {
				fmt.Printf("          last run: %v late — SLA VIOLATION\n", rec.DispatchDelay.Round(time.Second))
			} else {
				fmt.Printf("          last run: on time (%v delay)\n", rec.DispatchDelay.Round(time.Second))
			}
		}
	}

	// Show circuit breaker status
	if todo.CircuitBreaker.Failures > 0 {
		fmt.Printf("Circuit:  %d failures → OPEN, timeout %v\n", todo.CircuitBreaker.Failures, todo.CircuitBreaker.Timeout)
		if todo.CircuitBreaker.HalfOpenMax > 0 {
			fmt.Printf("          half-open test requests: %d\n", todo.CircuitBreaker.HalfOpenMax)
		}

		// Load circuit breaker state
		if daemon.IsDaemonRunning() {
			circuitStorage := daemon.NewCircuitBreakerStorage(filepath.Join(config.Dir(), "circuits"))
			record, err := circuitStorage.LoadCircuit(todo.ID)
			if err == nil {
				fmt.Printf("          state: %s", record.State.String())
				if record.State == daemon.Closed {
					if record.FailureCount > 0 {
						fmt.Printf(" (%d consecutive failures)", record.FailureCount)
					}
				} else if record.State == daemon.Open {
					if record.OpenedAt != nil {
						fmt.Printf(" (since %s)", record.OpenedAt.Format("15:04:05"))
					}
					if record.NextRetryAt != nil {
						fmt.Printf(", retry at %s", record.NextRetryAt.Format("15:04:05"))
					}
				} else if record.State == daemon.HalfOpen {
					fmt.Printf(" (%d test requests)", record.HalfOpenCount)
				}
				fmt.Printf("\n")

				if record.LastFailureAt != nil {
					fmt.Printf("          last failure: %s\n", record.LastFailureAt.Format("2006-01-02 15:04:05"))
				}
			}
		}
	}

	fmt.Printf("\n%s", todo.Content)
}
