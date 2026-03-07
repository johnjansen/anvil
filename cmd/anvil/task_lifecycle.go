package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

func taskRunCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task run <name> [--force]\n")
		os.Exit(1)
	}

	// Parse --force flag
	force := false
	var filtered []string
	for _, a := range args {
		if a == "--force" {
			force = true
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task run <name> [--force]\n")
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

	todo := findTodo(todos, filtered[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", filtered[0])
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Fprintln(os.Stderr, "daemon not running — start it with: anvil watch")
		os.Exit(1)
	}

	if err := daemon.SendRunRequest(abs, todo.ID, todo.Name, force); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run task: %v\n", err)
		os.Exit(1)
	}

	msg := "▶ Dispatched %s for immediate execution\n"
	if force {
		msg = "▶ Dispatched %s for immediate execution (bypassing time windows)\n"
	}
	fmt.Printf(msg, todo.Name)
}

func taskKillCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task kill <name> [--checkpoint|-c]\n")
		os.Exit(1)
	}

	// Parse --checkpoint / -c flag
	checkpoint := false
	var filtered []string
	for _, a := range args {
		if a == "--checkpoint" || a == "-c" {
			checkpoint = true
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task kill <name> [--checkpoint|-c]\n")
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

	todo := findTodo(todos, filtered[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", filtered[0])
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	if checkpoint {
		gracePeriod := todo.CheckpointGracePeriod
		if gracePeriod <= 0 {
			gracePeriod = 30 * time.Second
		}
		fmt.Printf("Gracefully stopping task: %s (waiting up to %v for checkpoint save)...\n", filtered[0], gracePeriod.Round(time.Second))
	}

	if err := daemon.SendKillRequest(todo.ID, checkpoint); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if checkpoint {
		fmt.Printf("Task stopped with checkpoint: %s\n", filtered[0])
	} else {
		fmt.Printf("killed task: %s\n", filtered[0])
	}
}

func taskStopCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task stop <name>\n")
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
		fmt.Fprintln(os.Stderr, "daemon not running — start it with: anvil watch")
		os.Exit(1)
	}

	if err := daemon.SendStopRequest(todo.ID); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("stopped %s — will not be re-dispatched until started\n", todo.Name)
}

func taskStartCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task start <name>\n")
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
		fmt.Fprintln(os.Stderr, "daemon not running — start it with: anvil watch")
		os.Exit(1)
	}

	if err := daemon.SendStartRequest(abs, todo.ID, todo.Name); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("started %s — will be dispatched on next tick\n", todo.Name)
}

