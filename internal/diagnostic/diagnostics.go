// Package diagnostic provides health check utilities for anvil.
package diagnostic

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
)

// CheckResult represents the outcome of a single diagnostic check.
type CheckResult struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"` // "pass", "warn", "fail"
	Message string   `json:"message"`
	Fix     string   `json:"fix,omitempty"`
}

// All runs all diagnostic checks and returns the results.
func All() []CheckResult {
	var results []CheckResult

	results = append(results, checkDaemon()...)
	results = append(results, checkConfig()...)
	results = append(results, checkWatchedProjects()...)
	results = append(results, checkTasks()...)
	results = append(results, checkRecentRuns()...)

	return results
}

// checkDaemon verifies the daemon is running and responsive.
func checkDaemon() []CheckResult {
	var results []CheckResult

	// Check PID file
	pidPath := config.PidFile()
	pidData, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		results = append(results, CheckResult{
			Name:   "daemon_pid",
			Status: "warn",
			Message: "PID file does not exist",
			Fix:    "Start the daemon with 'anvil watch'",
		})
		// Can't check further without PID
		return results
	}

	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(pidData)), "%d", &pid); err != nil {
		results = append(results, CheckResult{
			Name:   "daemon_pid",
			Status: "fail",
			Message: "PID file is malformed",
			Fix:    "Remove " + pidPath + " and restart daemon",
		})
		return results
	}

	// Check if process is running
	if err := syscallExists(pid); err != nil {
		results = append(results, CheckResult{
			Name:   "daemon_process",
			Status: "fail",
			Message: fmt.Sprintf("Daemon process (PID %d) is not running", pid),
			Fix:    "Remove " + pidPath + " and restart daemon with 'anvil watch'",
		})
	} else {
		results = append(results, CheckResult{
			Name:   "daemon_process",
			Status: "pass",
			Message: fmt.Sprintf("Daemon process (PID %d) is running", pid),
		})
	}

	// Check socket connectivity
	sockPath := filepath.Join(config.Dir(), "daemon.sock")
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		results = append(results, CheckResult{
			Name:   "daemon_socket",
			Status: "fail",
			Message: "Cannot connect to daemon socket",
			Fix:    "The daemon may have crashed. Restart with 'anvil watch'",
		})
	} else {
		conn.Close()
		results = append(results, CheckResult{
			Name:   "daemon_socket",
			Status: "pass",
			Message: "Daemon socket is responsive",
		})
	}

	return results
}

// checkConfig verifies the config file is valid.
func checkConfig() []CheckResult {
	var results []CheckResult

	cfgPath := config.Path()

	// Check if config exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		results = append(results, CheckResult{
			Name:   "config_exists",
			Status: "warn",
			Message: "Config file does not exist",
			Fix:    "Run 'anvil init' to create a default config",
		})
		return results
	}

	results = append(results, CheckResult{
		Name:   "config_exists",
		Status: "pass",
		Message: "Config file exists",
	})

	// Try to parse config
	cfg, err := config.Load()
	if err != nil {
		results = append(results, CheckResult{
			Name:   "config_parse",
			Status: "fail",
			Message: fmt.Sprintf("Config file is invalid: %v", err),
			Fix:    "Fix the syntax errors in " + cfgPath,
		})
		return results
	}

	results = append(results, CheckResult{
		Name:   "config_parse",
		Status: "pass",
		Message: "Config file parses successfully",
	})

	// Check runners are configured
	if len(cfg.Runners) == 0 {
		results = append(results, CheckResult{
			Name:   "config_runners",
			Status: "warn",
			Message: "No runners configured",
			Fix:    "Add runners to " + cfgPath,
		})
	} else {
		results = append(results, CheckResult{
			Name:   "config_runners",
			Status: "pass",
			Message: fmt.Sprintf("%d runner(s) configured", len(cfg.Runners)),
		})
	}

	// Check max_workers
	if cfg.MaxWorkers <= 0 {
		results = append(results, CheckResult{
			Name:   "config_max_workers",
			Status: "warn",
			Message: "max_workers is not set or is zero",
			Fix:    "Set max_workers in " + cfgPath,
		})
	} else {
		results = append(results, CheckResult{
			Name:   "config_max_workers",
			Status: "pass",
			Message: fmt.Sprintf("max_workers set to %d", cfg.MaxWorkers),
		})
	}

	return results
}

// checkWatchedProjects verifies all watched project paths exist.
func checkWatchedProjects() []CheckResult {
	var results []CheckResult

	watchedDir := config.WatchedDir()
	entries, err := os.ReadDir(watchedDir)
	if os.IsNotExist(err) {
		results = append(results, CheckResult{
			Name:   "watched_projects",
			Status: "warn",
			Message: "No watched projects directory",
			Fix:    "Add a project with 'anvil project add <path>'",
		})
		return results
	}

	if len(entries) == 0 {
		results = append(results, CheckResult{
			Name:   "watched_projects",
			Status: "warn",
			Message: "No projects being watched",
			Fix:    "Add a project with 'anvil project add <path>'",
		})
		return results
	}

	var missing []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(watchedDir, e.Name())
		target, err := os.Readlink(path)
		if err != nil {
			continue // not a symlink, skip
		}
		// Resolve relative symlinks
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		if _, err := os.Stat(target); os.IsNotExist(err) {
			missing = append(missing, target)
		}
	}

	if len(missing) > 0 {
		results = append(results, CheckResult{
			Name:   "watched_projects",
			Status: "fail",
			Message: fmt.Sprintf("%d watched project(s) no longer exist", len(missing)),
			Fix:    "Run 'anvil project rm <path>' for: " + strings.Join(missing, ", "),
		})
	} else {
		results = append(results, CheckResult{
			Name:   "watched_projects",
			Status: "pass",
			Message: fmt.Sprintf("%d project(s) being watched", len(entries)),
		})
	}

	return results
}

