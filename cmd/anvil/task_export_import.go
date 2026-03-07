package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

func taskExportCmd(args []string) {
	exportAll := false
	outputFile := ""

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--all" || arg == "-a" {
			exportAll = true
		} else if arg == "-o" || arg == "--output" {
			if i+1 >= len(args) {
				log.Fatal("missing value for -o/--output")
			}
			i++
			outputFile = args[i]
		} else if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: anvil task export [options] [task-names...]")
			fmt.Println("")
			fmt.Println("Export tasks to a JSON file for sharing or backup.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --all, -a      Export all tasks from current project")
			fmt.Println("  -o, --output   Output file (default: stdout)")
			fmt.Println("  -h             Show this help")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  anvil task export task1.md task2.md -o backup.json")
			fmt.Println("  anvil task export --all -o all-tasks.json")
			return
		} else if strings.HasPrefix(arg, "-") {
			log.Fatalf("unknown flag: %s", arg)
		} else {
			break
		}
		i++
	}

	taskNames := args[i:]

	if !exportAll && len(taskNames) == 0 {
		log.Fatal("specify task names or use --all")
	}

	// Load current project
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

	// Filter tasks
	var toExport []project.Todo
	if exportAll {
		toExport = todos
	} else {
		for _, name := range taskNames {
			found := false
			for _, t := range todos {
				if t.Name == name {
					toExport = append(toExport, t)
					found = true
					break
				}
			}
			if !found {
				log.Fatalf("task not found: %s", name)
			}
		}
	}

	// Build export data
	type ExportTask struct {
		Name         string `json:"name"`
		Content      string `json:"content"`
		ProjectPath  string `json:"project_path"`
		Schedule     string `json:"schedule,omitempty"`
		Priority     int    `json:"priority"`
		Timeout      int    `json:"timeout,omitempty"`
		Retry         int     `json:"retry,omitempty"`
		RetryDelay    string  `json:"retry_delay,omitempty"`
		RetryStrategy string  `json:"retry_strategy,omitempty"`
		RetryJitter   float64 `json:"retry_jitter,omitempty"`
		RetryMaxTime  string  `json:"retry_max_time,omitempty"`
		Runner        string  `json:"runner,omitempty"`
		Webhook       string  `json:"webhook,omitempty"`
		PreCheck      string  `json:"pre_check,omitempty"`
		OnSuccess     string  `json:"on_success,omitempty"`
		OnFailure     string  `json:"on_failure,omitempty"`
		Disabled      bool    `json:"disabled"`
		MaxLogSize    int64   `json:"max_log_size,omitempty"`
		SkipPerms     bool    `json:"skip_permissions"`
		AllowedTools  string  `json:"allowed_tools,omitempty"`
	}

	exportTasks := make([]ExportTask, len(toExport))
	for i, t := range toExport {
		var allowedToolsStr string
		if len(t.AllowedTools) > 0 {
			allowedToolsStr = strings.Join(t.AllowedTools, ",")
		}
		exportTasks[i] = ExportTask{
			Name:         t.Name,
			Content:      t.Content,
			ProjectPath:  abs,
			Schedule:     t.Schedule,
			Priority:     t.Priority,
			Timeout:      int(t.Timeout.Seconds()),
			Retry:         t.Retry,
			RetryDelay:    t.RetryDelay.String(),
			RetryStrategy: t.RetryStrategy,
			RetryJitter:   t.RetryJitter,
			RetryMaxTime:  t.RetryMaxTime.String(),
			Runner:        t.Runner,
			Webhook:      t.Webhook,
			PreCheck:     t.PreCheck,
			OnSuccess:    t.OnSuccess,
			OnFailure:    t.OnFailure,
			Disabled:     t.Disabled,
			MaxLogSize:   t.MaxLogSize,
			SkipPerms:    t.SkipPermissions,
			AllowedTools: allowedToolsStr,
		}
	}

	type ExportData struct {
		Version    string       `json:"version"`
		ExportedAt time.Time    `json:"exported_at"`
		Tasks      []ExportTask `json:"tasks"`
	}

	data := ExportData{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Tasks:      exportTasks,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal export: %v", err)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			log.Fatalf("failed to write export file: %v", err)
		}
		fmt.Printf("Exported %d tasks to %s\n", len(exportTasks), outputFile)
	} else {
		fmt.Println(string(jsonData))
	}
}

