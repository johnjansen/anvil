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
)

func taskWaitCmd(args []string) {
	var timeoutDur time.Duration
	matchPattern := ""
	var nameArgs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--timeout", "-t":
			if i+1 >= len(args) {
				log.Fatal("missing value for --timeout")
			}
			i++
			var err error
			timeoutDur, err = time.ParseDuration(args[i])
			if err != nil {
				log.Fatalf("invalid timeout duration: %v", err)
			}
		case "--match", "-m":
			if i+1 >= len(args) {
				log.Fatal("missing value for --match")
			}
			i++
			matchPattern = strings.ToLower(args[i])
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil task wait <name> [--timeout DURATION]
       anvil task wait --match <pattern> [--timeout DURATION]

Block until a running task completes.

Options:
  --timeout, -t DUR   Cancel wait after duration (e.g., 5m, 1h)
  --match, -m PAT     Wait for first task matching pattern (case-insensitive)

Exit codes:
  0  Task completed successfully
  1  Task failed
  2  Wait timed out
`)
			os.Exit(0)
		default:
			nameArgs = append(nameArgs, args[i])
		}
	}

	if len(nameArgs) == 0 && matchPattern == "" {
		fmt.Fprintf(os.Stderr, "usage: anvil task wait <name> [--timeout DURATION]\n")
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Fprintln(os.Stderr, "daemon is not running")
		os.Exit(1)
	}

	// Build the target name for matching
	targetName := ""
	if len(nameArgs) > 0 {
		targetName = nameArgs[0]
		if !strings.HasSuffix(targetName, ".md") {
			targetName += ".md"
		}
	}

	// Check that the task is actually running before we start waiting
	tasks, err := daemon.SendPsRequest()
	if err != nil {
		log.Fatalf("failed to query daemon: %v", err)
	}

	found := findRunningTask(tasks, targetName, matchPattern)
	if found == nil {
		if targetName != "" {
			fmt.Fprintf(os.Stderr, "task not currently running: %s\n", targetName)
		} else {
			fmt.Fprintf(os.Stderr, "no running task matches pattern: %s\n", matchPattern)
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "waiting for %s ...\n", found.Name)

	// Set up timeout if specified
	var deadline <-chan time.Time
	if timeoutDur > 0 {
		deadline = time.After(timeoutDur)
	}

	// Poll until the task is no longer running
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	waitingFor := found.Name
	for {
		select {
		case <-deadline:
			fmt.Fprintf(os.Stderr, "timed out after %s\n", timeoutDur)
			os.Exit(2)
		case <-ticker.C:
			tasks, err := daemon.SendPsRequest()
			if err != nil {
				// Daemon may have gone away — treat as task completed
				fmt.Fprintf(os.Stderr, "daemon unreachable, assuming task completed\n")
				os.Exit(0)
			}
			still := false
			for _, t := range tasks {
				if t.Name == waitingFor {
					still = true
					break
				}
			}
			if !still {
				// Task is no longer running — check last run record for exit status
				exitCode := checkTaskResult(waitingFor)
				if exitCode == 0 {
					fmt.Fprintf(os.Stderr, "task completed successfully: %s\n", waitingFor)
				} else {
					fmt.Fprintf(os.Stderr, "task failed: %s\n", waitingFor)
				}
				os.Exit(exitCode)
			}
		}
	}
}

// findRunningTask finds a running task by exact name or pattern match.
func findRunningTask(tasks []daemon.TaskInfo, targetName, matchPattern string) *daemon.TaskInfo {
	for i := range tasks {
		if targetName != "" && tasks[i].Name == targetName {
			return &tasks[i]
		}
		if matchPattern != "" && strings.Contains(strings.ToLower(tasks[i].Name), matchPattern) {
			return &tasks[i]
		}
	}
	return nil
}

// checkTaskResult checks the most recent run record for a task to determine if it succeeded.
// Returns 0 for success, 1 for failure.
func checkTaskResult(taskName string) int {
	abs, err := filepath.Abs(".")
	if err != nil {
		return 1
	}
	proj, err := project.Load(abs)
	if err != nil {
		return 1
	}
	todos, err := proj.LoadTodos()
	if err != nil {
		return 1
	}
	todo := findTodo(todos, taskName)
	if todo == nil {
		// Task may have been a one-shot that was removed on success
		return 0
	}
	if todo.ID == "" {
		return 0
	}
	rec, err := project.ReadCurrentRunRecord(abs, todo.ID)
	if err != nil {
		// No record found — assume success
		return 0
	}
	if rec.Success {
		return 0
	}
	return 1
}

// suggestStagger prints a hint for staggering overlapping schedules.
