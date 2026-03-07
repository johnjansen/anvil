package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
)

func taskDryRunCmd(args []string) {
	validateOnly := false
	allTasks := false
	jsonOutput := false
	var rest []string
	for _, a := range args {
		switch a {
		case "--validate-only":
			validateOnly = true
		case "--all", "-a":
			allTasks = true
		case "--json":
			jsonOutput = true
		default:
			rest = append(rest, a)
		}
	}

	if !allTasks && len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task dry-run <name> [--validate-only] [--json]\n")
		fmt.Fprintf(os.Stderr, "       anvil task dry-run --all [--validate-only] [--json]\n")
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

	globalCfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load global config: %v", err)
	}

	var targets []project.Todo
	if allTasks {
		targets = todos
	} else {
		todo := findTodo(todos, rest[0])
		if todo == nil {
			fmt.Fprintf(os.Stderr, "task not found: %s\n", rest[0])
			os.Exit(1)
		}
		targets = []project.Todo{*todo}
	}

	if jsonOutput {
		dryRunJSON(targets, abs, globalCfg, validateOnly)
		return
	}

	for i, todo := range targets {
		if i > 0 {
			fmt.Println(strings.Repeat("\u2500", 60))
		}
		dryRunPrint(&todo, abs, globalCfg, validateOnly)
	}
}