// taskImportCmd imports tasks from a JSON file.
func taskImportCmd(args []string) {
	inputFile := ""
	basePath := ""
	dryRun := false
	force := false

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--base-path" {
			if i+1 >= len(args) {
				log.Fatal("missing value for --base-path")
			}
			i++
			basePath = args[i]
		} else if arg == "--dry-run" || arg == "-n" {
			dryRun = true
		} else if arg == "--force" || arg == "-f" {
			force = true
		} else if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: anvil task import <file> [options]")
			fmt.Println("")
			fmt.Println("Import tasks from a JSON export file.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --base-path    Remap project paths during import")
			fmt.Println("  --dry-run, -n  Preview without creating tasks")
			fmt.Println("  --force, -f   Overwrite existing tasks")
			fmt.Println("  -h            Show this help")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  anvil task import backup.json")
			fmt.Println("  anvil task import backup.json --dry-run")
			fmt.Println("  anvil task import backup.json --base-path /new/project/path")
			return
		} else if strings.HasPrefix(arg, "-") {
			log.Fatalf("unknown flag: %s", arg)
		} else {
			if inputFile != "" {
				log.Fatal("multiple input files specified")
			}
			inputFile = arg
		}
		i++
	}

	if inputFile == "" {
		log.Fatal("missing input file")
	}

	data, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("failed to read import file: %v", err)
	}

	type ExportTask struct {
		Name         string `json:"name"`
		Content      string `json:"content"`
		ProjectPath  string `json:"project_path"`
		Schedule     string `json:"schedule,omitempty"`
		Priority     int    `json:"priority"`
		Timeout      int    `json:"timeout,omitempty"`
		Retry         int     `json:"retry,omitempty"`
		RetryDelay    string  `json:"retry_delay,omitempty"`
		RetryStrategy string  `json:"retry_strategy,omitempty"`
		RetryJitter   float64 `json:"retry_jitter,omitempty"`
		RetryMaxTime  string  `json:"retry_max_time,omitempty"`
		Runner        string  `json:"runner,omitempty"`
		Webhook       string  `json:"webhook,omitempty"`
		PreCheck      string  `json:"pre_check,omitempty"`
		OnSuccess     string  `json:"on_success,omitempty"`
		OnFailure     string  `json:"on_failure,omitempty"`
		Disabled      bool    `json:"disabled"`
		MaxLogSize    int64   `json:"max_log_size,omitempty"`
		SkipPerms     bool    `json:"skip_permissions"`
		AllowedTools  string  `json:"allowed_tools,omitempty"`
	}

	type ExportData struct {
		Version    string       `json:"version"`
		ExportedAt time.Time    `json:"exported_at"`
		Tasks      []ExportTask `json:"tasks"`
	}

	var export ExportData
	if err := json.Unmarshal(data, &export); err != nil {
		log.Fatalf("failed to parse import file: %v", err)
	}

	if len(export.Tasks) == 0 {
		log.Fatal("no tasks found in import file")
	}

	// Determine target project path
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}
	targetPath := abs
	if basePath != "" {
		targetPath = basePath
	}

	proj, err := project.Load(targetPath)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	// Check for existing tasks
	existingTodos, _ := proj.LoadTodos()
	existingNames := make(map[string]bool)
	for _, t := range existingTodos {
		existingNames[t.Name] = true
	}

	importCount := 0
	skipCount := 0

	for _, et := range export.Tasks {
		// Check if task exists
		if existingNames[et.Name] && !force {
			fmt.Printf("Skipping %s (already exists, use --force to overwrite)\n", et.Name)
			skipCount++
			continue
		}

		if dryRun {
			fmt.Printf("Would import: %s\n", et.Name)
			importCount++
			continue
		}

		// Build frontmatter and content
		var frontmatter []string
		if et.Schedule != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("schedule: %q", et.Schedule))
		}
		if et.Priority != 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("priority: %d", et.Priority))
		}
		if et.Timeout > 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("timeout: %ds", et.Timeout))
		}
		if et.Retry > 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("retry: %d", et.Retry))
		}
		if et.RetryDelay != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("retry_delay: %q", et.RetryDelay))
		}
		if et.RetryStrategy != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("retry_strategy: %q", et.RetryStrategy))
		}
		if et.RetryJitter != 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("retry_jitter: %g", et.RetryJitter))
		}
		if et.RetryMaxTime != "" && et.RetryMaxTime != "0s" {
			frontmatter = append(frontmatter, fmt.Sprintf("retry_max_time: %q", et.RetryMaxTime))
		}
		if et.Runner != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("runner: %q", et.Runner))
		}
		if et.Webhook != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("webhook: %q", et.Webhook))
		}
		if et.PreCheck != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("pre_check: %q", et.PreCheck))
		}
		if et.OnSuccess != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("on_success: %q", et.OnSuccess))
		}
		if et.OnFailure != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("on_failure: %q", et.OnFailure))
		}
		if et.Disabled {
			frontmatter = append(frontmatter, "disabled: true")
		}
		if et.MaxLogSize > 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("max_log_size: %d", et.MaxLogSize))
		}
		if et.SkipPerms {
			frontmatter = append(frontmatter, "skip_permissions: true")
		}
		if et.AllowedTools != "" {
			tools := strings.Split(et.AllowedTools, ",")
			for i := range tools {
				tools[i] = strings.TrimSpace(tools[i])
			}
			frontmatter = append(frontmatter, fmt.Sprintf("allowed_tools: [%s]", strings.Join(tools, ", ")))
		}

		// Build full content with frontmatter
		var content string
		if len(frontmatter) > 0 {
			content = "---\n" + strings.Join(frontmatter, "\n") + "\n---\n\n" + et.Content
		} else {
			content = et.Content
		}

		// Write task file
		todoDir := filepath.Join(proj.Path, ".anvil", "todos")
		if err := os.MkdirAll(todoDir, 0755); err != nil {
			log.Fatalf("failed to create todos directory: %v", err)
		}

		taskFile := filepath.Join(todoDir, et.Name)
		if err := os.WriteFile(taskFile, []byte(content), 0644); err != nil {
			log.Fatalf("failed to write task file: %v", err)
		}

		fmt.Printf("Imported: %s\n", et.Name)
		importCount++
	}

	if dryRun {
		fmt.Printf("\nDry run: would import %d task(s), skip %d\n", importCount, skipCount)
	} else {
		fmt.Printf("\nImported %d task(s) from %s\n", importCount, inputFile)
	}
}

