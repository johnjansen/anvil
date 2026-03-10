package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/internal/tui"
)

func statusCmd(args []string) {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		}
	}

	watched, err := loadAllWatched()
	if err != nil {
		log.Fatalf("failed to read watched: %v", err)
	}

	daemonRunning := daemon.IsDaemonRunning()
	draining := false
	if daemonRunning {
		if status, err := daemon.SendStatusRequest(); err == nil && status.Draining {
			draining = true
		}
	}

	// Fetch throttle state if daemon is running
	var throttlePaused bool
	var throttleRate int
	var pausedLabels []string
	if daemonRunning {
		if status, err := daemon.SendStatusRequest(); err == nil {
			throttlePaused = status.Paused
			throttleRate = status.ThrottleRate
			pausedLabels = status.PausedLabels
		}
	}

	if jsonOutput {
		type projectStatusJSON struct {
			Path  string `json:"path"`
			Tasks int    `json:"tasks"`
			Error string `json:"error,omitempty"`
		}
		type statusJSON struct {
			DaemonRunning bool                `json:"daemon_running"`
			Draining      bool                `json:"draining"`
			Paused        bool                `json:"paused"`
			ThrottleRate  int                 `json:"throttle_rate,omitempty"`
			PausedLabels  []string            `json:"paused_labels,omitempty"`
			Projects      []projectStatusJSON `json:"projects"`
		}
		st := statusJSON{
			DaemonRunning: daemonRunning,
			Draining:      draining,
			Paused:        throttlePaused,
			ThrottleRate:  throttleRate,
			PausedLabels:  pausedLabels,
			Projects:      []projectStatusJSON{},
		}
		for _, w := range watched {
			proj, err := project.Load(w.Path)
			if err != nil {
				st.Projects = append(st.Projects, projectStatusJSON{
					Path:  w.Path,
					Error: err.Error(),
				})
				continue
			}
			todos, _ := proj.LoadTodos()
			st.Projects = append(st.Projects, projectStatusJSON{
				Path:  w.Path,
				Tasks: len(todos),
			})
		}
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Show daemon drain state if running
	if draining {
		fmt.Println("daemon: draining (stop-on-idle active)")
	}

	// Show throttle state
	if throttlePaused {
		fmt.Println("daemon: PAUSED (all task dispatching suspended)")
	}
	if throttleRate > 0 {
		fmt.Printf("daemon: throttle rate %d/m\n", throttleRate)
	}
	if len(pausedLabels) > 0 {
		fmt.Printf("daemon: paused labels: %s\n", strings.Join(pausedLabels, ", "))
	}

	if len(watched) == 0 {
		fmt.Println("no watched projects")
		return
	}

	for _, w := range watched {
		proj, err := project.Load(w.Path)
		if err != nil {
			fmt.Printf("  %s  (error: %v)\n", w.Path, err)
			continue
		}
		todos, _ := proj.LoadTodos()
		fmt.Printf("  %s  todos=%d\n", w.Path, len(todos))
	}
}

