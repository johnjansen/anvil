package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

func followLog(sessionPath string, projectPath string, taskName string) {
	// Wait for the file to exist (task may not have started yet)
	for {
		if _, err := os.Stat(sessionPath); err == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "waiting for log file...\n")
		time.Sleep(500 * time.Millisecond)
	}

	f, err := os.Open(sessionPath)
	if err != nil {
		log.Fatalf("failed to open session log: %v", err)
	}
	defer f.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	buf := make([]byte, 4096)
	stableCount := 0

	for {
		select {
		case <-sigCh:
			return
		default:
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			stableCount = 0
			continue
		}
		if readErr != nil && readErr != io.EOF {
			return
		}

		// No new data — check periodically if task has finished
		stableCount++
		if stableCount%10 == 0 {
			taskRunning := false
			if daemon.IsDaemonRunning() {
				tasks, psErr := daemon.SendPsRequest()
				if psErr == nil {
					fullKey := fmt.Sprintf("%s/%s", projectPath, taskName)
					for _, t := range tasks {
						if t.Name == fullKey {
							taskRunning = true
							break
						}
					}
				}
			}
			if !taskRunning && taskName != "" {
				fmt.Println("[task completed]")
				return
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// rawLogPath returns the path to the raw stdout/stderr log file for a completed run.
// The daemon writes these files to <project>/.anvil/logs/<taskID>/<runID>.log
func rawLogPath(projectPath, taskID, runID string) string {
	return filepath.Join(projectPath, ".anvil", "logs", taskID, runID+".log")
}

func logsCmd(args []string) {
	if !daemon.IsDaemonRunning() {
		fmt.Fprintln(os.Stderr, "daemon not running")
		os.Exit(1)
	}

	// Parse --runs N flag
	runsCount := 1
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--runs" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "invalid --runs value: must be a positive integer")
				os.Exit(1)
			}
			runsCount = n
			i++ // skip the value
		} else if strings.HasPrefix(args[i], "--runs=") {
			val := strings.TrimPrefix(args[i], "--runs=")
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "invalid --runs value: must be a positive integer")
				os.Exit(1)
			}
			runsCount = n
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	if len(filteredArgs) == 0 {
		logsMultiplex()
		return
	}

	// Single task mode: anvil logs <name> [--runs N]
	name := filteredArgs[0]
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Resolve todo name -> task ID and full daemon key
	var taskID string
	var todoName string
	proj, err := project.Load(abs)
	if err == nil {
		todos, err := proj.LoadTodos()
		if err == nil {
			if todo := findTodo(todos, name); todo != nil {
				taskID = todo.ID
				todoName = todo.Name
			}
		}
	}

	// Check if the task is currently running (only for single-run mode)
	if runsCount == 1 {
		tasks, err := daemon.SendPsRequest()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get running tasks: %v\n", err)
			os.Exit(1)
		}

		fullKey := fmt.Sprintf("%s/%s", abs, todoName)
		for _, t := range tasks {
			if t.Name == fullKey && t.LogPath != "" {
				// Task is running — follow the live raw log
				followLog(t.LogPath, abs, todoName)
				return
			}
		}
	}

	// Task is not running — print log(s) from completed runs
	if taskID == "" {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", name)
		os.Exit(1)
	}

	if runsCount == 1 {
		// Single run: show latest log
		rec, err := project.ReadCurrentRunRecord(abs, taskID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "no run record found for task %s\n", name)
			os.Exit(1)
		}

		logPath := rawLogPath(abs, rec.TaskID, rec.RunID)
		data, err := os.ReadFile(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "no raw log found for task %s (looked at %s)\n", name, logPath)
				os.Exit(1)
			}
			log.Fatalf("failed to read raw log: %v", err)
		}
		fmt.Print(string(data))
	} else {
		// Multiple runs: show last N run logs with headers
		records, err := project.ReadAllRunRecords(abs, taskID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read run records for task %s: %v\n", name, err)
			os.Exit(1)
		}
		if len(records) == 0 {
			fmt.Fprintf(os.Stderr, "no run records found for task %s\n", name)
			os.Exit(1)
		}

		// Limit to requested count
		if runsCount > len(records) {
			runsCount = len(records)
		}

		// Print runs oldest-first for chronological reading
		for i := runsCount - 1; i >= 0; i-- {
			rec := records[i]
			status := "success"
			if !rec.Success {
				status = "failure"
			}
			fmt.Printf("=== Run %s [%s] %s → %s ===\n",
				rec.RunID,
				status,
				rec.Started.Format("2006-01-02 15:04:05"),
				rec.Finished.Format("15:04:05"))

			logPath := rawLogPath(abs, rec.TaskID, rec.RunID)
			data, err := os.ReadFile(logPath)
			if err != nil {
				fmt.Printf("  (log not available: %v)\n", err)
			} else {
				fmt.Print(string(data))
				if len(data) > 0 && data[len(data)-1] != '\n' {
					fmt.Println()
				}
			}
			if i > 0 {
				fmt.Println()
			}
		}
	}
}