// checkTasks verifies task files have valid frontmatter.
func checkTasks() []CheckResult {
	var results []CheckResult

	watchedDir := config.WatchedDir()
	entries, err := os.ReadDir(watchedDir)
	if os.IsNotExist(err) || len(entries) == 0 {
		// No projects, nothing to check
		return results
	}

	var staleLocks []string
	var invalidCron []string
	var missingIDs []string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		projPath := filepath.Join(watchedDir, e.Name())
		target, err := os.Readlink(projPath)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(projPath), target)
		}

		proj, err := project.Load(target)
		if err != nil {
			continue
		}

		todos, err := proj.LoadTodos()
		if err != nil {
			continue
		}

		for _, todo := range todos {
			// Check for stale locks
			if todo.IsLocked {
				staleLocks = append(staleLocks, todo.Name)
			}

			// Check for missing ID
			if todo.ID == "" {
				missingIDs = append(missingIDs, todo.Name)
			}

			// Check for invalid cron
			if todo.Schedule != "" && todo.Schedule != "persistent" {
				if _, err := cron.Parse(todo.Schedule); err != nil {
					invalidCron = append(invalidCron, fmt.Sprintf("%s (%s)", todo.Name, todo.Schedule))
				}
			}
		}
	}

	if len(staleLocks) > 0 {
		results = append(results, CheckResult{
			Name:   "task_locks",
			Status: "fail",
			Message: fmt.Sprintf("%d task(s) have stale lock files", len(staleLocks)),
			Fix:    "Run 'anvil task unlock <name>' for each affected task",
		})
	} else {
		results = append(results, CheckResult{
			Name:   "task_locks",
			Status: "pass",
			Message: "No stale lock files found",
		})
	}

	if len(missingIDs) > 0 {
		results = append(results, CheckResult{
			Name:   "task_ids",
			Status: "warn",
			Message: fmt.Sprintf("%d task(s) are missing IDs", len(missingIDs)),
			Fix:    "Recreate tasks with 'anvil add' to fix",
		})
	} else {
		results = append(results, CheckResult{
			Name:   "task_ids",
			Status: "pass",
			Message: "All tasks have valid IDs",
		})
	}

	if len(invalidCron) > 0 {
		results = append(results, CheckResult{
			Name:   "task_cron",
			Status: "fail",
			Message: fmt.Sprintf("%d task(s) have invalid cron schedules", len(invalidCron)),
			Fix:    "Fix cron expressions: " + strings.Join(invalidCron, ", "),
		})
	} else {
		results = append(results, CheckResult{
			Name:   "task_cron",
			Status: "pass",
			Message: "All cron schedules are valid",
		})
	}

	return results
}

// checkRecentRuns looks for consistent failures in recent runs.
func checkRecentRuns() []CheckResult {
	var results []CheckResult

	watchedDir := config.WatchedDir()
	entries, err := os.ReadDir(watchedDir)
	if os.IsNotExist(err) || len(entries) == 0 {
		return results
	}

	type failureInfo struct {
		taskID  string
		name    string
		count   int
	}
	var failures []failureInfo

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		projPath := filepath.Join(watchedDir, e.Name())
		target, err := os.Readlink(projPath)
		if err != nil {
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(projPath), target)
		}

		proj, err := project.Load(target)
		if err != nil {
			continue
		}

		todos, err := proj.LoadTodos()
		if err != nil {
			continue
		}

		for _, todo := range todos {
			if todo.ID == "" {
				continue
			}

			// Check last 10 runs
			records, err := project.ReadAllRunRecords(target, todo.ID)
			if err != nil {
				continue
			}

			// Only check runs from the last 24 hours
			cutoff := time.Now().Add(-24 * time.Hour)
			var recentFailures int
			for _, rec := range records {
				if rec.Started.Before(cutoff) {
					break
				}
				if !rec.Success {
					recentFailures++
				}
			}

			if recentFailures >= 3 {
				failures = append(failures, failureInfo{
					taskID: todo.ID,
					name:   todo.Name,
					count:  recentFailures,
				})
			}
		}
	}

	if len(failures) > 0 {
		msg := "Recent failures: "
		for _, f := range failures {
			msg += fmt.Sprintf("%s (%d failures), ", f.name, f.count)
		}
		msg = strings.TrimSuffix(msg, ", ")
		results = append(results, CheckResult{
			Name:   "recent_runs",
			Status: "warn",
			Message: msg,
			Fix:    "Check task logs with 'anvil task log <name>' for details",
		})
	} else {
		results = append(results, CheckResult{
			Name:   "recent_runs",
			Status: "pass",
			Message: "No consistent failures in recent runs",
		})
	}

	return results
}

// syscallExists checks if a process with the given PID exists.
func syscallExists(pid int) error {
	// On Unix, we can use kill(pid, 0) to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = process.Signal(os.Signal(nil))
	if err != nil {
		return err
	}
	return nil
}
