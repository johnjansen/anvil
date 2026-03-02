package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"gopkg.in/yaml.v3"
)

func stopOnIdleCmd() {
	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	if err := daemon.SendDrainRequest(); err != nil {
		fmt.Printf("failed to set stop-on-idle: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("stop-on-idle: daemon will finish running tasks then exit")
}

func taskPauseCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task pause <name>\n")
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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", args[0])
		os.Exit(1)
	}

	if todo.Disabled {
		fmt.Printf("task %s is already paused\n", args[0])
		return
	}

	// Read current file content
	raw, err := os.ReadFile(todo.Path)
	if err != nil {
		log.Fatalf("failed to read task file: %v", err)
	}

	// Parse and update front-matter to add disabled: true
	contentStr := string(raw)
	body := contentStr

	if strings.HasPrefix(contentStr, "---\n") {
		parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
		if len(parts) == 2 {
			fm := parts[0]
			body = parts[1]

			var fmData struct {
				Schedule             string   `yaml:"schedule"`
				ID                   string   `yaml:"id"`
				Resume               *bool    `yaml:"resume"`
				MaxConcurrent        int      `yaml:"max_concurrent"`
				SkipPermissions      bool     `yaml:"skip_permissions"`
				AllowedTools         []string `yaml:"allowed_tools"`
				PreCheck             string   `yaml:"pre_check"`
				OnSuccess            string   `yaml:"on_success"`
				OnFailure            string   `yaml:"on_failure"`
				Disabled             bool     `yaml:"disabled"`
				Timeout              string   `yaml:"timeout"`
				Retry                int      `yaml:"retry"`
				RetryDelay           string   `yaml:"retry_delay"`
				PersistentCooldown   string   `yaml:"persistent_cooldown"`
				PersistentMaxRuntime string   `yaml:"persistent_max_runtime"`
			}
			if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
				// Set disabled to true
				fmData.Disabled = true

				// Marshal back
				fmBytes, err := yaml.Marshal(fmData)
				if err != nil {
					log.Fatalf("failed to marshal front-matter: %v", err)
				}

				// Build new file content
				var sb strings.Builder
				sb.WriteString("---\n")
				sb.WriteString(string(fmBytes))
				sb.WriteString("---\n")
				sb.WriteString(body)

				// Write back in place
				if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
					log.Fatalf("failed to write task file: %v", err)
				}

				fmt.Printf("paused: %s\n", args[0])
				return
			}
		}
	}

	// No front-matter exists, create one with disabled: true
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("disabled: true\n")
	sb.WriteString("---\n")
	sb.WriteString(contentStr)

	if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
		log.Fatalf("failed to write task file: %v", err)
	}

	project.WriteActivity(abs, project.ActivityEntry{
		Timestamp: time.Now(),
		Action:    "paused",
		TaskID:    todo.ID,
		TaskName:  todo.Name,
	})
	fmt.Printf("paused: %s\n", args[0])
}

func taskResumeCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task resume <name>\n")
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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", args[0])
		os.Exit(1)
	}

	if !todo.Disabled {
		fmt.Printf("task %s is not paused\n", args[0])
		return
	}

	// Read current file content
	raw, err := os.ReadFile(todo.Path)
	if err != nil {
		log.Fatalf("failed to read task file: %v", err)
	}

	// Parse and update front-matter to remove disabled field
	contentStr := string(raw)
	body := contentStr

	if strings.HasPrefix(contentStr, "---\n") {
		parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
		if len(parts) == 2 {
			fm := parts[0]
			body = parts[1]

			var fmData struct {
				Schedule             string   `yaml:"schedule"`
				ID                   string   `yaml:"id"`
				Resume               *bool    `yaml:"resume"`
				MaxConcurrent        int      `yaml:"max_concurrent"`
				SkipPermissions      bool     `yaml:"skip_permissions"`
				AllowedTools         []string `yaml:"allowed_tools"`
				PreCheck             string   `yaml:"pre_check"`
				OnSuccess            string   `yaml:"on_success"`
				OnFailure            string   `yaml:"on_failure"`
				Disabled             bool     `yaml:"disabled"`
				Timeout              string   `yaml:"timeout"`
				Retry                int      `yaml:"retry"`
				RetryDelay           string   `yaml:"retry_delay"`
				PersistentCooldown   string   `yaml:"persistent_cooldown"`
				PersistentMaxRuntime string   `yaml:"persistent_max_runtime"`
			}
			if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
				// Set disabled to false (YAML will omit if false when marshaling)
				fmData.Disabled = false

				// Marshal back
				fmBytes, err := yaml.Marshal(fmData)
				if err != nil {
					log.Fatalf("failed to marshal front-matter: %v", err)
				}

				// Build new file content (without the disabled line if it's false)
				fmStr := string(fmBytes)
				// Remove "disabled: false\n" if present
				fmStr = strings.ReplaceAll(fmStr, "disabled: false\n", "")
				fmStr = strings.ReplaceAll(fmStr, "disabled: false", "")

				var sb strings.Builder
				sb.WriteString("---\n")
				sb.WriteString(fmStr)
				sb.WriteString("---\n")
				sb.WriteString(body)

				// Write back in place
				if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
					log.Fatalf("failed to write task file: %v", err)
				}

				project.WriteActivity(abs, project.ActivityEntry{
					Timestamp: time.Now(),
					Action:    "resumed",
					TaskID:    todo.ID,
					TaskName:  todo.Name,
				})
				fmt.Printf("resumed: %s\n", args[0])
				return
			}
		}
	}

	fmt.Printf("task %s has no front-matter to resume\n", args[0])
}

func taskStopOnIdleCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task stop-on-idle <name>\n")
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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", args[0])
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	if err := daemon.SendDrainTaskRequest(todo.ID); err != nil {
		fmt.Printf("failed to set stop-on-idle for task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("stop-on-idle: task %s will not be rescheduled after its current run\n", todo.Name)
}

// taskNextCmd shows the next scheduled run time for tasks.