// logsMultiplex follows raw output from all currently running tasks, prefixing
// each line with the task name. Exits when all followed tasks complete or on SIGINT.
func logsMultiplex() {
	tasks, err := daemon.SendPsRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get running tasks: %v\n", err)
		os.Exit(1)
	}

	// Collect tasks that have a raw log path
	type taskState struct {
		name    string
		logPath string
		file    *os.File
		offset  int64
		buf     []byte // partial line buffer
	}

	var states []*taskState
	for _, t := range tasks {
		if t.LogPath == "" {
			continue
		}
		f, err := os.Open(t.LogPath)
		if err != nil {
			continue
		}
		// Display name: strip project path prefix for readability
		displayName := t.Name
		if idx := strings.LastIndex(displayName, "/"); idx >= 0 {
			displayName = displayName[idx+1:]
		}
		// Strip .md suffix if present
		displayName = strings.TrimSuffix(displayName, ".md")
		states = append(states, &taskState{
			name:    displayName,
			logPath: t.LogPath,
			file:    f,
		})
	}

	if len(states) == 0 {
		fmt.Println("no tasks running")
		return
	}
	defer func() {
		for _, s := range states {
			s.file.Close()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// printLines flushes complete lines from a task's buffer to stdout with prefix.
	printLines := func(s *taskState, flushAll bool) {
		for {
			idx := -1
			for i, b := range s.buf {
				if b == '\n' {
					idx = i
					break
				}
			}
			if idx < 0 {
				if flushAll && len(s.buf) > 0 {
					fmt.Printf("%s: %s\n", s.name, string(s.buf))
					s.buf = s.buf[:0]
				}
				break
			}
			fmt.Printf("%s: %s\n", s.name, string(s.buf[:idx]))
			s.buf = s.buf[idx+1:]
		}
	}

	buf := make([]byte, 4096)
	psCheckTick := 0

	for {
		select {
		case <-sigCh:
			// Flush remaining partial lines before exit
			for _, s := range states {
				printLines(s, true)
			}
			return
		default:
		}

		// Read new bytes from each active task's log file
		for _, s := range states {
			if s.file == nil {
				continue
			}
			n, readErr := s.file.Read(buf)
			if n > 0 {
				s.buf = append(s.buf, buf[:n]...)
				printLines(s, false)
			}
			if readErr != nil && readErr != io.EOF {
				printLines(s, true)
				s.file.Close()
				s.file = nil
			}
		}

		// Periodically re-check /ps to remove finished tasks (~every 2s = 8 * 250ms)
		psCheckTick++
		if psCheckTick >= 8 {
			psCheckTick = 0
			running, psErr := daemon.SendPsRequest()
			if psErr == nil {
				runningPaths := make(map[string]bool)
				for _, t := range running {
					runningPaths[t.LogPath] = true
				}
				for _, s := range states {
					if s.file != nil && !runningPaths[s.logPath] {
						// Task finished — drain remaining bytes then close
						for {
							n, err := s.file.Read(buf)
							if n > 0 {
								s.buf = append(s.buf, buf[:n]...)
							}
							if err != nil {
								break
							}
						}
						printLines(s, true)
						s.file.Close()
						s.file = nil
					}
				}
			}
		}

		// Check if all tasks are done
		allDone := true
		for _, s := range states {
			if s.file != nil {
				allDone = false
				break
			}
		}
		if allDone {
			fmt.Println("all tasks completed")
			return
		}

		time.Sleep(250 * time.Millisecond)
	}
}

// taskEditApply applies schedule, priority, and/or disabled changes to a single task file.
