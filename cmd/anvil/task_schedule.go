package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

func taskNextCmd(args []string) {
	jsonOutput := false
	allProjects := false
	verbose := false
	var taskName string

	for _, a := range args {
		switch a {
		case "--json":
			jsonOutput = true
		case "--verbose", "-v":
			verbose = true
		case "--all":
			allProjects = true
		case "-h", "--help":
			fmt.Println("Usage: anvil task next [options] [task-name]")
			fmt.Println("")
			fmt.Println("Show the next scheduled run time for tasks.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --json      Output as JSON")
			fmt.Println("  --all       Show tasks from all watched projects")
			fmt.Println("  --verbose   Show time window constraints and quiet hours info")
			fmt.Println("  -h          Show this help")
			fmt.Println("")
			fmt.Println("If task-name is omitted, shows all tasks in the current project.")
			return
		default:
			taskName = a
		}
	}

	now := time.Now()

	if allProjects {
		// Load all watched projects
		watched, err := loadAllWatched()
		if err != nil {
			log.Fatalf("failed to read watched: %v", err)
		}
		if len(watched) == 0 {
			if jsonOutput {
				fmt.Println("[]")
			} else {
				fmt.Println("no watched projects")
			}
			return
		}

		type nextResult struct {
			Project string `json:"project"`
			Name    string `json:"name"`
			NextRun string `json:"next_run,omitempty"`
			In      string `json:"in,omitempty"`
			Error   string `json:"error,omitempty"`
		}
		var results []nextResult

		for _, w := range watched {
			proj, err := project.Load(w.Path)
			if err != nil {
				continue
			}
			todos, _ := proj.LoadTodos()
			for _, t := range todos {
				r := nextResult{
					Project: filepath.Base(w.Path),
					Name:    t.Name,
				}
				if t.Schedule == "" || t.Schedule == "once" {
					r.NextRun = "never (one-shot)"
				} else {
					p, err := cron.Parse(t.Schedule)
					if err != nil {
						r.Error = err.Error()
					} else {
						next, err := p.Next(now)
						if err != nil {
							r.Error = err.Error()
						} else {
							r.NextRun = next.Format("Mon Jan 2 15:04:05")
							r.In = formatDurationShort(time.Until(next))
						}
					}
				}
				results = append(results, r)
			}
		}

		if jsonOutput {
			data, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, r := range results {
				if r.Error != "" {
					fmt.Printf("%s/%s: error: %s\n", r.Project, r.Name, r.Error)
				} else {
					fmt.Printf("%s/%s: %s (%s)\n", r.Project, r.Name, r.NextRun, r.In)
				}
			}
		}
		return
	}

	// Single project mode
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

	if taskName != "" {
		// Show specific task
		todo := findTodo(todos, taskName)
		if todo == nil {
			fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
			os.Exit(1)
		}

		if todo.Schedule == "" || todo.Schedule == "once" {
			if jsonOutput {
				data, _ := json.MarshalIndent(map[string]string{
					"name": todo.Name, "next_run": "never", "note": "one-shot task"}, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("%s: never (one-shot task)\n", todo.Name)
			}
			return
		}

		// Load global config for quiet hours
		cfg, _ := config.Load()

		// Calculate window-aware next run
		next, err := daemon.NextAllowedRun(todo.Schedule, todo.Window, cfg.QuietHours, todo.Priority, now)
		if err != nil {
			if jsonOutput {
				data, _ := json.MarshalIndent(map[string]string{
					"name": todo.Name, "error": err.Error()}, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Fprintf(os.Stderr, "failed to calculate next run: %v\n", err)
			}
			os.Exit(1)
		}

		if jsonOutput {
			result := map[string]string{
				"name":     todo.Name,
				"schedule": todo.Schedule,
				"next_run": next.Format(time.RFC3339),
				"in":       formatDurationShort(time.Until(next)),
			}
			if todo.Window.Start != "" {
				result["window_start"] = todo.Window.Start
				result["window_end"] = todo.Window.End
				result["window_days"] = todo.Window.Days
			}
			if cfg.QuietHours.Enabled {
				result["quiet_hours"] = cfg.QuietHours.Start + "-" + cfg.QuietHours.End
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("%s: next run at %s (%s)\n", todo.Name, next.Format("Mon Jan 2 15:04:05 MST"), formatDurationShort(time.Until(next)))
			if verbose {
				if todo.Window.Start != "" {
					days := todo.Window.Days
					if days == "" {
						days = "all"
					}
					fmt.Printf("  window: %s–%s (days: %s)\n", todo.Window.Start, todo.Window.End, days)
				}
				if cfg.QuietHours.Enabled {
					fmt.Printf("  quiet hours: %s–%s (exempt: p%d and above)\n", cfg.QuietHours.Start, cfg.QuietHours.End, cfg.QuietHours.ExcludePriority)
				}
				if todo.Window.Start == "" && !cfg.QuietHours.Enabled {
					fmt.Println("  no time window constraints active")
				}
			}
		}
		return
	}

	// Show all tasks in current project
	type nextResult struct {
		Name    string `json:"name"`
		NextRun string `json:"next_run,omitempty"`
		In      string `json:"in,omitempty"`
		Error   string `json:"error,omitempty"`
	}
	var results []nextResult

	for _, t := range todos {
		r := nextResult{Name: t.Name}
		if t.Schedule == "" || t.Schedule == "once" {
			r.NextRun = "never (one-shot)"
		} else {
			p, err := cron.Parse(t.Schedule)
			if err != nil {
				r.Error = err.Error()
			} else {
				next, err := p.Next(now)
				if err != nil {
					r.Error = err.Error()
				} else {
					r.NextRun = next.Format("Mon Jan 2 15:04:05")
					r.In = formatDurationShort(time.Until(next))
				}
			}
		}
		results = append(results, r)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	} else {
		for _, r := range results {
			if r.Error != "" {
				fmt.Printf("%s: error: %s\n", r.Name, r.Error)
			} else {
				fmt.Printf("%s: %s (%s)\n", r.Name, r.NextRun, r.In)
			}
		}
	}
}

// taskExportCmd exports tasks to a JSON file.