func dryRunJSON(todos []project.Todo, projectPath string, cfg *config.Config, validateOnly bool) {
	type validationError struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	type dryRunResult struct {
		Name            string            `json:"name"`
		File            string            `json:"file"`
		ID              string            `json:"id"`
		Schedule        string            `json:"schedule"`
		NextRun         string            `json:"next_run,omitempty"`
		Priority        int               `json:"priority"`
		Disabled        bool              `json:"disabled"`
		Runner          string            `json:"runner,omitempty"`
		RunnerChain     []string          `json:"runner_chain,omitempty"`
		RunnerOnTimeout string            `json:"runner_on_timeout,omitempty"`
		Timeout         string            `json:"timeout,omitempty"`
		PreCheck        string            `json:"pre_check,omitempty"`
		PreCheckResult  string            `json:"pre_check_result,omitempty"`
		OnSuccess       string            `json:"on_success,omitempty"`
		OnFailure       string            `json:"on_failure,omitempty"`
		AllowedTools    []string          `json:"allowed_tools,omitempty"`
		SkipPermissions bool              `json:"skip_permissions,omitempty"`
		MaxConcurrent   int               `json:"max_concurrent,omitempty"`
		Retry           int               `json:"retry,omitempty"`
		RetryDelay      string            `json:"retry_delay,omitempty"`
		RetryStrategy   string            `json:"retry_strategy,omitempty"`
		RetryJitter     float64           `json:"retry_jitter,omitempty"`
		RetryMaxTime    string            `json:"retry_max_time,omitempty"`
		DependsOn       []string          `json:"depends_on,omitempty"`
		DepsStatus      string            `json:"deps_status,omitempty"`
		Labels          []string          `json:"labels,omitempty"`
		Env             map[string]string `json:"env,omitempty"`
		Checkpoint      bool              `json:"checkpoint,omitempty"`
		Webhook         string            `json:"webhook,omitempty"`
		ContentPreview  string            `json:"content_preview,omitempty"`
		ContentLines    int               `json:"content_lines"`
		Errors          []validationError `json:"errors,omitempty"`
		Valid           bool              `json:"valid"`
	}

	var results []dryRunResult
	for _, todo := range todos {
		r := dryRunResult{
			Name:            todo.Name,
			File:            todo.Path,
			ID:              todo.ID,
			Schedule:        todo.Schedule,
			Priority:        todo.Priority,
			Disabled:        todo.Disabled,
			Runner:          todo.Runner,
			RunnerChain:     todo.RunnerChain,
			RunnerOnTimeout: todo.RunnerOnTimeout,
			PreCheck:        todo.PreCheck,
			OnSuccess:       todo.OnSuccess,
			OnFailure:       todo.OnFailure,
			AllowedTools:    todo.AllowedTools,
			SkipPermissions: todo.SkipPermissions,
			MaxConcurrent:   todo.MaxConcurrent,
			Retry:           todo.Retry,
			DependsOn:       todo.DependsOn,
			Labels:          todo.Labels,
			Checkpoint:      todo.Checkpoint,
			Webhook:         todo.Webhook,
			Valid:           true,
		}

		if todo.Schedule != "" && todo.Schedule != "persistent" {
			p, err := cron.Parse(todo.Schedule)
			if err != nil {
				r.Errors = append(r.Errors, validationError{Field: "schedule", Message: err.Error()})
				r.Valid = false
			} else {
				if next, err := p.Next(time.Now()); err == nil {
					r.NextRun = next.Format(time.RFC3339)
				}
			}
		}

		if todo.Timeout > 0 {
			r.Timeout = todo.Timeout.String()
		} else if cfg.Timeout > 0 {
			r.Timeout = cfg.Timeout.String() + " (global default)"
		}

		if todo.Retry > 0 {
			if todo.RetryDelay > 0 {
				r.RetryDelay = todo.RetryDelay.String()
			} else {
				r.RetryDelay = "1m0s (default)"
			}
			if todo.RetryStrategy != "" {
				r.RetryStrategy = todo.RetryStrategy
			} else {
				r.RetryStrategy = "exponential (default)"
			}
			if todo.RetryJitter > 0 {
				r.RetryJitter = todo.RetryJitter
			}
			if todo.RetryMaxTime > 0 {
				r.RetryMaxTime = todo.RetryMaxTime.String()
			}
		}

		mergedEnv := dryRunMergeEnv(cfg.Env, todo.Env)
		if len(mergedEnv) > 0 {
			r.Env = dryRunResolveEnv(mergedEnv)
		}

		content := strings.TrimSpace(todo.Content)
		lines := strings.Split(content, "\n")
		r.ContentLines = len(lines)
		if !validateOnly {
			if len(lines) > 20 {
				r.ContentPreview = strings.Join(lines[:20], "\n") + "\n..."
			} else {
				r.ContentPreview = content
			}
		}

		if todo.PreCheck != "" && !validateOnly {
			cmd := exec.Command("sh", "-c", todo.PreCheck)
			cmd.Dir = projectPath
			if err := cmd.Run(); err != nil {
				r.PreCheckResult = "would_skip"
			} else {
				r.PreCheckResult = "would_proceed"
			}
		}

		if len(todo.DependsOn) > 0 {
			allMet := true
			for _, dep := range todo.DependsOn {
				rec, err := project.ReadCurrentRunRecord(projectPath, dep)
				if err != nil || !rec.Success {
					allMet = false
				}
			}
			if allMet {
				r.DepsStatus = "all_met"
			} else {
				r.DepsStatus = "not_met"
			}
		}

		if todo.ID == "" {
			r.Errors = append(r.Errors, validationError{Field: "id", Message: "missing task ID"})
			r.Valid = false
		}
		if todo.Schedule == "" {
			r.Errors = append(r.Errors, validationError{Field: "schedule", Message: "no schedule set (one-shot task)"})
		}
		if len(r.Errors) > 0 {
			r.Valid = false
		}

		results = append(results, r)
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal JSON: %v", err)
	}
	fmt.Println(string(data))
}