func taskHistoryCmd(args []string) {
	limit := 10
	showFailuresOnly := false
	showRetriedOnly := false
	showStats := false
	jsonOutput := false
	followMode := false
	versionsMode := false
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-n", "--limit":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "usage: anvil task history <name> [-n limit] [-f] [--failures] [--retried] [--stats] [--json]\n")
				os.Exit(1)
			}
			if _, err := fmt.Sscanf(args[i+1], "%d", &limit); err != nil {
				fmt.Fprintf(os.Stderr, "invalid limit: %s\n", args[i+1])
				os.Exit(1)
			}
			i += 2
		case "-f", "--follow":
			followMode = true
			i++
		case "--failures", "--show-failures-only":
			showFailuresOnly = true
			i++
		case "--retried":
			showRetriedOnly = true
			i++
		case "--stats":
			showStats = true
			i++
		case "--json":
			jsonOutput = true
			i++
		case "--versions":
			versionsMode = true
			i++
		default:
			break
		}
	}
	taskName := strings.Join(args[i:], " ")
	if taskName == "" {
		fmt.Fprintf(os.Stderr, "usage: anvil task history <name> [-n limit] [-f] [--failures] [--retried] [--stats] [--json] [--versions]\n")
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

	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	// Show version history if --versions flag is set
	if versionsMode {
		taskNameClean := strings.TrimSuffix(todo.Name, ".md")
		versions, err := project.ReadAllVersions(abs, taskNameClean)
		if err != nil {
			log.Fatalf("failed to read versions: %v", err)
		}
		if len(versions) == 0 {
			fmt.Printf("no versions found for task: %s\n", taskNameClean)
			return
		}
		if jsonOutput {
			data, err := json.MarshalIndent(versions, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal JSON: %v", err)
			}
			fmt.Println(string(data))
			return
		}
		fmt.Printf("%-10s %-20s %-14s %s\n", "VERSION", "DATE", "AUTHOR", "SUMMARY")
		for _, v := range versions {
			author := v.Author
			if len(author) > 14 {
				author = author[:14]
			}
			summary := v.Summary
			if len(summary) > 40 {
				summary = summary[:40] + "..."
			}
			fmt.Printf("%-10s %-20s %-14s %s\n",
				fmt.Sprintf("v%d", v.VersionNumber),
				v.Timestamp.Format("2006-01-02 15:04:05"),
				author,
				summary)
		}
		return
	}

	// In follow mode, wait for new runs to complete and display them
	if followMode {
		runFollowMode(abs, todo)
		return
	}

	records, err := project.ReadAllRunRecords(abs, todo.ID)
	if err != nil {
		log.Fatalf("failed to read run history: %v", err)
	}

	if len(records) == 0 {
		fmt.Println("no run history found")
		return
	}

	// Filter failures if requested
	if showFailuresOnly {
		var filtered []project.RunRecord
		for _, rec := range records {
			if !rec.Success {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	// Filter to only retried runs (attempt > 1)
	if showRetriedOnly {
		var filtered []project.RunRecord
		for _, rec := range records {
			if rec.Attempt > 1 {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	// Show retry statistics
	if showStats {
		total := len(records)
		succeeded := 0
		failed := 0
		retried := 0
		for _, rec := range records {
			if rec.Success {
				succeeded++
			} else {
				failed++
			}
			if rec.Attempt > 1 {
				retried++
			}
		}
		retriedPct := 0.0
		if total > 0 {
			retriedPct = float64(retried) / float64(total) * 100
		}
		fmt.Printf("Total: %d, Succeeded: %d, Failed: %d, Retried: %d (%.0f%%)\n", total, succeeded, failed, retried, retriedPct)
		return
	}

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	if jsonOutput {
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Print header
	fmt.Printf("%-20s %10s %10s %-12s %-12s %10s\n", "STARTED", "DURATION", "ATTEMPTS", "RUNNER", "NODE", "STATUS")
	for _, rec := range records {
		duration := ""
		if !rec.Finished.IsZero() {
			d := rec.Finished.Sub(rec.Started)
			if d < time.Minute {
				duration = fmt.Sprintf("%.0fs", d.Seconds())
			} else {
				duration = fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*float64(d.Minutes()))
			}
		}

		status := "ok"
		if !rec.Success {
			status = "failed"
			if rec.Error == "stopped-with-checkpoint" || rec.Error == "killed-after-grace-period" {
				// Show checkpoint-related statuses without truncation
				status = rec.Error
			} else if rec.Error != "" {
				// Truncate error for display, collapse newlines
				errMsg := strings.Join(strings.Fields(rec.Error), " ")
				if len(errMsg) > 20 {
					errMsg = errMsg[:20] + "..."
				}
				status = errMsg
			}
			// Annotate if all retries were exhausted
			if rec.MaxRetries > 0 && rec.Attempt >= rec.MaxRetries {
				status += " (retries exhausted)"
			}
		} else if rec.Attempt > 1 {
			// Succeeded after retries
			status = "ok (retry succeeded)"
		}

		// Format attempts column (e.g., "1/3" or "-" if no retries configured)
		attempts := "-"
		if rec.MaxRetries > 0 {
			attempts = fmt.Sprintf("%d/%d", rec.Attempt, rec.MaxRetries)
		} else if rec.Attempt > 0 {
			attempts = fmt.Sprintf("%d", rec.Attempt)
		}

		// Format runner column
		runnerLabel := "-"
		if rec.RunnerCommand != "" {
			runnerLabel = rec.RunnerCommand
			if len(runnerLabel) > 12 {
				runnerLabel = runnerLabel[:12]
			}
		} else if rec.RunnerIndex >= 100 {
			runnerLabel = "timeout-fb"
		} else if rec.RunnerIndex > 0 {
			runnerLabel = fmt.Sprintf("runner[%d]", rec.RunnerIndex)
		}

		nodeLabel := "-"
		if rec.NodeID != "" {
			nodeLabel = rec.NodeID
			if len(nodeLabel) > 12 {
				nodeLabel = nodeLabel[:12]
			}
		}
		fmt.Printf("%-20s %10s %10s %-12s %-12s %10s\n", rec.Started.Format("2006-01-02 15:04"), duration, attempts, runnerLabel, nodeLabel, status)

		// Print output summary if available
		if rec.OutputSummary != "" {
			summaryLines := strings.Split(rec.OutputSummary, "\n")
			for _, line := range summaryLines {
				fmt.Printf("  %s\n", line)
			}
		}

		// Print checkpoint data if available
		if rec.CheckpointData != "" {
			cpPreview := rec.CheckpointData
			if len(cpPreview) > 80 {
				cpPreview = cpPreview[:80] + "..."
			}
			fmt.Printf("  checkpoint: %s\n", cpPreview)
		}
	}
}

// runFollowMode watches for new runs and prints them as they complete.
func runFollowMode(projectPath string, todo *project.Todo) {
	fmt.Printf("Following runs for task %s (Ctrl+C to exit)...\n\n", todo.Name)

	lastRunID := ""
	for {
		records, err := project.ReadAllRunRecords(projectPath, todo.ID)
		if err != nil || len(records) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		// Get the most recent run
		latest := records[0]

		// If this is a new run we haven't displayed yet
		if latest.RunID != lastRunID {
			lastRunID = latest.RunID

			// Wait for the run to complete (Finished time is set)
			for latest.Finished.IsZero() {
				time.Sleep(1 * time.Second)
				records, err = project.ReadAllRunRecords(projectPath, todo.ID)
				if err != nil || len(records) == 0 {
					break
				}
				latest = records[0]
			}

			// Display the completed run
			duration := ""
			if !latest.Finished.IsZero() {
				d := latest.Finished.Sub(latest.Started)
				if d < time.Minute {
					duration = fmt.Sprintf("%.0fs", d.Seconds())
				} else {
					duration = fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*float64(d.Minutes()))
				}
			}

			status := "ok"
			if !latest.Success {
				status = "failed"
			}

			fmt.Printf("RUN %s: %s %s %s\n",
				latest.RunID[:8],
				latest.Started.Format("2006-01-02 15:04"),
				duration,
				status,
			)

			if latest.OutputSummary != "" {
				summaryLines := strings.Split(latest.OutputSummary, "\n")
				for _, line := range summaryLines {
					fmt.Printf("  %s\n", line)
				}
			}
			fmt.Println()
		}

		time.Sleep(2 * time.Second)
	}
}

func taskUnlockCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task unlock <name>\n")
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

	// Remove the lock file if it exists
	if err := project.RemoveLock(*todo); err != nil {
		log.Fatalf("failed to remove lock: %v", err)
	}

	project.WriteActivity(abs, project.ActivityEntry{
		Timestamp: time.Now(),
		Action:    "unlocked",
		TaskID:    todo.ID,
		TaskName:  todo.Name,
	})
	fmt.Printf("unlocked: %s\n", todo.Name)
}
