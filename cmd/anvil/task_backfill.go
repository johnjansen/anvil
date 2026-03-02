package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

func taskBackfillCmd(args []string) {
	jsonOutput := false
	allProjects := false
	var taskName string

	for _, a := range args {
		switch a {
		case "--json":
			jsonOutput = true
		case "--all":
			allProjects = true
		case "-h", "--help":
			fmt.Println("Usage: anvil task backfill [options] [task-name]")
			fmt.Println("")
			fmt.Println("Show pending backfill tasks that would run due to missed cron windows.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --json      Output as JSON")
			fmt.Println("  --all       Show backfill tasks from all watched projects")
			fmt.Println("  -h          Show this help")
			fmt.Println("")
			fmt.Println("If task-name is omitted, shows all backfill tasks in the current project.")
			return
		default:
			taskName = a
		}
	}

	// If daemon is running, use the live queue data
	if daemon.IsDaemonRunning() {
		tasks, err := daemon.SendQueueRequest()
		if err != nil {
			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "failed to get queue status: %v\n", err)
			}
			os.Exit(1)
		}

		// Filter for backfill-related tasks
		var backfillTasks []daemon.TaskQueueInfo
		for _, t := range tasks {
			// For now, we'll show all pending tasks as potential backfill candidates
			// In a more advanced implementation, we could check if the task has backfill enabled
			if t.Status == "pending" || t.Status == "skipped" {
				backfillTasks = append(backfillTasks, t)
			}
		}

		if jsonOutput {
			data, err := json.MarshalIndent(backfillTasks, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal JSON: %v", err)
			}
			fmt.Println(string(data))
			return
		}

		if len(backfillTasks) == 0 {
			fmt.Println("no backfill tasks pending")
			return
		}

		fmt.Printf("%-40s %-10s %-10s %s\n", "TASK", "PRIORITY", "STATUS", "SKIP REASON")
		fmt.Printf("%s\n", strings.Repeat("-", 90))

		for _, t := range backfillTasks {
			skipReason := t.SkipReason
			if skipReason == "" {
				skipReason = "-"
			}
			fmt.Printf("%-40s %-10d %-10s %s\n",
				truncateHelper(t.Name, 40),
				t.Priority,
				t.Status,
				skipReason)
		}
		return
	}

	// If daemon is not running, analyze projects directly
	paths := getWatchedProjectPaths(allProjects)
	if len(paths) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("no watched projects found")
		}
		return
	}

	var backfillInfo []BackfillTaskInfo

	for _, projectPath := range paths {
		proj, err := project.Load(projectPath)
		if err != nil {
			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "skip %s: %v\n", projectPath, err)
			}
			continue
		}

		allTodos, err := proj.LoadTodos()
		if err != nil {
			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "skip %s: failed to load todos: %v\n", projectPath, err)
			}
			continue
		}

		for _, t := range allTodos {
			// Skip if not matching task name filter
			if taskName != "" && !strings.Contains(t.Name, taskName) {
				continue
			}

			// Skip disabled tasks
			if t.Disabled {
				continue
			}

			// Check if backfill is enabled for this task
			if t.Backfill == nil || !t.Backfill.Enabled {
				continue
			}

			// Only check cron tasks (not persistent or one-shot)
			if t.Schedule == "" || t.IsPersistent() {
				continue
			}

			// Get last run time
			lastRunTime := time.Time{}
			records, err := project.ReadAllRunRecords(proj.Path, t.ID)
			if err == nil && len(records) > 0 {
				lastRunTime = records[0].Started
			}

			// If no previous runs, check if we should backfill from task creation time
			// For now, we'll only backfill if there were previous runs
			if !lastRunTime.IsZero() {
				// Parse the cron expression
				parser, err := cron.Parse(t.Schedule)
				if err != nil {
					continue
				}

				// Find the previous scheduled time before now
				now := time.Now()
				prevScheduled, err := parser.Prev(now)
				if err != nil || prevScheduled.IsZero() {
					continue
				}

				// Check if there are missed runs
				if prevScheduled.After(lastRunTime) {
					// There are missed runs, check if within max_delay
					delay := now.Sub(prevScheduled)
					if t.Backfill.MaxDelay == 0 || delay <= t.Backfill.MaxDelay {
						info := BackfillTaskInfo{
							Project:      filepath.Base(proj.Path),
							Name:         t.Name,
							Schedule:     t.Schedule,
							Scheduled:    prevScheduled,
							Missed:       prevScheduled.Sub(lastRunTime),
							Delay:        delay,
							MaxDelay:     t.Backfill.MaxDelay,
							WouldBackfill: true,
						}
						backfillInfo = append(backfillInfo, info)
					}
				}
			}
		}
	}

	// Sort by project/name for consistent output
	sort.Slice(backfillInfo, func(i, j int) bool {
		if backfillInfo[i].Project != backfillInfo[j].Project {
			return backfillInfo[i].Project < backfillInfo[j].Project
		}
		return backfillInfo[i].Name < backfillInfo[j].Name
	})

	if jsonOutput {
		data, err := json.MarshalIndent(backfillInfo, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	if len(backfillInfo) == 0 {
		fmt.Println("no backfill tasks pending")
		return
	}

	fmt.Printf("%-30s %-20s %-20s %-15s %-15s %s\n", "TASK", "SCHEDULED", "MISSED", "DELAY", "MAX_DELAY", "STATUS")
	fmt.Printf("%s\n", strings.Repeat("-", 120))

	for _, info := range backfillInfo {
		status := "backfill"
		if !info.WouldBackfill {
			status = "skipped"
		}
		fmt.Printf("%-30s %-20s %-20s %-15s %-15s %s\n",
			truncateHelper(info.Project+"/"+info.Name, 30),
			info.Scheduled.Format("2006-01-02 15:04"),
			formatDurationHelper(info.Missed),
			formatDurationHelper(info.Delay),
			formatDurationHelper(info.MaxDelay),
			status)
	}
}

// BackfillTaskInfo holds information about a task eligible for backfill
type BackfillTaskInfo struct {
	Project       string        `json:"project"`
	Name          string        `json:"name"`
	Schedule      string        `json:"schedule"`
	Scheduled     time.Time     `json:"scheduled"`
	Missed        time.Duration `json:"missed"`
	Delay         time.Duration `json:"delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	WouldBackfill bool          `json:"would_backfill"`
}

func formatDurationHelper(d time.Duration) string {
	if d == 0 {
		return "unlimited"
	}
	return d.String()
}

func truncateHelper(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// getWatchedProjectPaths returns project paths from watched directories
func getWatchedProjectPaths(allProjects bool) []string {
	var paths []string

	if allProjects {
		watched, err := loadAllWatched()
		if err != nil {
			return nil
		}
		for _, w := range watched {
			paths = append(paths, w.Path)
		}
	} else {
		abs, err := filepath.Abs(".")
		if err != nil {
			return nil
		}
		paths = append(paths, abs)
	}

	return paths
}