func dryRunPrint(todo *project.Todo, projectPath string, cfg *config.Config, validateOnly bool) {
	var errors []string

	fmt.Printf("Task:       %s\n", todo.Name)
	fmt.Printf("File:       %s\n", todo.Path)
	fmt.Printf("ID:         %s\n", todo.ID)
	if todo.ID == "" {
		errors = append(errors, "missing task ID")
	}
	fmt.Printf("Priority:   %d\n", todo.Priority)
	fmt.Printf("Disabled:   %t\n", todo.Disabled)
	if len(todo.Labels) > 0 {
		fmt.Printf("Labels:     %s\n", strings.Join(todo.Labels, ", "))
	}

	fmt.Println()
	fmt.Println("\u2500\u2500 Schedule \u2500\u2500")
	if todo.Schedule == "" {
		fmt.Println("Schedule:   (none \u2014 one-shot task)")
	} else if todo.Schedule == "persistent" {
		fmt.Println("Schedule:   persistent")
		if todo.PersistentCooldown > 0 {
			fmt.Printf("Cooldown:   %s\n", todo.PersistentCooldown)
		}
		if todo.PersistentMaxRuntime > 0 {
			fmt.Printf("Max Runtime: %s\n", todo.PersistentMaxRuntime)
		}
		if todo.PersistentBudget > 0 {
			fmt.Printf("Budget:     %s\n", todo.PersistentBudget)
		}
	} else {
		fmt.Printf("Schedule:   %s\n", todo.Schedule)
		p, err := cron.Parse(todo.Schedule)
		if err != nil {
			fmt.Printf("            ERROR: invalid cron expression: %v\n", err)
			errors = append(errors, fmt.Sprintf("invalid schedule: %v", err))
		} else {
			if next, err := p.Next(time.Now()); err == nil {
				until := time.Until(next).Round(time.Minute)
				fmt.Printf("Next Run:   %s (%s from now)\n", next.Format("Mon Jan 2 15:04:05"), until)
			}
		}
	}
	if todo.Window.Start != "" || todo.Window.End != "" {
		fmt.Printf("Window:     %s \u2013 %s", todo.Window.Start, todo.Window.End)
		if todo.Window.Days != "" {
			fmt.Printf(" (days: %s)", todo.Window.Days)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("\u2500\u2500 Runner \u2500\u2500")
	if todo.Runner != "" {
		fmt.Printf("Runner:     %s (task override)\n", todo.Runner)
	} else if len(todo.RunnerChain) > 0 {
		fmt.Println("Runner Chain (task override):")
		for i, r := range todo.RunnerChain {
			fmt.Printf("  %d. %s\n", i+1, r)
		}
	} else if len(cfg.Runners) > 0 {
		fmt.Println("Runner Chain (global config):")
		for i, r := range cfg.Runners {
			fmt.Printf("  %d. %s\n", i+1, r)
		}
	} else if cfg.Runner != "" {
		fmt.Printf("Runner:     %s (global config)\n", cfg.Runner)
	} else {
		fmt.Println("Runner:     echo (default)")
	}
	if todo.RunnerOnTimeout != "" {
		fmt.Printf("On Timeout: %s\n", todo.RunnerOnTimeout)
	}
	if todo.Timeout > 0 {
		fmt.Printf("Timeout:    %s\n", todo.Timeout)
	} else if cfg.Timeout > 0 {
		fmt.Printf("Timeout:    %s (global default)\n", cfg.Timeout)
	}
	if todo.SkipPermissions {
		fmt.Println("Permissions: skip (--dangerously-skip-permissions)")
	}
	if len(todo.AllowedTools) > 0 {
		fmt.Printf("Tools:      %s\n", strings.Join(todo.AllowedTools, ", "))
	}
	fmt.Printf("Concurrent: %d\n", max(todo.MaxConcurrent, 1))

	if todo.PreCheck != "" || len(todo.DependsOn) > 0 {
		fmt.Println()
		fmt.Println("\u2500\u2500 Pre-execution Checks \u2500\u2500")
	}
	if todo.PreCheck != "" {
		fmt.Printf("Pre-check:  %s\n", todo.PreCheck)
		if !validateOnly {
			cmd := exec.Command("sh", "-c", todo.PreCheck)
			cmd.Dir = projectPath
			if err := cmd.Run(); err != nil {
				fmt.Println("  Result:   FAILED \u2014 task would be SKIPPED")
			} else {
				fmt.Println("  Result:   passed \u2014 task would proceed")
			}
		}
	}
	if len(todo.DependsOn) > 0 {
		fmt.Printf("Depends On: %s\n", strings.Join(todo.DependsOn, ", "))
		for _, dep := range todo.DependsOn {
			rec, err := project.ReadCurrentRunRecord(projectPath, dep)
			if err != nil {
				fmt.Printf("  %-28s not_run\n", dep)
			} else if !rec.Success {
				fmt.Printf("  %-28s failed\n", dep)
			} else {
				age := time.Since(rec.Finished).Round(time.Second)
				fmt.Printf("  %-28s success (%s ago)\n", dep, age)
			}
		}
	}

	if todo.Retry > 0 {
		fmt.Println()
		fmt.Println("\u2500\u2500 Retry \u2500\u2500")
		delayStr := todo.RetryDelay.String()
		if todo.RetryDelay <= 0 {
			delayStr = "1m0s (default)"
		}
		strategyStr := todo.RetryStrategy
		if strategyStr == "" {
			strategyStr = "exponential (default)"
		}
		fmt.Printf("Retries:    %d, delay %s, strategy %s\n", todo.Retry, delayStr, strategyStr)
		if todo.RetryJitter > 0 {
			fmt.Printf("Jitter:     %.0f%%\n", todo.RetryJitter*100)
		}
		if todo.RetryMaxTime > 0 {
			fmt.Printf("Max Time:   %s\n", todo.RetryMaxTime)
		}
	}

	if todo.OnSuccess != "" || todo.OnFailure != "" || todo.Webhook != "" {
		fmt.Println()
		fmt.Println("\u2500\u2500 Hooks \u2500\u2500")
		if todo.OnSuccess != "" {
			fmt.Printf("On Success: %s\n", todo.OnSuccess)
		}
		if todo.OnFailure != "" {
			fmt.Printf("On Failure: %s\n", todo.OnFailure)
		}
		if todo.Webhook != "" {
			fmt.Printf("Webhook:    %s\n", todo.Webhook)
		}
	}

	mergedEnv := dryRunMergeEnv(cfg.Env, todo.Env)
	if len(mergedEnv) > 0 {
		fmt.Println()
		fmt.Println("\u2500\u2500 Environment \u2500\u2500")
		resolved := dryRunResolveEnv(mergedEnv)
		for k, v := range resolved {
			display := v
			kLower := strings.ToLower(k)
			if len(v) > 8 && (strings.Contains(kLower, "key") ||
				strings.Contains(kLower, "secret") ||
				strings.Contains(kLower, "token") ||
				strings.Contains(kLower, "password")) {
				display = v[:4] + "****"
			}
			source := "literal"
			if raw, ok := todo.Env[k]; ok && strings.HasPrefix(raw, "env:") {
				source = "env:" + raw[4:]
			} else if raw, ok := cfg.Env[k]; ok && strings.HasPrefix(raw, "env:") {
				source = "env:" + raw[4:]
			}
			if _, inTask := todo.Env[k]; inTask {
				fmt.Printf("  %s=%s (%s, task)\n", k, display, source)
			} else {
				fmt.Printf("  %s=%s (%s, global)\n", k, display, source)
			}
		}
	}

	if todo.Checkpoint {
		fmt.Println()
		fmt.Println("\u2500\u2500 State \u2500\u2500")
		fmt.Println("Checkpoint: enabled")
	}

	if !validateOnly {
		fmt.Println()
		fmt.Println("\u2500\u2500 Content Preview \u2500\u2500")
		content := strings.TrimSpace(todo.Content)
		lines := strings.Split(content, "\n")
		if len(lines) > 30 {
			for _, line := range lines[:30] {
				fmt.Printf("  %s\n", line)
			}
			fmt.Printf("  ... (%d more lines)\n", len(lines)-30)
		} else {
			for _, line := range lines {
				fmt.Printf("  %s\n", line)
			}
		}
		fmt.Printf("(%d lines total)\n", len(lines))
	}

	fmt.Println()
	if len(errors) > 0 {
		fmt.Println("\u2500\u2500 Validation Errors \u2500\u2500")
		for _, e := range errors {
			fmt.Printf("  \u2718 %s\n", e)
		}
		fmt.Println()
		fmt.Println("Result: INVALID \u2014 task has configuration errors")
	} else {
		fmt.Println("Result: VALID \u2014 task is ready to execute")
	}
}

func dryRunMergeEnv(global, task map[string]string) map[string]string {
	if len(global) == 0 && len(task) == 0 {
		return nil
	}
	merged := make(map[string]string, len(global)+len(task))
	for k, v := range global {
		merged[k] = v
	}
	for k, v := range task {
		merged[k] = v
	}
	return merged
}

func dryRunResolveEnv(vars map[string]string) map[string]string {
	if len(vars) == 0 {
		return nil
	}
	resolved := make(map[string]string, len(vars))
	for k, v := range vars {
		if strings.HasPrefix(v, "env:") {
			resolved[k] = os.Getenv(v[4:])
		} else {
			resolved[k] = v
		}
	}
	return resolved
}