// Conflict represents a scheduling conflict between two tasks
type Conflict struct {
	Task1     string
	Task2     string
	Schedule1 string
	Schedule2 string
	Reason    string
}

// detectConflicts analyzes tasks for scheduling conflicts
func detectConflicts(todos []project.Todo) []Conflict {
	var conflicts []Conflict

	// Build a map of task name to schedule
	taskSchedules := make(map[string]string)
	for _, t := range todos {
		if t.Schedule != "" {
			taskSchedules[t.Name] = t.Schedule
		}
	}

	// Check for frequency-based conflicts
	for i := range todos {
		for j := i + 1; j < len(todos); j++ {
			t1, t2 := todos[i], todos[j]
			if t1.Schedule == "" || t2.Schedule == "" {
				continue
			}

			// Both have schedules - check for conflicts
			// High frequency: both run every minute
			if t1.Schedule == "*/1 * * * *" && t2.Schedule == "*/1 * * * *" {
				conflicts = append(conflicts, Conflict{
					Task1: t1.Name, Task2: t2.Name,
					Schedule1: t1.Schedule, Schedule2: t2.Schedule,
					Reason: "Both tasks run every minute - may compete for resources",
				})
				continue
			}

			// Check for same minute patterns that could overlap
			// e.g., */5 and */10 at minute 0
			if strings.Contains(t1.Schedule, "*/5") && strings.Contains(t2.Schedule, "*/") {
				conflicts = append(conflicts, Conflict{
					Task1: t1.Name, Task2: t2.Name,
					Schedule1: t1.Schedule, Schedule2: t2.Schedule,
					Reason: "Tasks may run at the same time - check schedules",
				})
			}
		}
	}

	return conflicts
}
