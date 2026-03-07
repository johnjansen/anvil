package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"gopkg.in/yaml.v3"
)

func taskQueueCmd(args []string) {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		}
	}

	if !daemon.IsDaemonRunning() {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("daemon not running")
		}
		return
	}

	tasks, err := daemon.SendQueueRequest()
	if err != nil {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Fprintf(os.Stderr, "failed to get queue status: %v\n", err)
		}
		os.Exit(1)
	}

	if len(tasks) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("no tasks in queue")
		}
		return
	}

	if jsonOutput {
		data, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	fmt.Printf("%-40s %-10s %-10s %s\n", "TASK", "PRIORITY", "STATUS", "SKIP REASON")
	fmt.Printf("%s\n", strings.Repeat("-", 90))

	for _, t := range tasks {
		skipReason := t.SkipReason
		if skipReason == "" {
			skipReason = "-"
		}
		fmt.Printf("%-40s %-10d %-10s %s\n",
			truncate(t.Name, 40),
			t.Priority,
			t.Status,
			skipReason)
	}
}

func taskTimeoutCmd(args []string) {
	allTasks := false
	for _, a := range args {
		if a == "--all" || a == "-a" {
			allTasks = true
		}
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	tasks, err := daemon.SendTimeoutRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get timeout status: %v\n", err)
		os.Exit(1)
	}

	if len(tasks) == 0 {
		fmt.Println("no running tasks")
		return
	}

	// Filter by task name if provided and not --all
	targetName := ""
	if !allTasks && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		targetName = args[0]
	}

	fmt.Printf("%-40s %-15s %-15s %-10s %s\n", "TASK", "ELAPSED", "TIMEOUT", "REMAINING", "PROGRESS")
	fmt.Printf("%s\n", strings.Repeat("-", 100))

	for _, t := range tasks {
		// Filter by target name if specified
		if targetName != "" && !strings.Contains(t.Name, targetName) {
			continue
		}

		elapsed := t.Elapsed
		timeout := t.Timeout
		remaining := t.TimeRemaining
		percent := t.PercentUsed

		fmt.Printf("%-40s %-15s %-15s %-10s %.1f%%\n",
			truncate(t.Name, 40),
			elapsed,
			timeout,
			remaining,
			percent)
	}
}