func reloadCmd(args []string) {
	graceful := false
	timeoutStr := ""

	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil reload [--graceful] [--timeout duration]

Reload the daemon configuration without restarting.

Sends SIGHUP to the daemon to reload ~/.anvil/config.yaml.
New tasks will use the updated config; running tasks are unaffected.

Options:
  --graceful           Wait for running tasks to complete before reloading
  --timeout duration   Max time to wait for tasks (default: 5m). Forces reload after timeout.
`)
			return
		case "--graceful":
			graceful = true
		default:
			if strings.HasPrefix(a, "--timeout=") {
				timeoutStr = strings.TrimPrefix(a, "--timeout=")
			} else if strings.HasPrefix(a, "--timeout") {
				// handled below with next arg
			}
		}
	}

	// Parse --timeout value (may be space-separated)
	for i := 0; i < len(args); i++ {
		if args[i] == "--timeout" && i+1 < len(args) {
			timeoutStr = args[i+1]
			break
		}
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	if !graceful {
		// Immediate reload (original behavior)
		if err := daemon.SendReloadRequest(); err != nil {
			fmt.Printf("failed to reload config: %v\n", err)
			return
		}
		fmt.Println("config reload triggered")
		return
	}

	// Graceful reload: wait for running tasks to complete first
	timeoutDur := 5 * time.Minute
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			log.Fatalf("invalid timeout duration: %s (use format like 5m, 30s, 1h)", timeoutStr)
		}
		timeoutDur = d
	}

	// Check for running tasks
	tasks, err := daemon.SendPsRequest()
	if err != nil {
		log.Fatalf("failed to check running tasks: %v", err)
	}

	if len(tasks) == 0 {
		// No running tasks, reload immediately
		if err := daemon.SendReloadRequest(); err != nil {
			fmt.Printf("failed to reload config: %v\n", err)
			return
		}
		fmt.Println("no running tasks — config reload triggered")
		return
	}

	fmt.Printf("Waiting for %d running task(s) to complete (timeout: %s)...\n", len(tasks), timeoutDur)
	for _, t := range tasks {
		fmt.Printf("  - %s/%s (running %s)\n", t.Project, t.Name, t.Elapsed)
	}

	deadline := time.After(timeoutDur)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	startTime := time.Now()

	for {
		select {
		case <-deadline:
			fmt.Fprintf(os.Stderr, "\nTimeout reached (%s). Force-reloading daemon...\n", timeoutDur)
			if err := daemon.SendReloadRequest(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to reload config: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("config reload triggered (forced after timeout)")
			return
		case <-ticker.C:
			tasks, err := daemon.SendPsRequest()
			if err != nil {
				fmt.Fprintf(os.Stderr, "daemon unreachable: %v\n", err)
				os.Exit(1)
			}
			if len(tasks) == 0 {
				// All tasks finished
				if err := daemon.SendReloadRequest(); err != nil {
					fmt.Fprintf(os.Stderr, "failed to reload config: %v\n", err)
					os.Exit(1)
				}
				elapsed := time.Since(startTime).Round(time.Second)
				fmt.Printf("\nAll tasks completed (%s). Config reload triggered.\n", elapsed)
				return
			}
			elapsed := time.Since(startTime).Round(time.Second)
			remaining := timeoutDur - time.Since(startTime)
			if remaining < 0 {
				remaining = 0
			}
			fmt.Printf("\r  %d task(s) still running... (%s elapsed, %s remaining)  ", len(tasks), elapsed, remaining.Round(time.Second))
		}
	}
}

func cleanupCmd(args []string) {
	olderThan := ""
	dryRun := false

	for _, a := range args {
		if a == "--dry-run" || a == "-n" {
			dryRun = true
		} else if strings.HasPrefix(a, "--older-than=") {
			olderThan = strings.TrimPrefix(a, "--older-than=")
		} else if strings.HasPrefix(a, "-o=") {
			olderThan = strings.TrimPrefix(a, "-o=")
		}
	}

	var maxAge time.Duration
	if olderThan != "" {
		var err error
		// ParseDuration doesn't support "d" suffix, only "h", "m", "s"
		// Convert "d" to "24h" for user convenience
		durationStr := olderThan
		if strings.HasSuffix(olderThan, "d") {
			days := strings.TrimSuffix(olderThan, "d")
			durationStr = days + "d" // This will fail, so we convert
			if daysNum, err := strconv.Atoi(days); err == nil {
				durationStr = fmt.Sprintf("%dh", daysNum*24)
			}
		}
		maxAge, err = time.ParseDuration(durationStr)
		if err != nil {
			log.Fatalf("invalid duration: %s (use format like 168h, 24h)", olderThan)
		}
	}

	watched, err := loadAllWatched()
	if err != nil {
		log.Fatalf("failed to read watched: %v", err)
	}

	if len(watched) == 0 {
		fmt.Println("no watched projects")
		return
	}

	// If no retention config and no --older-than, show current config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if maxAge == 0 && cfg.Retention.MaxAge == 0 && cfg.Retention.MaxRuns == 0 {
		fmt.Println("No retention policy configured. Set in ~/.anvil/config.yaml:")
		fmt.Println("  retention:")
		fmt.Println("    max_age: 168h")
		fmt.Println("    max_runs: 50")
		fmt.Println("")
		fmt.Println("Or use --older-than to prune manually:")
		fmt.Println("  anvil cleanup --older-than=72h")
		return
	}

	action := "Would prune"
	if dryRun {
		action = "Would prune"
	} else {
		action = "Pruned"
	}

	totalFreed := 0

	for _, w := range watched {
		// Prune logs
		logsDir := filepath.Join(w.Path, ".anvil", "logs")
		if _, err := os.Stat(logsDir); err == nil {
			count, freed := pruneDir(logsDir, maxAge, 0, dryRun)
			totalFreed += freed
			if count > 0 {
				fmt.Printf("%s %d log files from %s\n", action, count, w.Path)
			}
		}

		// Prune runs
		runsDir := filepath.Join(w.Path, ".anvil", "runs")
		if _, err := os.Stat(runsDir); err == nil {
			count, freed := pruneDir(runsDir, maxAge, cfg.Retention.MaxRuns, dryRun)
			totalFreed += freed
			if count > 0 {
				fmt.Printf("%s %d run files from %s\n", action, count, w.Path)
			}
		}
	}

	if dryRun {
		fmt.Printf("Total space that would be freed: %d bytes\n", totalFreed)
		fmt.Println("(use without --dry-run to actually delete)")
	} else {
		fmt.Printf("Total space freed: %d bytes\n", totalFreed)
	}
}

func pruneDir(dir string, maxAge time.Duration, maxRuns int, dryRun bool) (int, int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}

	// Collect task directories
	var taskDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			taskDirs = append(taskDirs, filepath.Join(dir, entry.Name()))
		}
	}

	deleted := 0
	freed := 0

	for _, taskDir := range taskDirs {
		taskEntries, err := os.ReadDir(taskDir)
		if err != nil {
			continue
		}

		// Collect files with modification times
		type fileInfo struct {
			name    string
			path    string
			size    int64
			modTime time.Time
		}

		var files []fileInfo
		for _, entry := range taskEntries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{
				name:    entry.Name(),
				path:    filepath.Join(taskDir, entry.Name()),
				size:    info.Size(),
				modTime: info.ModTime(),
			})
		}

		if len(files) == 0 {
			continue
		}

		// Sort by modification time (oldest first)
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime.Before(files[j].modTime)
		})

		now := time.Now()
		cutoff := now.Add(-maxAge)

		// Mark files to delete by age
		toDelete := make(map[string]fileInfo)
		if maxAge > 0 {
			for _, f := range files {
				if f.modTime.Before(cutoff) {
					toDelete[f.path] = f
				}
			}
		}

		// Mark files to delete by count (keep maxRuns newest)
		if maxRuns > 0 && len(files) > maxRuns {
			for i := 0; i < len(files)-maxRuns; i++ {
				f := files[i]
				toDelete[f.path] = f
			}
		}

		// Delete marked files
		for _, f := range toDelete {
			if !dryRun {
				if err := os.Remove(f.path); err == nil {
					deleted++
					freed += int(f.size)
				}
			} else {
				deleted++
				freed += int(f.size)
			}
		}
	}

	return deleted, freed
}

func psCmd(args []string) {
	jsonOutput := false
	watchMode := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOutput = true
		case "--watch", "-f":
			watchMode = true
		}
	}

	if watchMode {
		psWatch()
		return
	}

	if !daemon.IsDaemonRunning() {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("daemon not running")
		}
		return
	}

	// Show drain status header if applicable (text mode only)
	if !jsonOutput {
		if status, err := daemon.SendStatusRequest(); err == nil {
			if status.Draining {
				fmt.Println("(draining — no new tasks will be dispatched)")
			}
			// Show rate limit status if configured
			if status.Paused {
				fmt.Println("(PAUSED — no new tasks will be dispatched)")
			}
			if status.ThrottleRate > 0 {
				fmt.Printf("Throttle: %d tasks/min\n", status.ThrottleRate)
			}
			if len(status.PausedLabels) > 0 {
				fmt.Printf("Paused labels: %s\n", strings.Join(status.PausedLabels, ", "))
			}
			if status.RateLimited {
				slots := status.RateLimitSlots
				inUse := status.RateInUse
				pct := 0
				if slots > 0 {
					pct = (inUse * 100) / slots
				}
				bar := strings.Repeat("█", pct/5) + strings.Repeat("░", 20-pct/5)
				fmt.Printf("Rate limit: [%s] %d/%d\n", bar, inUse, slots)
			}
		}
	}

	tasks, err := daemon.SendPsRequest()
	if err != nil {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Printf("failed to get tasks: %v\n", err)
		}
		return
	}

	if len(tasks) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("no running tasks")
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

	// Print table header
	fmt.Printf("%-30s %-20s %-10s %-10s %-30s %s\n", "PROJECT", "TASK", "PID", "ELAPSED", "STATUS", "STARTED")
	fmt.Printf("%s\n", strings.Repeat("-", 120))

	// Print each task
	for _, t := range tasks {
		status := ""
		if t.Status != "" {
			status = t.Status
		}
		fmt.Printf("%-30s %-20s %-10d %-10s %-30s %s\n",
			truncate(t.Project, 30),
			truncate(t.Name, 20),
			t.PID,
			t.Elapsed,
			truncate(status, 30),
			t.Started)
	}
}

func psWatch() {
	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	// Start the new TUI dashboard
	if err := tui.StartDashboard(); err != nil {
		fmt.Printf("Error starting dashboard: %v\n", err)
	}
}

// rawLineWriter translates \n to \r\n for raw terminal mode where OPOST is disabled.
type rawLineWriter struct {
	w io.Writer
}

func (r *rawLineWriter) Write(p []byte) (n int, err error) {
	out := bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))
	_, err = r.w.Write(out)
	return len(p), err
}