func taskEditCmd(args []string) {
	// Parse flags
	var newSchedule *string
	var newPriority *int
	var newContent *string
	var contentFile *string
	var removeField *string
	var bulkPattern string
	var dryRun bool
	var setDisabled *bool
	var addLabel string
	var removeLabel string

	// Fields that can be cleared with --remove
	removableFields := map[string]bool{
		"schedule": true, "allowed_tools": true, "pre_check": true,
		"on_success": true, "on_failure": true, "timeout": true,
		"persistent_cooldown": true, "persistent_max_runtime": true,
		"persistent_max_failures": true, "persistent_budget": true,
	}

	var nameArgs []string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--all":
			if i+1 >= len(args) {
				// --all without pattern matches all tasks
				bulkPattern = "*"
			} else if strings.HasPrefix(args[i+1], "-") {
				// Next arg is a flag, match all tasks
				bulkPattern = "*"
			} else {
				i++
				bulkPattern = args[i]
			}
		case "--dry-run":
			dryRun = true
		case "--disabled":
			v := true
			setDisabled = &v
		case "--enabled":
			v := false
			setDisabled = &v
		case "-s", "--schedule":
			if i+1 >= len(args) {
				log.Fatal("missing value for -s/--schedule")
			}
			i++
			newSchedule = &args[i]
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
			newPriority = &n
		case "--content":
			if i+1 >= len(args) {
				log.Fatal("missing value for --content")
			}
			i++
			newContent = &args[i]
		case "--content-file":
			if i+1 >= len(args) {
				log.Fatal("missing value for --content-file")
			}
			i++
			contentFile = &args[i]
		case "--remove", "--clear":
			if i+1 >= len(args) {
				log.Fatal("missing field name for --remove")
			}
			i++
			removeField = &args[i]
		case "--add-label":
			if i+1 >= len(args) {
				log.Fatal("missing value for --add-label")
			}
			i++
			addLabel = args[i]
		case "--remove-label":
			if i+1 >= len(args) {
				log.Fatal("missing value for --remove-label")
			}
			i++
			removeLabel = args[i]
		default:
			nameArgs = append(nameArgs, args[i])
		}
		i++
	}

	if newContent != nil && contentFile != nil {
		log.Fatal("cannot use both --content and --content-file")
	}

	if removeField != nil && (newSchedule != nil || newPriority != nil || newContent != nil || contentFile != nil) {
		log.Fatal("cannot combine --remove with other edit flags")
	}

	if removeField != nil && !removableFields[*removeField] {
		var fields []string
		for k := range removableFields {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		log.Fatalf("invalid field %q for --remove. Valid fields: %s", *removeField, strings.Join(fields, ", "))
	}

	// Read content from file if --content-file was provided
	if contentFile != nil {
		data, err := os.ReadFile(*contentFile)
		if err != nil {
			log.Fatalf("failed to read content file %q: %v", *contentFile, err)
		}
		s := string(data)
		newContent = &s
	}

	// Bulk edit mode: --all [pattern]
	if bulkPattern != "" {
		if newSchedule == nil && newPriority == nil && setDisabled == nil {
			log.Fatal("--all requires at least one edit flag: -s/--schedule, -p/--priority, --disabled, or --enabled")
		}
		if newContent != nil || contentFile != nil || removeField != nil {
			log.Fatal("--all does not support --content, --content-file, or --remove")
		}
		if newSchedule != nil && *newSchedule != "" && *newSchedule != "persistent" {
			if _, err := cron.Parse(*newSchedule); err != nil {
				log.Fatalf("invalid schedule %q: %v", *newSchedule, err)
			}
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

		// Ensure pattern has .md suffix for matching
		matchPattern := bulkPattern
		if !strings.HasSuffix(matchPattern, ".md") && !strings.HasSuffix(matchPattern, "*") {
			matchPattern += ".md"
		}

		var matched []project.Todo
		for _, t := range todos {
			ok, err := filepath.Match(matchPattern, t.Name)
			if err != nil {
				log.Fatalf("invalid pattern %q: %v", bulkPattern, err)
			}
			if ok {
				matched = append(matched, t)
			}
		}

		if len(matched) == 0 {
			fmt.Fprintf(os.Stderr, "no tasks match pattern: %s\n", bulkPattern)
			os.Exit(1)
		}

		if dryRun {
			fmt.Printf("dry run: would update %d task(s):\n", len(matched))
			for _, t := range matched {
				changes := []string{}
				if newSchedule != nil {
					changes = append(changes, fmt.Sprintf("schedule: %s -> %s", t.Schedule, *newSchedule))
				}
				if newPriority != nil {
					changes = append(changes, fmt.Sprintf("priority: p%d -> p%d", t.Priority, *newPriority))
				}
				if setDisabled != nil {
					changes = append(changes, fmt.Sprintf("disabled: %t -> %t", t.Disabled, *setDisabled))
				}
				fmt.Printf("  %s (%s)\n", t.Name, strings.Join(changes, ", "))
			}
			return
		}

		updated := 0
		for _, t := range matched {
			if err := taskEditApply(&t, abs, newSchedule, newPriority, setDisabled); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update %s: %v\n", t.Name, err)
				continue
			}
			updated++
		}
		fmt.Printf("updated %d task(s)\n", updated)
		return
	}

	if len(nameArgs) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task edit <name> [-s schedule] [-p priority] [--content text] [--content-file path] [--remove field] [--add-label L] [--remove-label L]\n")
		fmt.Fprintf(os.Stderr, "       anvil task edit --all [pattern] [-s schedule] [-p priority] [--disabled|--enabled] [--dry-run]\n")
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

	// Single-task --disabled/--enabled support
	if setDisabled != nil && len(nameArgs) > 0 {
		todo := findTodo(todos, nameArgs[0])
		if todo == nil {
			fmt.Fprintf(os.Stderr, "task not found: %s\n", nameArgs[0])
			os.Exit(1)
		}
		if err := taskEditApply(todo, abs, newSchedule, newPriority, setDisabled); err != nil {
			log.Fatalf("failed to update task: %v", err)
		}
		return
	}

	todo := findTodo(todos, nameArgs[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", nameArgs[0])
		os.Exit(1)
	}

	// Handle --add-label / --remove-label
	if addLabel != "" || removeLabel != "" {
		raw, err := os.ReadFile(todo.Path)
		if err != nil {
			log.Fatalf("failed to read task file: %v", err)
		}
		contentStr := string(raw)
		var fmMap map[string]interface{}
		body := contentStr

		if strings.HasPrefix(contentStr, "---\n") {
			parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
			if len(parts) == 2 {
				body = parts[1]
				if err := yaml.Unmarshal([]byte(parts[0]), &fmMap); err != nil {
					log.Fatalf("failed to parse front-matter: %v", err)
				}
			}
		}
		if fmMap == nil {
			fmMap = make(map[string]interface{})
		}

		// Get current labels
		var labels []string
		if raw, ok := fmMap["labels"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						labels = append(labels, s)
					}
				}
			}
		}

		if addLabel != "" {
			// Check for duplicate (case-insensitive)
			found := false
			for _, l := range labels {
				if strings.EqualFold(l, addLabel) {
					found = true
					break
				}
			}
			if !found {
				labels = append(labels, addLabel)
				fmt.Printf("added label %q to %s\n", addLabel, nameArgs[0])
			} else {
				fmt.Printf("task %s already has label %q\n", nameArgs[0], addLabel)
				return
			}
		}

		if removeLabel != "" {
			var newLabels []string
			found := false
			for _, l := range labels {
				if strings.EqualFold(l, removeLabel) {
					found = true
				} else {
					newLabels = append(newLabels, l)
				}
			}
			if !found {
				fmt.Printf("task %s does not have label %q\n", nameArgs[0], removeLabel)
				return
			}
			labels = newLabels
			fmt.Printf("removed label %q from %s\n", removeLabel, nameArgs[0])
		}

		if len(labels) > 0 {
			fmMap["labels"] = labels
		} else {
			delete(fmMap, "labels")
		}

		fmBytes, err := yaml.Marshal(fmMap)
		if err != nil {
			log.Fatalf("failed to marshal front-matter: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(string(fmBytes))
		sb.WriteString("---\n")
		sb.WriteString(body)
		if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
			log.Fatalf("failed to write task file: %v", err)
		}
		return
	}

	// Handle --remove flag: clear a field from frontmatter
	if removeField != nil {
		raw, err := os.ReadFile(todo.Path)
		if err != nil {
			log.Fatalf("failed to read task file: %v", err)
		}
		contentStr := string(raw)
		if !strings.HasPrefix(contentStr, "---\n") {
			log.Fatal("task has no front-matter to modify")
		}
		parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
		if len(parts) != 2 {
			log.Fatal("failed to parse task front-matter")
		}
		var fmMap map[string]interface{}
		if err := yaml.Unmarshal([]byte(parts[0]), &fmMap); err != nil {
			log.Fatalf("failed to parse front-matter: %v", err)
		}
		if _, exists := fmMap[*removeField]; !exists {
			fmt.Printf("field %q is not set — nothing to remove\n", *removeField)
			return
		}
		delete(fmMap, *removeField)
		fmBytes, err := yaml.Marshal(fmMap)
		if err != nil {
			log.Fatalf("failed to marshal front-matter: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(string(fmBytes))
		sb.WriteString("---\n")
		sb.WriteString(parts[1])
		if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
			log.Fatalf("failed to write task file: %v", err)
		}
		fmt.Printf("removed %s from %s\n", *removeField, todo.Name)
		return
	}

	// If targeted flags provided, apply them without opening editor
	if newSchedule != nil || newPriority != nil || newContent != nil {
		// Validate schedule if provided
		if newSchedule != nil && *newSchedule != "" && *newSchedule != "persistent" {
			if _, err := cron.Parse(*newSchedule); err != nil {
				log.Fatalf("invalid schedule %q: %v", *newSchedule, err)
			}
		}

		// Determine new priority (default to current)
		priority := todo.Priority
		if newPriority != nil {
			priority = *newPriority
		}

		// Read current file content
		raw, err := os.ReadFile(todo.Path)
		if err != nil {
			log.Fatalf("failed to read task file: %v", err)
		}

		// Parse and update front-matter
		contentStr := string(raw)
		body := contentStr

		if strings.HasPrefix(contentStr, "---\n") {
			parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
			if len(parts) == 2 {
				fm := parts[0]
				body = parts[1]

				var fmData struct {
					Schedule        string   `yaml:"schedule"`
					ID              string   `yaml:"id"`
					Resume          *bool    `yaml:"resume"`
					MaxConcurrent   int      `yaml:"max_concurrent"`
					SkipPermissions bool     `yaml:"skip_permissions"`
					AllowedTools    []string `yaml:"allowed_tools"`
					PreCheck        string   `yaml:"pre_check"`
					OnSuccess       string   `yaml:"on_success"`
					OnFailure       string   `yaml:"on_failure"`
				}
				if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
					// Update schedule if provided
					if newSchedule != nil {
						fmData.Schedule = *newSchedule
					}

					// Marshal back
					fmBytes, err := yaml.Marshal(fmData)
					if err != nil {
						log.Fatalf("failed to marshal front-matter: %v", err)
					}

					// Replace body content if --content was provided
					if newContent != nil {
						body = *newContent
						if !strings.HasSuffix(body, "\n") {
							body += "\n"
						}
					}

					// Build new file content
					var sb strings.Builder
					sb.WriteString("---\n")
					sb.WriteString(string(fmBytes))
					sb.WriteString("---\n")
					sb.WriteString(body)

					// If priority changed, move to new directory
					if priority != todo.Priority {
						newDir := filepath.Join(abs, ".anvil", "todos", fmt.Sprintf("p%d", priority))
						if err := os.MkdirAll(newDir, 0755); err != nil {
							log.Fatalf("failed to create priority directory: %v", err)
						}
						newPath := filepath.Join(newDir, filepath.Base(todo.Path))
						// Write updated content to new location before moving
						if err := os.WriteFile(newPath, []byte(sb.String()), 0644); err != nil {
							log.Fatalf("failed to write task file: %v", err)
						}
						// Remove old file
						if err := os.Remove(todo.Path); err != nil {
							log.Fatalf("failed to remove old task file: %v", err)
						}
						fmt.Printf("updated priority: p%d -> p%d\n", todo.Priority, priority)
					} else {
						// Write back in place
						if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
							log.Fatalf("failed to write task file: %v", err)
						}
					}

					if newSchedule != nil {
						fmt.Printf("updated schedule: %s\n", *newSchedule)
					}
					if newContent != nil {
						fmt.Printf("updated content\n")
					}
					return
				}
			}
		}
		log.Fatal("failed to parse task front-matter")
	}

	// No flags: open in editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, todo.Path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("editor exited with error: %v", err)
	}

	// Validate the edited file's schedule if it has one
	raw, err := os.ReadFile(todo.Path)
	if err == nil {
		contentStr := string(raw)
		if strings.HasPrefix(contentStr, "---\n") {
			parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
			if len(parts) == 2 {
				var fmData struct {
					Schedule string `yaml:"schedule"`
				}
				if err := yaml.Unmarshal([]byte(parts[0]), &fmData); err != nil {
					log.Printf("WARN: failed to parse frontmatter after edit: %v", err)
				} else if fmData.Schedule != "" && fmData.Schedule != "persistent" {
					if _, err := cron.Parse(fmData.Schedule); err != nil {
						log.Fatalf("invalid schedule %q after edit: %v", fmData.Schedule, err)
					}
				}
			}
		}
	}

	fmt.Printf("edited: %s\n", todo.Name)
}
