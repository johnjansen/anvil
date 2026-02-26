package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/internal/runner"
	"github.com/johnjansen/anvil/internal/updater"
	"github.com/johnjansen/anvil/internal/webhook"

	"gopkg.in/yaml.v3"
)

// version returns the current anvil version from build info.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// ErrDaemonAlreadyRunning is returned when a daemon is already running
var ErrDaemonAlreadyRunning = errors.New("daemon already running")

// checkAndWritePID checks for an existing daemon PID file and writes our PID.
// Returns ErrDaemonAlreadyRunning if another daemon is running.
func checkAndWritePID() error {
	pidPath := config.PidFile()

	// Check if PID file exists
	if _, err := os.Stat(pidPath); err == nil {
		// PID file exists, check if the process is still running
		data, readErr := os.ReadFile(pidPath)
		if readErr == nil {
			pidStr := strings.TrimSpace(string(data))
			pid, parseErr := strconv.Atoi(pidStr)
			if parseErr == nil {
				// Try to send signal 0 (check if process exists)
				proc, err := os.FindProcess(pid)
				if err == nil {
					// Signal 0 checks if process exists without sending a signal
					if err := proc.Signal(syscall.Signal(0)); err == nil {
						// Process is running
						return fmt.Errorf("%w (PID %d)", ErrDaemonAlreadyRunning, pid)
					}
					// Process not found or dead, continue to cleanup
				}
			}
		}
		// Stale PID file, remove it
		os.Remove(pidPath)
	}

	// Write our PID file
	return os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
}

// removePIDFile removes the daemon PID file
func removePIDFile() {
	os.Remove(config.PidFile())
}

// workItem is a single unit of work dispatched to the worker pool.
type workItem struct {
	project *project.Project
	todo    project.Todo
}

type Daemon struct {
	config      *config.Config
	runner      *runner.Runner
	workQueue   chan workItem
	inFlight    map[string]int // taskKey -> count; incremented when queued, decremented when done
	inFlightMu  sync.Mutex
	stop        chan struct{}
	stopOnce    sync.Once
	done        chan struct{}
	reload      chan struct{} // SIGHUP trigger for config reload
	lastTick    time.Time // last minute we processed (truncated to minute)
	lastTickMu  sync.Mutex
	startedAt   time.Time // when the daemon started
	socketPath  string
	tasks       map[string]*RunningTask
	tasksMu     sync.RWMutex
	httpServer  *http.Server
	draining    int32           // atomic: 1 = stop-on-idle mode active
	gracefulStop int32          // atomic: 1 = graceful shutdown in progress
	drainedTasks map[string]bool // taskID -> true: per-task stop-on-idle
	drainedMu   sync.Mutex
	// persistentFailures tracks consecutive failure counts for persistent tasks
	// to implement exponential backoff and hard-stop after max failures
	persistentFailures   map[string]int
	persistentFailuresMu sync.Mutex
	// persistentCooldowns tracks when a persistent task can next be dispatched.
	// Map value is the time when the cooldown expires.
	persistentCooldowns map[string]time.Time
	persistentCooldownsMu sync.Mutex
	// starvationTrackers tracks when persistent tasks started waiting for a worker slot.
	// Used to implement starvation prevention - after N minutes, persistent tasks yield to higher priority work.
	starvationTrackers map[string]time.Time
	starvationTrackersMu sync.Mutex
	// runnerCooldowns tracks runner indices that are temporarily skipped due to failures.
	// Map value is the time when the cooldown expires.
	runnerCooldowns map[int]time.Time
	runnerCooldownsMu sync.Mutex
	// pendingTasks tracks tasks that were due but not dispatched, with skip reasons.
	// Used by CLI to show queue status.
	pendingTasks  map[string]string // taskKey -> skip reason
	pendingTasksMu sync.RWMutex
	// stoppedTasks tracks persistent tasks that have been explicitly stopped.
	// Stopped tasks are not re-dispatched until started again via /start.
	stoppedTasks map[string]bool // taskID -> true
	stoppedMu    sync.Mutex
	// persistentBudgetUsed tracks cumulative wall-clock time consumed by each
	// persistent task during this daemon lifetime.  Resets on daemon restart.
	persistentBudgetUsed   map[string]time.Duration
	persistentBudgetUsedMu sync.Mutex
	webhooks *webhook.Sender
	// Metrics counters for Prometheus endpoint
	metricsSuccessCount int64 // atomic: total successful task runs
	metricsFailureCount int64 // atomic: total failed task runs
	// Histogram buckets for task duration (60s, 300s, 900s, +Inf)
	metricsDurBucket60   int64 // atomic: tasks completing in ≤60s
	metricsDurBucket300  int64 // atomic: tasks completing in ≤300s
	metricsDurBucket900  int64 // atomic: tasks completing in ≤900s
	metricsDurBucketInf  int64 // atomic: all completed tasks (≤+Inf)
	metricsDurSum        int64 // atomic: sum of all durations in milliseconds
}

type RunningTask struct {
	Project   string
	Name      string
	TaskID    string
	PID       int
	Started   time.Time
	Timeout   time.Duration // task-specific timeout (0 = use global)
	Cancel    context.CancelFunc
	LogPath   string
	SessionID string
	Status    string // dynamic status reported by task via ##anvil:status
}

type KillRequest struct {
	ID string `json:"id"`
}

type TaskInfo struct {
	Project     string `json:"project"`
	Name        string `json:"name"`
	PID         int    `json:"pid"`
	Started     string `json:"started"`
	Elapsed     string `json:"elapsed"`
	Timeout     string `json:"timeout,omitempty"`
	TimeRemaining string `json:"time_remaining,omitempty"`
	PercentUsed float64 `json:"percent_used,omitempty"`
	LogPath     string `json:"log_path,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Status      string `json:"status,omitempty"`
}

// TaskQueueInfo holds information about a task in the queue or its last skip reason.
type TaskQueueInfo struct {
	Project    string `json:"project"`
	Name       string `json:"name"`
	Priority   int    `json:"priority"`
	Schedule   string `json:"schedule"`
	Status     string `json:"status"`       // "running", "pending", "skipped"
	SkipReason string `json:"skip_reason,omitempty"` // why task was skipped in last tick
}

// TaskBudgetInfo holds budget consumption info for a persistent task.
type TaskBudgetInfo struct {
	Project    string  `json:"project"`
	Name       string  `json:"name"`
	Budget     string  `json:"budget"`
	Used       string  `json:"used"`
	Remaining  string  `json:"remaining"`
	PercentUsed float64 `json:"percent_used"`
	Exhausted  bool    `json:"exhausted"`
}

func New(cfg *config.Config) *Daemon {
	poolSize := cfg.MaxWorkers
	if poolSize < 1 {
		poolSize = 1
	}
	return &Daemon{
		config:       cfg,
		runner:       runner.New(cfg.Runners, cfg.Timeout),
		workQueue:    make(chan workItem, poolSize*4),
		inFlight:     make(map[string]int),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
		reload:       make(chan struct{}, 1),
		socketPath:   filepath.Join(config.Dir(), "daemon.sock"),
		tasks:        make(map[string]*RunningTask),
		drainedTasks: make(map[string]bool),
		persistentFailures: make(map[string]int),
		persistentCooldowns: make(map[string]time.Time),
	starvationTrackers: make(map[string]time.Time),
		runnerCooldowns: make(map[int]time.Time),
		pendingTasks:  make(map[string]string),
		stoppedTasks:         make(map[string]bool),
		persistentBudgetUsed: make(map[string]time.Duration),
		webhooks:             webhook.New(cfg.Webhooks),
	}
}

func (d *Daemon) Run() {
	defer close(d.done)

	// Record start time for health checks
	d.startedAt = time.Now()

	// Write PID file on startup
	if err := checkAndWritePID(); err != nil {
		if errors.Is(err, ErrDaemonAlreadyRunning) {
			dlog.Fatal("%s", err.Error())
		} else {
			dlog.Fatal("failed to write PID file: %v", err)
		}
		return
	}
	// Clean up PID file on shutdown
	defer removePIDFile()

	// Set up SIGHUP handler for config reload
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)
	go func() {
		for range sigChan {
			select {
			case d.reload <- struct{}{}:
			default:
				// reload channel already has a pending signal
			}
		}
	}()

	poolSize := d.config.MaxWorkers
	if poolSize < 1 {
		poolSize = 1
	}

	ticker := time.NewTicker(d.config.TickInterval)
	defer ticker.Stop()

	dlog.Startup(d.config.TickInterval.String(), strings.Join(d.config.Runners, ", "), poolSize)

	// Check for updates on startup if auto_update is enabled
	if d.config.AutoUpdate {
		currentVersion := version()
		latestVersion, err := updater.CheckLatest()
		if err != nil {
			dlog.Warn("auto-update check failed: %v", err)
		} else if updater.NeedsUpdate(currentVersion, latestVersion) {
			dlog.Info("auto-update available: %s -> %s", currentVersion, latestVersion)
			result := updater.Apply(currentVersion, latestVersion)
			if result.Error != nil {
				dlog.Warn("auto-update failed: %v, continuing with current version", result.Error)
			} else {
				dlog.Info("auto-update applied: %s -> %s, restart to use new version", currentVersion, latestVersion)
			}
		} else {
			dlog.Info("auto-update: already on latest version %s", latestVersion)
		}
	}

	// Start socket server
	go d.startSocketServer()

	// Start SIGHUP handler for config reload
	sighupChan := make(chan os.Signal, 1)
	signal.Notify(sighupChan, syscall.SIGHUP)
	defer signal.Stop(sighupChan)

	// Start worker pool
	var workerWg sync.WaitGroup
	for i := 0; i < poolSize; i++ {
		workerWg.Add(1)
		go func(id int) {
			defer workerWg.Done()
			d.worker(id)
		}(i)
	}

	// Set up SIGTERM/SIGINT handler for graceful shutdown
	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(termChan)

	for {
		select {
		case <-d.stop:
			dlog.Stopping()
			if d.httpServer != nil {
				d.httpServer.Shutdown(context.Background())
			}
			close(d.workQueue) // signals workers to drain and exit
			workerWg.Wait()
			os.Remove(d.socketPath)
			return
		case <-termChan:
			d.gracefulShutdown(&workerWg)
			os.Remove(d.socketPath)
			return
		case <-sighupChan:
			d.reloadConfig()
		case <-d.reload:
			d.reloadConfig()
		case now := <-ticker.C:
			d.tick(now)
		}
	}
}

func (d *Daemon) Stop() {
	d.stopOnce.Do(func() {
		close(d.stop)
	})
	<-d.done
}

// Done returns a channel that is closed when the daemon has fully stopped.
func (d *Daemon) Done() <-chan struct{} {
	return d.done
}

// gracefulShutdown waits for running tasks to complete before stopping.
// It sets the draining flag to prevent new task dispatches, then waits up to
// the configured timeout for running tasks to finish.
func (d *Daemon) gracefulShutdown(workerWg *sync.WaitGroup) {
	atomic.StoreInt32(&d.gracefulStop, 1)
	atomic.StoreInt32(&d.draining, 1) // prevent new task dispatches

	timeout := d.config.GracefulShutdownTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	d.tasksMu.RLock()
	taskCount := len(d.tasks)
	d.tasksMu.RUnlock()

	if taskCount == 0 {
		dlog.Info("graceful shutdown: no running tasks, stopping immediately")
	} else {
		dlog.Info("graceful shutdown: waiting for %d running task(s) to complete (timeout: %s)", taskCount, timeout)
		deadline := time.After(timeout)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

	waitLoop:
		for {
			select {
			case <-deadline:
				d.tasksMu.RLock()
				remaining := len(d.tasks)
				d.tasksMu.RUnlock()
				dlog.Warn("graceful shutdown timeout reached, %d task(s) still running — force stopping", remaining)
				break waitLoop
			case <-ticker.C:
				d.tasksMu.RLock()
				remaining := len(d.tasks)
				d.tasksMu.RUnlock()
				if remaining == 0 {
					dlog.Info("graceful shutdown: all tasks completed")
					break waitLoop
				}
			}
		}
	}

	dlog.Stopping()
	if d.httpServer != nil {
		d.httpServer.Shutdown(context.Background())
	}
	close(d.workQueue)
	workerWg.Wait()
	d.stopOnce.Do(func() {
		close(d.stop)
	})
}

// reloadConfig reads the config file and updates the daemon's configuration.
// It logs what changed and applies updates to max_workers, timeout, runners, and tick_interval.
// Running tasks are not affected - only new task dispatches use the updated config.
func (d *Daemon) reloadConfig() {
	newConfig, err := config.Load()
	if err != nil {
		dlog.Warn("failed to reload config: %v", err)
		return
	}

	var changes []string

	// max_workers: can grow or shrink
	if newConfig.MaxWorkers != d.config.MaxWorkers {
		oldVal := d.config.MaxWorkers
		d.config.MaxWorkers = newConfig.MaxWorkers
		changes = append(changes, fmt.Sprintf("max_workers %d->%d", oldVal, newConfig.MaxWorkers))
	}

	// timeout: apply to new tasks
	if newConfig.Timeout != d.config.Timeout {
		oldVal := d.config.Timeout
		d.config.Timeout = newConfig.Timeout
		changes = append(changes, fmt.Sprintf("timeout %v->%v", oldVal, newConfig.Timeout))
	}

	// runners: apply to new tasks
	if len(newConfig.Runners) > 0 {
		oldRunners := strings.Join(d.config.Runners, ", ")
		newRunners := strings.Join(newConfig.Runners, ", ")
		if oldRunners != newRunners {
			d.config.Runners = newConfig.Runners
			changes = append(changes, fmt.Sprintf("runners %s->%s", oldRunners, newRunners))
		}
	}

	// tick_interval: will be picked up on next tick
	if newConfig.TickInterval != d.config.TickInterval {
		oldVal := d.config.TickInterval
		d.config.TickInterval = newConfig.TickInterval
		changes = append(changes, fmt.Sprintf("tick_interval %v->%v", oldVal, newConfig.TickInterval))
	}

	// webhooks: update sender config
	d.config.Webhooks = newConfig.Webhooks
	if d.webhooks != nil {
		d.webhooks.UpdateConfig(newConfig.Webhooks)
	} else if len(newConfig.Webhooks) > 0 {
		d.webhooks = webhook.New(newConfig.Webhooks)
		changes = append(changes, "webhooks configured")
	}

	if len(changes) > 0 {
		dlog.Info("config reloaded: %s", strings.Join(changes, ", "))
	} else {
		dlog.Info("config reloaded: no changes")
	}
}

// worker pulls work items from the queue and executes them one at a time.
func (d *Daemon) worker(id int) {
	dlog.WorkerStarted(id)
	for item := range d.workQueue {
		projName := filepath.Base(item.project.Path)
		dlog.WorkerPickup(id, projName, item.todo.Name, item.todo.Priority)
		d.runTask(id, item.project, item.todo)
		dlog.WorkerIdle(id)
	}
	dlog.WorkerStopped(id)
}

// mergeEnv merges global and task-specific environment variable maps.
// Task env overrides global env for the same key. Returns nil if both are empty.
func mergeEnv(global, task map[string]string) map[string]string {
	if len(global) == 0 && len(task) == 0 {
		return nil
	}
	merged := make(map[string]string, len(global)+len(task))
	for k, v := range global {
		merged[k] = v
	}
	for k, v := range task {
		merged[k] = v
	}
	return merged
}

// checkDependenciesMet verifies all dependencies have completed successfully in the current cycle.
// Returns (true, "") if all dependencies are met, (false, reason) if not.
func checkDependenciesMet(projectPath string, dependsOn []string) (met bool, reason string) {
	for _, dep := range dependsOn {
		// dep is the task name (e.g., "fetch-data.md")
		// We need to find the task ID from the project todos directory
		runRecord, err := project.ReadCurrentRunRecord(projectPath, dep)
		if err != nil {
			// No run record found - dependency hasn't run yet
			return false, "dependency not run: " + dep
		}
		if !runRecord.Success {
			return false, "dependency failed: " + dep
		}
		// Check if the run finished in this daemon cycle (within last ~1 minute)
		// This ensures dependencies from previous cycles are re-run
		elapsed := time.Since(runRecord.Finished)
		if elapsed > 2*time.Minute {
			// Dependency completed more than 2 minutes ago, re-run to ensure fresh data
			return false, "dependency stale: " + dep
		}
	}
	return true, ""
}

// runTask executes a single todo task and handles all bookkeeping.
func (d *Daemon) runTask(workerID int, proj *project.Project, t project.Todo) {
	taskKey := fmt.Sprintf("%s/%s", proj.Path, t.Name)

	// Drain completion check: registered first so it runs LAST (LIFO).
	// After all other defers clean up tasks/inFlight maps, check whether
	// we are draining and everything has finished — if so, trigger stop.
	defer func() {
		if atomic.LoadInt32(&d.draining) == 1 {
			d.tasksMu.RLock()
			tasksRunning := len(d.tasks)
			d.tasksMu.RUnlock()
			d.inFlightMu.Lock()
			queued := len(d.inFlight)
			d.inFlightMu.Unlock()
			if tasksRunning == 0 && queued == 0 {
				dlog.Info("drain complete — all tasks finished, stopping daemon")
				go d.Stop()
			}
		}
	}()

	// Decrement in-flight count when the task completes; remove key at zero
	defer func() {
		d.inFlightMu.Lock()
		d.inFlight[taskKey]--
		if d.inFlight[taskKey] <= 0 {
			delete(d.inFlight, taskKey)
		}
		d.inFlightMu.Unlock()
	}()

	// Use task-specific timeout if set, otherwise fall back to global config
	// For persistent tasks, use PersistentMaxRuntime if set (forces cycle after max runtime)
	// Default persistent max runtime is 4 hours to prevent unbounded context growth
	timeout := d.config.Timeout
	if t.IsPersistent() {
		if t.PersistentMaxRuntime > 0 {
			timeout = t.PersistentMaxRuntime
		} else {
			timeout = 4 * time.Hour // default: force-cycle every 4 hours
		}
	} else if t.Timeout > 0 {
		timeout = t.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// For persistent tasks, log a warning at 80% of max runtime to signal upcoming cycle
	if t.IsPersistent() {
		warningAt := time.Duration(float64(timeout) * 0.8)
		taskLabel := filepath.Base(proj.Path) + "/" + t.Name
		warningTimer := time.AfterFunc(warningAt, func() {
			remaining := timeout - warningAt
			dlog.Warn("persistent task %s approaching max runtime — cycling in %v", taskLabel, remaining.Round(time.Second))
		})
		defer warningTimer.Stop()
	}

	runID := newRunID()
	startTime := time.Now()

	// Track the running task
	d.tasksMu.Lock()
	d.tasks[taskKey] = &RunningTask{
		Project: proj.Path,
		Name:    t.Name,
		TaskID:  t.ID,
		PID:     os.Getpid(),
		Started: startTime,
		Timeout: timeout,
		Cancel:  cancel,
	}
	d.tasksMu.Unlock()

	defer func() {
		d.tasksMu.Lock()
		delete(d.tasks, taskKey)
		d.tasksMu.Unlock()
	}()

	// Determine resume behavior:
	// - Explicit frontmatter resume: true/false takes priority
	// - Default: recurring tasks resume (use latest session), one-shots don't
	var resume bool
	if t.Resume != nil {
		resume = *t.Resume
	} else {
		resume = t.Schedule != ""
	}

	// Get the session ID to resume (if any)
	var sessionToResume string
	if resume {
		if latestSession, err := project.LatestSessionID(proj.Path, t.ID); err == nil {
			sessionToResume = latestSession
		} else {
			// No prior run record — start a fresh session instead of resuming
			resume = false
		}
	}

	// Pre-check: if set, run a shell guard and skip silently on non-zero exit.
	// This lets recurring tasks avoid expensive agent invocations when there
	// is nothing to do (e.g. no open issues), eliminating idle noise.
	if t.PreCheck != "" {
		precheckCmd := exec.CommandContext(ctx, "sh", "-c", t.PreCheck)
		precheckCmd.Dir = proj.Path
		if precheckErr := precheckCmd.Run(); precheckErr != nil {
			taskLabel := filepath.Base(proj.Path) + "/" + t.Name
			// Distinguish between check ran and returned non-zero vs failed to start
			if precheckCmd.ProcessState != nil {
				// Command ran but exited non-zero — expected skip case
				dlog.Info("pre_check skipped %s (exit %d)", taskLabel, precheckCmd.ProcessState.ExitCode())
			} else {
				// Command failed to start (binary not found, permission denied, etc.)
				dlog.Warn("pre_check failed for %s: %v", taskLabel, precheckErr)
			}
			return
		}
	}

	// Run the task with retry support
	projName := filepath.Base(proj.Path)
	taskLabel := projName + "/" + t.Name
	var childPID int
	logDir := filepath.Join(proj.Path, ".anvil", "logs", t.ID)

	// Build skip indices from unexpired runner cooldowns
	skipIndices := make(map[int]bool)
	d.runnerCooldownsMu.Lock()
	now := time.Now()
	for idx, expires := range d.runnerCooldowns {
		if now.Before(expires) {
			skipIndices[idx] = true
		} else {
			// Cooldown expired, remove it
			delete(d.runnerCooldowns, idx)
		}
	}
	d.runnerCooldownsMu.Unlock()

	// Determine default retry delay if not set
	retryDelay := t.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 1 * time.Minute // default 1 minute
	}

	// For one-shot tasks:
	// - With retry: Write lock file only after all retries are exhausted (final failure)
	//   This allows retries to happen without the lock blocking re-dispatch
	// - Without retry: Write lock file before execution for crash protection
	var finalFailureLockPath string
	if t.Schedule == "" {
		if t.Retry > 0 {
			// With retry: will be written on final failure (after loop)
			finalFailureLockPath = t.Path + ".lock"
		} else {
			// No retry: write lock file before execution for crash protection
			lockPath := t.Path + ".lock"
			if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0600); err != nil {
				dlog.Warn("could not write lock file %s: %v", lockPath, err)
			} else {
				defer os.Remove(lockPath)
			}
		}
	}

	// Retry loop with exponential backoff
	var usedSessionID string
	var logPath string
	var usedRunnerIdx int
	var stderrOutput string
	var err error
	var finalAttempt int // tracks which attempt we ended on (0-based)

	// Track checkpoint data emitted by the task (last one wins)
	var checkpointMu sync.Mutex
	var lastCheckpointData string

	for attempt := 0; ; attempt++ {
		finalAttempt = attempt
		// Check if context is already cancelled before attempting
		if ctx.Err() != nil {
			err = ctx.Err()
			break
		}

		// Merge global config env with task-specific env (task overrides global)
		mergedEnv := mergeEnv(d.config.Env, t.Env)

		// If checkpoint is enabled, inject the latest checkpoint data as an env var
		if t.Checkpoint {
			cpData := project.LatestCheckpointData(proj.Path, t.ID)
			if cpData != "" {
				if mergedEnv == nil {
					mergedEnv = make(map[string]string)
				}
				mergedEnv["ANVIL_CHECKPOINT_DATA"] = cpData
			}
		}

		usedSessionID, logPath, usedRunnerIdx, stderrOutput, err = d.runner.Run(ctx, proj.Path, sessionToResume, resume, t.SkipPermissions, t.AllowedTools, t.Content, taskLabel, logDir, skipIndices, mergedEnv, func(pid int, lp string, sid string) {
			childPID = pid
			d.tasksMu.Lock()
			if task, ok := d.tasks[taskKey]; ok {
				task.PID = pid
				task.LogPath = lp
				task.SessionID = sid
			}
			d.tasksMu.Unlock()
			// Fire task_start webhook
			d.webhooks.Fire(webhook.EventStart, webhook.BuildPayload(t.Name, proj.Path, runID, startTime, time.Time{}, 0, ""))
			if t.Webhook != "" {
				d.webhooks.FireURL(t.Webhook, webhook.EventStart, webhook.BuildPayload(t.Name, proj.Path, runID, startTime, time.Time{}, 0, ""))
			}
		}, func(status string) {
			d.tasksMu.Lock()
			if task, ok := d.tasks[taskKey]; ok {
				task.Status = status
			}
			d.tasksMu.Unlock()
		}, func(data string) {
			if t.Checkpoint {
				checkpointMu.Lock()
				lastCheckpointData = data
				checkpointMu.Unlock()
			}
		})

		// Success - exit retry loop
		if err == nil {
			break
		}

		// Failure - check if we should retry
		retriesRemaining := t.Retry - attempt
		if retriesRemaining <= 0 {
			// No more retries, exit loop with error
			break
		}

		// Calculate exponential backoff: base * 2^attempt
		backoffDuration := retryDelay
		for i := 0; i < attempt; i++ {
			backoffDuration *= 2
		}

		dlog.Info("retry %d/%d for %s after %v (backoff %v)", attempt+1, t.Retry, taskLabel, err, backoffDuration)

		// Wait for backoff duration, but respect context cancellation
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break
		case <-time.After(backoffDuration):
			// Continue to next retry attempt
		}
	}

	// For one-shot tasks with retry: write lock file on final failure
	// (after all retries exhausted)
	if t.Schedule == "" && t.Retry > 0 && err != nil && finalFailureLockPath != "" {
		if err := os.WriteFile(finalFailureLockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0600); err != nil {
			dlog.Warn("could not write lock file %s: %v", finalFailureLockPath, err)
		} else {
			defer os.Remove(finalFailureLockPath)
		}
	}

	// For one-shot tasks with retry: write lock file on final failure
	// (after all retries exhausted)
	if t.Schedule == "" && t.Retry > 0 && err != nil && finalFailureLockPath != "" {
		if err := os.WriteFile(finalFailureLockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0600); err != nil {
			dlog.Warn("could not write lock file %s: %v", finalFailureLockPath, err)
		} else {
			defer os.Remove(finalFailureLockPath)
		}
	}

	// Parse token usage from runner stderr
	tokenUsage := runner.ParseTokenUsage(stderrOutput)

	// Calculate estimated cost using configured or default rates
	inputRate := d.config.InputTokenRate
	outputRate := d.config.OutputTokenRate
	if inputRate <= 0 {
		inputRate = 3.0 // $3.00 per 1M input tokens (Sonnet default)
	}
	if outputRate <= 0 {
		outputRate = 15.0 // $15.00 per 1M output tokens (Sonnet default)
	}
	estimatedCost := float64(tokenUsage.InputTokens)/1_000_000*inputRate +
		float64(tokenUsage.OutputTokens)/1_000_000*outputRate

	// Write run record after completion with outcome data
	checkpointMu.Lock()
	cpData := lastCheckpointData
	checkpointMu.Unlock()

	runRecord := project.RunRecord{
		RunID:            runID,
		TaskID:           t.ID,
		SessionID:        usedSessionID,
		PID:              childPID,
		Started:          startTime,
		Finished:         time.Now(),
		Success:          err == nil,
		InputTokens:      tokenUsage.InputTokens,
		OutputTokens:     tokenUsage.OutputTokens,
		EstimatedCostUSD: estimatedCost,
		CheckpointData:   cpData,
		Attempt:          finalAttempt + 1, // convert 0-based to 1-based
		MaxRetries:       t.Retry,
		RetryDelay:       retryDelay.String(),
	}
	if err != nil {
		runRecord.Error = err.Error()
	}

	// Capture output summary from the runner log file
	if logPath != "" {
		if summary, sumErr := captureOutputSummary(logPath); sumErr == nil {
			runRecord.OutputSummary = summary
		}
	}

	// Truncate oversized log files to keep disk usage bounded.
	// Uses per-task max_log_size if set, otherwise falls back to global retention config.
	if logPath != "" {
		maxSize := t.MaxLogSize
		if maxSize <= 0 {
			maxSize = d.config.Retention.MaxLogSize
		}
		if maxSize > 0 {
			truncateLog(logPath, maxSize)
		}
	}

	elapsed := time.Since(startTime)
	// Detect force-cycle: persistent task hit its max runtime (context deadline exceeded)
	forceCycled := t.IsPersistent() && err != nil && ctx.Err() == context.DeadlineExceeded
	whPayload := webhook.BuildPayload(t.Name, proj.Path, runID, startTime, runRecord.Finished, runRecord.EstimatedCostUSD, runRecord.Error)
	if forceCycled {
		// Force-cycle is a normal lifecycle event, not a failure.
		// Log it clearly and skip failure backoff/runner cooldown.
		dlog.Info("persistent task %s force-cycled after %v — will restart on next tick", t.Name, elapsed.Round(time.Second))
		// Clear failure count since force-cycle is not a real failure
		d.persistentFailuresMu.Lock()
		delete(d.persistentFailures, taskKey)
		d.persistentFailuresMu.Unlock()
		// Mark success in run record since this is expected behavior
		runRecord.Success = true
		runRecord.Error = ""
		d.webhooks.Fire(webhook.EventPersistentCycle, whPayload)
		if t.Webhook != "" {
			d.webhooks.FireURL(t.Webhook, webhook.EventPersistentCycle, whPayload)
		}
	} else if err != nil {
		atomic.AddInt64(&d.metricsFailureCount, 1)
		d.recordDurationMetric(elapsed)
		dlog.WorkerFail(workerID, projName, t.Name, err)
		// If the runner failed, set a 5-minute cooldown to avoid retrying it immediately
		if usedRunnerIdx >= 0 {
			d.runnerCooldownsMu.Lock()
			d.runnerCooldowns[usedRunnerIdx] = time.Now().Add(5 * time.Minute)
			d.runnerCooldownsMu.Unlock()
			dlog.Info("runner[%d] failed — marking as cooldown for 5 minutes", usedRunnerIdx)
		}
		// Run on_failure hook if defined
		if t.OnFailure != "" {
			d.runHook("on_failure", t.OnFailure, proj.Path, t, logPath, usedSessionID, startTime, elapsed, finalAttempt+1, t.Retry, retryDelay)
		}
		// Fire failure webhook (use timeout event if deadline exceeded)
		whEvent := webhook.EventFailure
		if ctx.Err() == context.DeadlineExceeded {
			whEvent = webhook.EventTimeout
		}
		d.webhooks.Fire(whEvent, whPayload)
		if t.Webhook != "" {
			d.webhooks.FireURL(t.Webhook, whEvent, whPayload)
		}
		// For persistent tasks, track failures and apply exponential backoff
		if t.IsPersistent() {
			d.persistentFailuresMu.Lock()
			d.persistentFailures[taskKey]++
			failCount := d.persistentFailures[taskKey]
			d.persistentFailuresMu.Unlock()
			// Calculate exponential backoff: base * 2^(failCount-1), default base is 1 minute
			baseBackoff := t.RetryDelay
			if baseBackoff <= 0 {
				baseBackoff = 1 * time.Minute
			}
			backoffDuration := baseBackoff
			for i := 1; i < failCount; i++ {
				backoffDuration *= 2
			}
			d.persistentCooldownsMu.Lock()
			d.persistentCooldowns[taskKey] = time.Now().Add(backoffDuration)
			d.persistentCooldownsMu.Unlock()
			dlog.Info("persistent task %s failed (attempt %d) — backing off for %v", t.Name, failCount, backoffDuration)
		}
	} else {
		atomic.AddInt64(&d.metricsSuccessCount, 1)
		d.recordDurationMetric(elapsed)
		dlog.WorkerDone(workerID, projName, t.Name, elapsed)
		// Run on_success hook if defined
		if t.OnSuccess != "" {
			d.runHook("on_success", t.OnSuccess, proj.Path, t, logPath, usedSessionID, startTime, elapsed, finalAttempt+1, t.Retry, retryDelay)
		}
		d.webhooks.Fire(webhook.EventSuccess, whPayload)
		if t.Webhook != "" {
			d.webhooks.FireURL(t.Webhook, webhook.EventSuccess, whPayload)
		}
		// Remove the todo file after successful execution (one-shot only)
		if t.Schedule == "" {
			if removeErr := os.Remove(t.Path); removeErr != nil {
				dlog.Warn("could not remove %s: %v", t.Path, removeErr)
			}
		}
		// Clear failure count on successful completion for persistent tasks
		if t.IsPersistent() {
			d.persistentFailuresMu.Lock()
			delete(d.persistentFailures, taskKey)
			d.persistentFailuresMu.Unlock()
		}
	}

	// For persistent tasks, accumulate budget usage
	if t.IsPersistent() {
		d.persistentBudgetUsedMu.Lock()
		d.persistentBudgetUsed[taskKey] += elapsed
		budgetUsed := d.persistentBudgetUsed[taskKey]
		d.persistentBudgetUsedMu.Unlock()
		// Emit warning when budget is below 20%
		if t.PersistentBudget > 0 {
			remaining := t.PersistentBudget - budgetUsed
			pctUsed := float64(budgetUsed) / float64(t.PersistentBudget) * 100
			if pctUsed >= 80 && remaining > 0 {
				dlog.Warn("##anvil:status Budget low for %s: %v remaining (%.0f%% of %v used)", t.Name, remaining.Round(time.Second), pctUsed, t.PersistentBudget)
			}
		}
	}

	// For persistent tasks, set cooldown after each cycle completes
	if t.IsPersistent() && t.PersistentCooldown > 0 {
		d.persistentCooldownsMu.Lock()
		d.persistentCooldowns[taskKey] = time.Now().Add(t.PersistentCooldown)
		d.persistentCooldownsMu.Unlock()
		// Clear starvation tracker - task completed successfully
		d.starvationTrackersMu.Lock()
		delete(d.starvationTrackers, taskKey)
		d.starvationTrackersMu.Unlock()
		dlog.Info("persistent task %s completed — next run in %v", t.Name, t.PersistentCooldown)
	}

	if writeErr := project.WriteRunRecord(proj.Path, runRecord); writeErr != nil {
		dlog.Warn("failed to write run record for %s: %v", t.Name, writeErr)
	}
}

// runHook executes a lifecycle hook (on_success or on_failure) as a shell command.
// Hook errors are logged as warnings but do not affect the task outcome.
func (d *Daemon) runHook(hookName, command, projectPath string, t project.Todo, logPath, sessionID string, startTime time.Time, elapsed time.Duration, attempt, maxRetries int, retryDelay time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	hookCmd := exec.CommandContext(ctx, "sh", "-c", command)
	hookCmd.Dir = projectPath

	exitCode := "0"
	if hookName == "on_failure" {
		exitCode = "1"
	}

	// Determine if the task will retry after this failure
	willRetry := "false"
	if hookName == "on_failure" && maxRetries > 0 && attempt < maxRetries {
		willRetry = "true"
	}

	hookCmd.Env = append(os.Environ(),
		"ANVIL_TASK_NAME="+t.Name,
		"ANVIL_EXIT_CODE="+exitCode,
		"ANVIL_LOG_PATH="+logPath,
		"ANVIL_PROJECT="+projectPath,
		"ANVIL_SESSION_ID="+sessionID,
		"ANVIL_START_TIME="+startTime.Format(time.RFC3339),
		"ANVIL_END_TIME="+time.Now().Format(time.RFC3339),
		fmt.Sprintf("ANVIL_ELAPSED_MS=%d", elapsed.Milliseconds()),
		fmt.Sprintf("ANVIL_RETRY_ATTEMPT=%d", attempt),
		fmt.Sprintf("ANVIL_RETRY_MAX=%d", maxRetries),
		"ANVIL_RETRY_DELAY="+retryDelay.String(),
		"ANVIL_WILL_RETRY="+willRetry,
	)

	if hookErr := hookCmd.Run(); hookErr != nil {
		dlog.Warn("%s hook failed for %s: %v", hookName, t.Name, hookErr)
	} else {
		dlog.Info("%s hook completed for %s", hookName, t.Name)
	}
}

// captureOutputSummary reads the first and last N lines of a log file.
// Returns a summary string like "first 3 lines...\n...\nlast 3 lines".
func captureOutputSummary(logPath string) (string, error) {
	const maxLines = 3
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) <= maxLines*2 {
		return string(data), nil
	}
	first := strings.Join(lines[:maxLines], "\n")
	last := strings.Join(lines[len(lines)-maxLines:], "\n")
	return first + "\n...\n" + last, nil
}

// truncateLog keeps only the last maxSize bytes of a log file.
// If the file is smaller than maxSize, it is left unchanged.
// A marker line is prepended to indicate truncation occurred.
func truncateLog(logPath string, maxSize int64) {
	info, err := os.Stat(logPath)
	if err != nil || info.Size() <= maxSize {
		return
	}

	f, err := os.Open(logPath)
	if err != nil {
		return
	}

	// Seek to keep the last maxSize bytes (minus room for the marker)
	marker := []byte("\n--- [log truncated: exceeded max_log_size] ---\n")
	keepSize := maxSize - int64(len(marker))
	if keepSize < 0 {
		keepSize = 0
	}

	offset := info.Size() - keepSize
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		f.Close()
		return
	}

	tail, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return
	}

	// Skip to the next newline to avoid a partial first line
	if idx := bytes.IndexByte(tail, '\n'); idx >= 0 && idx < len(tail)-1 {
		tail = tail[idx+1:]
	}

	// Write truncated content back
	if err := os.WriteFile(logPath, append(marker, tail...), 0644); err != nil {
		dlog.Warn("failed to truncate log %s: %v", logPath, err)
	}
}

func (d *Daemon) startSocketServer() {
	// Remove any existing socket file
	os.Remove(d.socketPath)

	// Create unix socket
	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		dlog.SocketStartFailed(err)
		return
	}
	defer listener.Close()

	// Set socket permissions for read/write by user
	os.Chmod(d.socketPath, 0600)

	mux := http.NewServeMux()
	mux.HandleFunc("/ps", d.handlePs)
	mux.HandleFunc("/kill", d.handleKill)
	mux.HandleFunc("/drain", d.handleDrain)
	mux.HandleFunc("/drain/task", d.handleDrainTask)
	mux.HandleFunc("/run", d.handleRun)
	mux.HandleFunc("/status", d.handleStatus)
	mux.HandleFunc("/health", d.handleHealth)
	mux.HandleFunc("/metrics", d.handleMetrics)
	mux.HandleFunc("/timeout", d.handleTimeout)
	mux.HandleFunc("/queue", d.handleQueue)
	mux.HandleFunc("/reload", d.handleReload)
	mux.HandleFunc("/stop", d.handleStopTask)
	mux.HandleFunc("/start", d.handleStartTask)
	mux.HandleFunc("/budget", d.handleBudget)
	mux.HandleFunc("/reset-budget", d.handleResetBudget)

	d.httpServer = &http.Server{
		Handler: mux,
	}

	dlog.SocketListening(d.socketPath)
	if err := d.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		dlog.SocketError(err)
	}
}

func (d *Daemon) handlePs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.tasksMu.RLock()
	tasks := make([]*RunningTask, 0, len(d.tasks))
	for _, task := range d.tasks {
		tasks = append(tasks, task)
	}
	d.tasksMu.RUnlock()

	var result []TaskInfo
	now := time.Now()
	for _, task := range tasks {
		elapsed := now.Sub(task.Started)
		// Use per-task timeout if set, otherwise fall back to global config
		timeout := task.Timeout
		if timeout == 0 {
			timeout = d.config.Timeout
		}
		result = append(result, TaskInfo{
			Project:       task.Project,
			Name:          task.Name,
			PID:           task.PID,
			Started:       task.Started.Format(time.RFC3339),
			Elapsed:       elapsed.Round(time.Second).String(),
			Timeout:       timeout.String(),
			TimeRemaining: (timeout - elapsed).String(),
			PercentUsed:   elapsed.Round(time.Second).Seconds() / timeout.Seconds() * 100,
			LogPath:       task.LogPath,
			SessionID:     task.SessionID,
			Status:        task.Status,
		})
	}

	// Sort by started time (oldest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Started < result[j].Started
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (d *Daemon) handleTimeout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.tasksMu.RLock()
	tasks := make([]*RunningTask, 0, len(d.tasks))
	for _, task := range d.tasks {
		tasks = append(tasks, task)
	}
	d.tasksMu.RUnlock()

	var result []TaskInfo
	now := time.Now()
	for _, task := range tasks {
		elapsed := now.Sub(task.Started)
		// Use per-task timeout if set, otherwise fall back to global config
		timeout := task.Timeout
		if timeout == 0 {
			timeout = d.config.Timeout
		}
		result = append(result, TaskInfo{
			Project:       task.Project,
			Name:          task.Name,
			PID:           task.PID,
			Started:       task.Started.Format(time.RFC3339),
			Elapsed:       elapsed.Round(time.Second).String(),
			Timeout:       timeout.String(),
			TimeRemaining: (timeout - elapsed).String(),
			PercentUsed:   elapsed.Round(time.Second).Seconds() / timeout.Seconds() * 100,
			LogPath:       task.LogPath,
			SessionID:     task.SessionID,
		})
	}

	// Sort by started time (oldest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Started < result[j].Started
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (d *Daemon) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var result []TaskQueueInfo

	// First, get running tasks
	d.tasksMu.RLock()
	for _, task := range d.tasks {
		result = append(result, TaskQueueInfo{
			Project: task.Project,
			Name:    task.Name,
			Status:  "running",
		})
	}
	d.tasksMu.RUnlock()

	// Get pending/skipped tasks from the last tick
	d.pendingTasksMu.RLock()
	for taskKey, skipReason := range d.pendingTasks {
		// Parse taskKey as project/path
		parts := strings.Split(taskKey, "/")
		projectPath := ""
		taskName := ""
		if len(parts) >= 2 {
			projectPath = strings.Join(parts[:len(parts)-1], "/")
			taskName = parts[len(parts)-1]
		} else {
			taskName = taskKey
		}
		status := "pending"
		if skipReason != "" {
			status = "skipped"
		}
		result = append(result, TaskQueueInfo{
			Project:    projectPath,
			Name:       taskName,
			Status:     status,
			SkipReason: skipReason,
		})
	}
	d.pendingTasksMu.RUnlock()

	// Sort by project/name for consistent output
	sort.Slice(result, func(i, j int) bool {
		if result[i].Project != result[j].Project {
			return result[i].Project < result[j].Project
		}
		return result[i].Name < result[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (d *Daemon) handleBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.persistentBudgetUsedMu.Lock()
	// Return the raw budget usage map as task_key -> seconds
	result := make(map[string]float64, len(d.persistentBudgetUsed))
	for k, v := range d.persistentBudgetUsed {
		result[k] = v.Seconds()
	}
	d.persistentBudgetUsedMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (d *Daemon) handleResetBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TaskKey string `json:"task_key"` // project_path/task_name
		Budget  string `json:"budget"`   // optional: new budget duration (empty = reset to 0)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	d.persistentBudgetUsedMu.Lock()
	if req.Budget != "" {
		// Set used to a negative offset so effective remaining = parsed budget
		if newBudget, err := time.ParseDuration(req.Budget); err == nil {
			// Reset used to (total budget - new budget) to give them the requested remaining time
			// Actually simpler: just reset used to 0 and let the budget field handle it
			_ = newBudget
			d.persistentBudgetUsed[req.TaskKey] = 0
		} else {
			d.persistentBudgetUsedMu.Unlock()
			http.Error(w, "invalid budget duration", http.StatusBadRequest)
			return
		}
	} else {
		d.persistentBudgetUsed[req.TaskKey] = 0
	}
	d.persistentBudgetUsedMu.Unlock()

	// Also unstop the task if it was stopped due to budget exhaustion
	d.stoppedMu.Lock()
	// Find and remove any stopped state for this task
	for id := range d.stoppedTasks {
		if strings.HasSuffix(req.TaskKey, "/"+id) || req.TaskKey == id {
			delete(d.stoppedTasks, id)
			break
		}
	}
	d.stoppedMu.Unlock()

	fmt.Fprintf(w, "budget reset for %s", req.TaskKey)
}

func (d *Daemon) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Trigger reload via the channel
	select {
	case d.reload <- struct{}{}:
		fmt.Fprintf(w, "config reload triggered")
	default:
		// Channel already has a pending signal
		fmt.Fprintf(w, "config reload already in progress")
	}
}

func (d *Daemon) handleKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req KillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	d.tasksMu.Lock()
	defer d.tasksMu.Unlock()

	// Find task by name or UUID (ID field contains the todo ID)
	var found *RunningTask
	for _, task := range d.tasks {
		if task.TaskID == req.ID || task.Name == req.ID || task.Project == req.ID {
			found = task
			break
		}
	}

	if found == nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	// Cancel the task's context
	found.Cancel()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "killed", "name": found.Name})
}

// DaemonStatus holds runtime state about the daemon.
type DaemonStatus struct {
	Draining bool `json:"draining"`
}

func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := DaemonStatus{
		Draining: atomic.LoadInt32(&d.draining) == 1,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HealthResponse is the JSON response for /health endpoint.
type HealthResponse struct {
	Healthy          bool              `json:"healthy"`
	WorkersAvailable int               `json:"workers_available"`
	WorkersTotal     int               `json:"workers_total"`
	WatchedProjects  int               `json:"watched_projects"`
	TasksRunning     int               `json:"tasks_running"`
	Uptime           string            `json:"daemon_uptime,omitempty"`
	Components       map[string]string `json:"components,omitempty"`
	Detailed         bool              `json:"-"`
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if detailed mode
	detailed := r.URL.Query().Get("detailed") == "true"

	// Get worker counts
	maxWorkers := d.config.MaxWorkers
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	d.inFlightMu.Lock()
	inFlight := len(d.inFlight)
	d.inFlightMu.Unlock()

	d.tasksMu.RLock()
	tasksRunning := len(d.tasks)
	d.tasksMu.RUnlock()

	workersAvailable := maxWorkers - inFlight
	if workersAvailable < 0 {
		workersAvailable = 0
	}

	// Check if draining (not healthy)
	draining := atomic.LoadInt32(&d.draining) == 1

	// Count watched projects
	watchedPaths := loadWatchedPaths()
	watchedProjects := len(watchedPaths)

	// Determine overall health
	healthy := !draining && workersAvailable > 0

	// Build response
	resp := HealthResponse{
		Healthy:          healthy,
		WorkersAvailable: workersAvailable,
		WorkersTotal:     maxWorkers,
		WatchedProjects:  watchedProjects,
		TasksRunning:     tasksRunning,
		Detailed:         detailed,
	}

	// Add component status if detailed
	if detailed {
		// Calculate uptime
		if d.startedAt.IsZero() {
			resp.Uptime = "unknown"
		} else {
			resp.Uptime = time.Since(d.startedAt).Round(time.Second).String()
		}

		resp.Components = map[string]string{
			"socket_server":    "ok",
			"config_loaded":   "ok",
			"watched_projects": "ok",
		}
		if draining {
			resp.Components["daemon_status"] = "draining"
			resp.Healthy = false
		}
	}

	// Set appropriate status code
	if !healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// recordDurationMetric updates histogram buckets and sum for a task duration.
func (d *Daemon) recordDurationMetric(elapsed time.Duration) {
	secs := elapsed.Seconds()
	if secs <= 60 {
		atomic.AddInt64(&d.metricsDurBucket60, 1)
	}
	if secs <= 300 {
		atomic.AddInt64(&d.metricsDurBucket300, 1)
	}
	if secs <= 900 {
		atomic.AddInt64(&d.metricsDurBucket900, 1)
	}
	atomic.AddInt64(&d.metricsDurBucketInf, 1)
	atomic.AddInt64(&d.metricsDurSum, int64(elapsed/time.Millisecond))
}

func (d *Daemon) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Gather gauge metrics
	maxWorkers := d.config.MaxWorkers
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	d.tasksMu.RLock()
	tasksRunning := len(d.tasks)
	d.tasksMu.RUnlock()

	d.inFlightMu.Lock()
	inFlight := len(d.inFlight)
	d.inFlightMu.Unlock()

	workersAvailable := maxWorkers - inFlight
	if workersAvailable < 0 {
		workersAvailable = 0
	}

	d.pendingTasksMu.RLock()
	tasksPending := len(d.pendingTasks)
	d.pendingTasksMu.RUnlock()

	watchedProjects := len(loadWatchedPaths())

	var uptimeSeconds float64
	if !d.startedAt.IsZero() {
		uptimeSeconds = time.Since(d.startedAt).Seconds()
	}

	// Read counters atomically
	successCount := atomic.LoadInt64(&d.metricsSuccessCount)
	failureCount := atomic.LoadInt64(&d.metricsFailureCount)
	bucket60 := atomic.LoadInt64(&d.metricsDurBucket60)
	bucket300 := atomic.LoadInt64(&d.metricsDurBucket300)
	bucket900 := atomic.LoadInt64(&d.metricsDurBucket900)
	bucketInf := atomic.LoadInt64(&d.metricsDurBucketInf)
	durSumMs := atomic.LoadInt64(&d.metricsDurSum)
	durSumSec := float64(durSumMs) / 1000.0

	// Write Prometheus text format
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP anvil_tasks_running Number of currently running tasks\n")
	fmt.Fprintf(w, "# TYPE anvil_tasks_running gauge\n")
	fmt.Fprintf(w, "anvil_tasks_running %d\n", tasksRunning)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "# HELP anvil_tasks_pending Number of pending tasks in queue\n")
	fmt.Fprintf(w, "# TYPE anvil_tasks_pending gauge\n")
	fmt.Fprintf(w, "anvil_tasks_pending %d\n", tasksPending)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "# HELP anvil_worker_slots_available Number of available worker slots\n")
	fmt.Fprintf(w, "# TYPE anvil_worker_slots_available gauge\n")
	fmt.Fprintf(w, "anvil_worker_slots_available %d\n", workersAvailable)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "# HELP anvil_projects_watched Number of watched projects\n")
	fmt.Fprintf(w, "# TYPE anvil_projects_watched gauge\n")
	fmt.Fprintf(w, "anvil_projects_watched %d\n", watchedProjects)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "# HELP anvil_daemon_uptime_seconds Daemon uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE anvil_daemon_uptime_seconds gauge\n")
	fmt.Fprintf(w, "anvil_daemon_uptime_seconds %.0f\n", uptimeSeconds)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "# HELP anvil_task_runs_total Total number of task runs since daemon start\n")
	fmt.Fprintf(w, "# TYPE anvil_task_runs_total counter\n")
	fmt.Fprintf(w, "anvil_task_runs_total{status=\"success\"} %d\n", successCount)
	fmt.Fprintf(w, "anvil_task_runs_total{status=\"failure\"} %d\n", failureCount)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "# HELP anvil_task_duration_seconds Task execution duration\n")
	fmt.Fprintf(w, "# TYPE anvil_task_duration_seconds histogram\n")
	fmt.Fprintf(w, "anvil_task_duration_seconds_bucket{le=\"60\"} %d\n", bucket60)
	fmt.Fprintf(w, "anvil_task_duration_seconds_bucket{le=\"300\"} %d\n", bucket300)
	fmt.Fprintf(w, "anvil_task_duration_seconds_bucket{le=\"900\"} %d\n", bucket900)
	fmt.Fprintf(w, "anvil_task_duration_seconds_bucket{le=\"+Inf\"} %d\n", bucketInf)
	fmt.Fprintf(w, "anvil_task_duration_seconds_sum %.3f\n", durSumSec)
	fmt.Fprintf(w, "anvil_task_duration_seconds_count %d\n", bucketInf)
}

func (d *Daemon) handleDrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	atomic.StoreInt32(&d.draining, 1)
	dlog.Info("stop-on-idle activated — daemon will drain and exit when all tasks finish")

	// If nothing is running or queued right now, stop immediately.
	d.tasksMu.RLock()
	tasksRunning := len(d.tasks)
	d.tasksMu.RUnlock()
	d.inFlightMu.Lock()
	queued := len(d.inFlight)
	d.inFlightMu.Unlock()
	if tasksRunning == 0 && queued == 0 {
		dlog.Info("drain complete — no tasks running, stopping daemon")
		go d.Stop()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "draining"})
}

// RunRequest is the JSON payload for /run (force-trigger a task).
type RunRequest struct {
	ProjectPath string `json:"project_path"`
	TaskID      string `json:"task_id"`
	TaskName    string `json:"task_name"`
}

// DrainTaskRequest is the JSON payload for /drain/task.
type DrainTaskRequest struct {
	ID string `json:"id"`
}

func (d *Daemon) handleDrainTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DrainTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	d.drainedMu.Lock()
	d.drainedTasks[req.ID] = true
	d.drainedMu.Unlock()

	dlog.Info("stop-on-idle set for task %s — will not reschedule after current run", req.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "drained", "id": req.ID})
}

func (d *Daemon) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ProjectPath == "" || req.TaskID == "" {
		http.Error(w, "project_path and task_id are required", http.StatusBadRequest)
		return
	}

	// Check if the task is already running
	d.tasksMu.Lock()
	var runningTask *RunningTask
	for _, task := range d.tasks {
		if task.TaskID == req.TaskID {
			runningTask = task
			break
		}
	}
	d.tasksMu.Unlock()

	// Load the project and find the todo
	proj, err := project.Load(req.ProjectPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load project: %v", err), http.StatusInternalServerError)
		return
	}

	allTodos, err := proj.LoadTodos()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load todos: %v", err), http.StatusInternalServerError)
		return
	}

	var found *project.Todo
	for i := range allTodos {
		if allTodos[i].ID == req.TaskID {
			found = &allTodos[i]
			break
		}
	}
	if found == nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	// If the task is already running, kill it first (for persistent tasks, restart; for others, reject)
	if runningTask != nil {
		if !found.IsPersistent() {
			http.Error(w, fmt.Sprintf("task %s is already running", req.TaskName), http.StatusConflict)
			return
		}
		// Persistent task: kill current instance, then re-dispatch
		runningTask.Cancel()
		dlog.Info("force-run: killed running persistent task %s for restart", req.TaskName)
	}

	// Clear stopped state so the task can be dispatched
	d.stoppedMu.Lock()
	delete(d.stoppedTasks, found.ID)
	d.stoppedMu.Unlock()

	// Enqueue for immediate dispatch (bypass cron, skip pre_check by clearing it)
	todo := *found
	todo.PreCheck = "" // Skip pre_check for forced runs

	taskKey := fmt.Sprintf("%s/%s", proj.Path, todo.Name)

	d.inFlightMu.Lock()
	d.inFlight[taskKey]++
	d.inFlightMu.Unlock()

	projName := filepath.Base(proj.Path)
	dlog.Info("force-run requested for %s/%s — dispatching immediately", projName, todo.Name)

	select {
	case d.workQueue <- workItem{project: proj, todo: todo}:
		// dispatched
	default:
		d.inFlightMu.Lock()
		d.inFlight[taskKey]--
		if d.inFlight[taskKey] <= 0 {
			delete(d.inFlight, taskKey)
		}
		d.inFlightMu.Unlock()
		http.Error(w, "work queue full — try again later", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "dispatched", "name": todo.Name})
}

// StopRequest is the JSON payload for /stop (stop a persistent task permanently).
type StopRequest struct {
	ID string `json:"id"`
}

// StartRequest is the JSON payload for /start (restart a stopped persistent task).
type StartRequest struct {
	ProjectPath string `json:"project_path"`
	TaskID      string `json:"task_id"`
	TaskName    string `json:"task_name"`
}

func (d *Daemon) handleStopTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	// Mark the task as stopped so it won't be re-dispatched
	d.stoppedMu.Lock()
	d.stoppedTasks[req.ID] = true
	d.stoppedMu.Unlock()

	// Kill the running instance if any
	d.tasksMu.Lock()
	var found *RunningTask
	for _, task := range d.tasks {
		if task.TaskID == req.ID || task.Name == req.ID {
			found = task
			break
		}
	}
	if found != nil {
		found.Cancel()
	}
	d.tasksMu.Unlock()

	name := req.ID
	if found != nil {
		name = found.Name
	}
	dlog.Info("persistent task %s stopped — will not be re-dispatched until started", name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped", "id": req.ID})
}

func (d *Daemon) handleStartTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.TaskID == "" {
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}

	// Clear stopped state
	d.stoppedMu.Lock()
	wasStopped := d.stoppedTasks[req.TaskID]
	delete(d.stoppedTasks, req.TaskID)
	d.stoppedMu.Unlock()

	// Also clear per-task drain state
	d.drainedMu.Lock()
	delete(d.drainedTasks, req.TaskID)
	d.drainedMu.Unlock()

	if !wasStopped {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "already_running", "id": req.TaskID})
		return
	}

	dlog.Info("persistent task %s started — will be dispatched on next tick", req.TaskName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started", "id": req.TaskID})
}

func (d *Daemon) tick(now time.Time) {
	// Run retention pruning if configured (once per minute)
	if d.config.Retention.MaxAge > 0 || d.config.Retention.MaxRuns > 0 {
		d.pruneOldData(now)
	}

	thisMinute := now.Truncate(time.Minute)

	// Only evaluate cron schedules once per minute
	cronTick := !thisMinute.Equal(d.lastTick)
	if cronTick {
		d.lastTick = thisMinute
	}

	paths := loadWatchedPaths()
	if len(paths) == 0 {
		if cronTick {
			dlog.TickNoProjects(now)
		}
		return
	}

	// Load all projects
	var projects []*project.Project
	for _, p := range paths {
		proj, err := project.Load(p)
		if err != nil {
			dlog.Warn("skip %s: %v", p, err)
			continue
		}
		projects = append(projects, proj)
	}

	// Sort projects alphabetically by path
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})

	// On non-cron ticks, log heartbeat and any running tasks
	if !cronTick {
		d.tasksMu.RLock()
		var busyNames []string
		for _, task := range d.tasks {
			elapsed := time.Since(task.Started).Round(time.Second)
			label := taskLogLink(task)
			if task.Status != "" {
				label = task.Status
			}
			busyNames = append(busyNames, fmt.Sprintf("%s (%s)", label, elapsed))
		}
		d.tasksMu.RUnlock()
		if len(busyNames) > 0 {
			dlog.TickRunning(now, strings.Join(busyNames, ", "))
		} else {
			dlog.TickIdle(now)
		}
		return
	}

	// Clear pending tasks from previous tick before processing new one
	d.pendingTasksMu.Lock()
	d.pendingTasks = make(map[string]string)
	d.pendingTasksMu.Unlock()

	// Collect all due todos across all projects for global priority ordering
	type projectTodo struct {
		proj *project.Project
		todo project.Todo
	}
	var dueTodos []projectTodo
	totalTodos := 0
	totalMatched := 0

	for _, proj := range projects {
		projName := filepath.Base(proj.Path)
		allTodos, err := proj.LoadTodos()
		if err != nil {
			dlog.Warn("%s: error loading todos: %v", projName, err)
			continue
		}
		totalTodos += len(allTodos)

		for _, t := range allTodos {
			// Skip disabled tasks silently
			if t.Disabled {
				continue
			}
			// Include task if:
			// - one-shot (empty schedule)
			// - cron schedule matches current minute
			// - persistent task (run immediately on tick)
			if t.Schedule == "" || t.IsPersistent() || cron.Matches(t.Schedule, thisMinute) {
				// For one-shot todos, skip if a stale lock file exists from a
				// previous daemon crash. The user must remove the lock file (or
				// the todo file) to unblock the task.
				if t.Schedule == "" {
					lockPath := t.Path + ".lock"
					if _, err := os.Stat(lockPath); err == nil {
						dlog.Warn("skip %s/%s — stale lock file %s (daemon crashed during previous run? remove lock to retry)", projName, t.Name, lockPath)
						continue
					}
				}
				dueTodos = append(dueTodos, projectTodo{proj, t})
				totalMatched++
			}
		}
	}

	// Sort globally by priority (p0 first), then by name (oldest timestamp first)
	sort.SliceStable(dueTodos, func(i, j int) bool {
		if dueTodos[i].todo.Priority != dueTodos[j].todo.Priority {
			return dueTodos[i].todo.Priority < dueTodos[j].todo.Priority
		}
		return dueTodos[i].todo.Name < dueTodos[j].todo.Name
	})

	// Clear pending tasks from previous tick
	d.pendingTasksMu.Lock()
	d.pendingTasks = make(map[string]string)
	d.pendingTasksMu.Unlock()

	dispatched := 0
	for _, pt := range dueTodos {
		taskKey := fmt.Sprintf("%s/%s", pt.proj.Path, pt.todo.Name)
		projName := filepath.Base(pt.proj.Path)

		// Skip if already at max concurrent instances (queued or executing)
		maxConcurrent := pt.todo.MaxConcurrent
		if maxConcurrent < 1 {
			maxConcurrent = 1
		}
		d.inFlightMu.Lock()
		if d.inFlight[taskKey] >= maxConcurrent {
			d.inFlightMu.Unlock()
			dlog.Info("skip %s/%s — already in-flight", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "max concurrent"
			d.pendingTasksMu.Unlock()
			continue
		}
		d.inFlight[taskKey]++
		d.inFlightMu.Unlock()

		// Skip if task has dependencies that haven't completed successfully
		if len(pt.todo.DependsOn) > 0 {
			depMet, depFail := checkDependenciesMet(pt.proj.Path, pt.todo.DependsOn)
			if !depMet {
				reason := "dependency not met"
				if depFail != "" {
					reason = depFail
				}
				dlog.Info("skip %s/%s — %s", projName, pt.todo.Name, reason)
				d.pendingTasksMu.Lock()
				d.pendingTasks[taskKey] = reason
				d.pendingTasksMu.Unlock()
				d.inFlightMu.Lock()
				d.inFlight[taskKey]--
				if d.inFlight[taskKey] <= 0 {
					delete(d.inFlight, taskKey)
				}
				d.inFlightMu.Unlock()
				continue
			}
		}

		// Skip dispatch if persistent task has been explicitly stopped
		d.stoppedMu.Lock()
		taskStopped := d.stoppedTasks[pt.todo.ID]
		d.stoppedMu.Unlock()
		if taskStopped {
			dlog.Info("skip %s/%s — stopped", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "stopped"
			d.pendingTasksMu.Unlock()
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
			continue
		}

		// Skip dispatch if daemon-wide drain is active (persistent tasks exempt)
		if atomic.LoadInt32(&d.draining) == 1 && !pt.todo.IsPersistent() {
			dlog.Info("skip %s/%s — draining", projName, pt.todo.Name)
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "daemon draining"
			d.pendingTasksMu.Unlock()
			continue
		}

		// Skip dispatch if this specific task is marked stop-on-idle
		d.drainedMu.Lock()
		taskDrained := d.drainedTasks[pt.todo.ID]
		d.drainedMu.Unlock()
		if taskDrained {
			dlog.Info("skip %s/%s — stop-on-idle set", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "stop-on-idle"
			d.pendingTasksMu.Unlock()
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
			continue
		}

		// Skip dispatch if all runners are in cooldown
		d.runnerCooldownsMu.Lock()
		now := time.Now()
		allInCooldown := true
		for idx := range d.runner.Commands {
			if expires, ok := d.runnerCooldowns[idx]; !ok || now.After(expires) {
				allInCooldown = false
				break
			}
		}
		d.runnerCooldownsMu.Unlock()
		if allInCooldown {
			dlog.Info("skip %s/%s — all runners in cooldown", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "all runners in cooldown"
			d.pendingTasksMu.Unlock()
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
			continue
		}

		// Skip dispatch if persistent task is in cooldown
		if pt.todo.IsPersistent() && pt.todo.PersistentCooldown > 0 {
			d.persistentCooldownsMu.Lock()
			if expires, ok := d.persistentCooldowns[taskKey]; ok && now.Before(expires) {
				d.persistentCooldownsMu.Unlock()
				dlog.Info("skip %s/%s — persistent task in cooldown (expires %v)", projName, pt.todo.Name, expires.Round(time.Second))
				d.pendingTasksMu.Lock()
				d.pendingTasks[taskKey] = "persistent task in cooldown"
				d.pendingTasksMu.Unlock()
				d.inFlightMu.Lock()
				d.inFlight[taskKey]--
				if d.inFlight[taskKey] <= 0 {
					delete(d.inFlight, taskKey)
				}
				d.inFlightMu.Unlock()
				continue
			}
			d.persistentCooldownsMu.Unlock()
		}

		// Skip dispatch if persistent task has exhausted its budget
		if pt.todo.IsPersistent() && pt.todo.PersistentBudget > 0 {
			d.persistentBudgetUsedMu.Lock()
			used := d.persistentBudgetUsed[taskKey]
			d.persistentBudgetUsedMu.Unlock()
			if used >= pt.todo.PersistentBudget {
				dlog.Info("skip %s/%s — persistent budget exhausted (%v / %v)", projName, pt.todo.Name, used.Round(time.Second), pt.todo.PersistentBudget)
				d.pendingTasksMu.Lock()
				d.pendingTasks[taskKey] = "budget exhausted"
				d.pendingTasksMu.Unlock()
				d.inFlightMu.Lock()
				d.inFlight[taskKey]--
				if d.inFlight[taskKey] <= 0 {
					delete(d.inFlight, taskKey)
				}
				d.inFlightMu.Unlock()
				continue
			}
		}

		// Starvation prevention: if a persistent task has been waiting too long,
		// skip it to let higher-priority work through. This prevents low-priority
		// persistent tasks from blocking high-priority cron jobs indefinitely.
		if pt.todo.IsPersistent() {
			d.starvationTrackersMu.Lock()
			waitStart, exists := d.starvationTrackers[taskKey]
			if !exists {
				// First time seeing this task as pending - start tracking
				d.starvationTrackers[taskKey] = now
				waitStart = now
			}
			waitDuration := now.Sub(waitStart)
			d.starvationTrackersMu.Unlock()

			// After 5 minutes of waiting, yield to let higher priority work through
			if waitDuration > 5*time.Minute {
				// Check if there are higher-priority tasks waiting
				// If yes, skip this persistent task to let them through
				d.pendingTasksMu.RLock()
				hasHigherPriorityWaiting := false
				for key, reason := range d.pendingTasks {
					if key != taskKey && reason != "persistent task in cooldown" {
						// Check if this is a higher priority task
						for _, otherPt := range dueTodos {
							otherKey := fmt.Sprintf("%s/%s", otherPt.proj.Path, otherPt.todo.Name)
							if otherKey == key && otherPt.todo.Priority < pt.todo.Priority {
								hasHigherPriorityWaiting = true
								break
							}
						}
						if hasHigherPriorityWaiting {
							break
						}
					}
				}
				d.pendingTasksMu.RUnlock()

				if hasHigherPriorityWaiting {
					dlog.Info("skip %s/%s — starvation prevention: yielding after %v of waiting", projName, pt.todo.Name, waitDuration.Round(time.Second))
					d.pendingTasksMu.Lock()
					d.pendingTasks[taskKey] = "starvation prevention: yielding"
					d.pendingTasksMu.Unlock()
					d.inFlightMu.Lock()
					d.inFlight[taskKey]--
					if d.inFlight[taskKey] <= 0 {
						delete(d.inFlight, taskKey)
					}
					d.inFlightMu.Unlock()
					// Clear starvation tracker - we're letting it wait longer
					d.starvationTrackersMu.Lock()
					delete(d.starvationTrackers, taskKey)
					d.starvationTrackersMu.Unlock()
					continue
				}
			}
		}

		dlog.Dispatch(projName, pt.todo.Name, pt.todo.Priority, pt.todo.Schedule)

		// Non-blocking send; if queue is full, clear in-flight and warn
		select {
		case d.workQueue <- workItem{project: pt.proj, todo: pt.todo}:
			dispatched++
			// Clear starvation tracker for persistent tasks when dispatched
			if pt.todo.IsPersistent() {
				d.starvationTrackersMu.Lock()
				delete(d.starvationTrackers, taskKey)
				d.starvationTrackersMu.Unlock()
			}
		default:
			dlog.Warn("work queue full, dropping %s/%s", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "work queue full"
			d.pendingTasksMu.Unlock()
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
		}
	}

	dlog.TickSummary(now, len(projects), totalTodos, totalMatched, dispatched)
}

// osc8Link wraps text in an OSC 8 terminal hyperlink pointing to url.
// In terminals that support OSC 8 (iTerm2, kitty, foot, etc.) the text
// becomes clickable; in others it renders as plain text.
func osc8Link(text, url string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

// taskLogLink returns the task name wrapped in an OSC 8 hyperlink that opens
// the session JSONL log. Falls back to plain text if no session is found.
func taskLogLink(task *RunningTask) string {
	label := fmt.Sprintf("%s/%s", filepath.Base(task.Project), task.Name)
	sessionID, err := project.LatestSessionID(task.Project, task.TaskID)
	if err != nil {
		return label
	}
	logPath := project.SessionPath(task.Project, sessionID)
	if _, statErr := os.Stat(logPath); statErr != nil {
		return label
	}
	return osc8Link(label, "file://"+logPath)
}

// loadWatchedPaths scans ~/.anvil/watched/ and returns project paths

// newRunID generates a unique run ID for a task execution
func newRunID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// pruneOldData removes old log and run files based on retention policy.
// It runs once per tick to keep disk usage bounded.
func (d *Daemon) pruneOldData(now time.Time) {
	paths := loadWatchedPaths()
	if len(paths) == 0 {
		return
	}

	// Only prune on cron ticks (once per minute) to avoid overhead
	thisMinute := now.Truncate(time.Minute)
	if !thisMinute.Equal(d.lastTick) {
		return
	}

	for _, projPath := range paths {
		d.pruneProject(projPath)
	}
}

func (d *Daemon) pruneProject(projPath string) {
	anvilDir := filepath.Join(projPath, ".anvil")

	// Prune logs
	logsDir := filepath.Join(anvilDir, "logs")
	if _, err := os.Stat(logsDir); err == nil {
		d.pruneDir(logsDir, "log")
	}

	// Prune runs
	runsDir := filepath.Join(anvilDir, "runs")
	if _, err := os.Stat(runsDir); err == nil {
		d.pruneDir(runsDir, "run")
	}
}

func (d *Daemon) pruneDir(dir, kind string) {
	taskDirs, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, taskDir := range taskDirs {
		if !taskDir.IsDir() {
			continue
		}
		taskPath := filepath.Join(dir, taskDir.Name())
		d.pruneTaskDir(taskPath, kind)
	}
}

func (d *Daemon) pruneTaskDir(taskPath, kind string) {
	entries, err := os.ReadDir(taskPath)
	if err != nil {
		return
	}

	// Collect files with their modification times
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{entry.Name(), info.ModTime()})
	}

	// Sort by modification time (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	var toDelete []string
	cutoff := time.Now().Add(-d.config.Retention.MaxAge)

	// Delete by age
	if d.config.Retention.MaxAge > 0 {
		for _, f := range files {
			if f.modTime.Before(cutoff) {
				toDelete = append(toDelete, f.path)
			}
		}
	}

	// Delete by count (keep MaxRuns most recent)
	if d.config.Retention.MaxRuns > 0 && len(files) > d.config.Retention.MaxRuns {
		keep := len(files) - d.config.Retention.MaxRuns
		// files are sorted oldest first, so delete first 'keep' entries
		for i := 0; i < keep; i++ {
			toDelete = append(toDelete, files[i].path)
		}
	}

	// Actually delete the files
	for _, name := range toDelete {
		path := filepath.Join(taskPath, name)
		if err := os.Remove(path); err != nil {
			dlog.Warn("failed to prune %s: %v", path, err)
		}
	}
}

func loadWatchedPaths() []string {
	watchedDir := config.WatchedDir()
	dirs, err := os.ReadDir(watchedDir)
	if err != nil {
		return nil
	}

	var paths []string
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		dirPath := filepath.Join(watchedDir, d.Name())
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		// Sort descending to get latest file first
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() > entries[j].Name()
		})

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
			if err != nil {
				break
			}

			p := parseWatchedPath(string(data))
			if p != "" {
				paths = append(paths, p)
			}
			break
		}
	}

	return paths
}

type watchFrontmatter struct {
	Path string `yaml:"path"`
}

func parseWatchedPath(content string) string {
	start := strings.Index(content, "---\n")
	if start == -1 {
		return ""
	}
	end := strings.Index(content[start+4:], "\n---")
	if end == -1 {
		return ""
	}

	var fm watchFrontmatter
	if err := yaml.Unmarshal([]byte(content[start+4:start+4+end]), &fm); err != nil {
		return ""
	}
	return fm.Path
}

// IsDaemonRunning checks if the daemon is running by attempting to connect to the socket
func IsDaemonRunning() bool {
	conn, err := net.Dial("unix", filepath.Join(config.Dir(), "daemon.sock"))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// socketClient returns an HTTP client that dials the daemon's unix socket
func socketClient() *http.Client {
	sockPath := filepath.Join(config.Dir(), "daemon.sock")
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sockPath)
			},
		},
	}
}

// SendPsRequest queries the daemon's /ps endpoint and returns the running tasks
func SendPsRequest() ([]TaskInfo, error) {
	resp, err := socketClient().Get("http://daemon/ps")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon request failed: %s", resp.Status)
	}

	var tasks []TaskInfo
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

// SendStatusRequest queries the daemon's /status endpoint.
func SendStatusRequest() (*DaemonStatus, error) {
	resp, err := socketClient().Get("http://daemon/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon request failed: %s", resp.Status)
	}

	var status DaemonStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// SendTimeoutRequest queries the daemon's /timeout endpoint and returns task timeout info.
func SendTimeoutRequest() ([]TaskInfo, error) {
	resp, err := socketClient().Get("http://daemon/timeout")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon request failed: %s", resp.Status)
	}

	var tasks []TaskInfo
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

// SendQueueRequest queries the daemon's /queue endpoint and returns queue status.
func SendQueueRequest() ([]TaskQueueInfo, error) {
	resp, err := socketClient().Get("http://daemon/queue")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon request failed: %s", resp.Status)
	}

	var tasks []TaskQueueInfo
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

// SendDrainRequest tells the daemon to stop-on-idle (daemon-wide).
func SendDrainRequest() error {
	resp, err := socketClient().Post("http://daemon/drain", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon drain failed: %s", string(body))
	}
	return nil
}

// SendDrainTaskRequest marks a specific task (by ID) for stop-on-idle.
func SendDrainTaskRequest(id string) error {
	data, err := json.Marshal(DrainTaskRequest{ID: id})
	if err != nil {
		return err
	}

	resp, err := socketClient().Post("http://daemon/drain/task", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon drain task failed: %s", string(body))
	}
	return nil
}

// SendRunRequest asks the daemon to immediately dispatch a task.
func SendRunRequest(projectPath, taskID, taskName string) error {
	data, err := json.Marshal(RunRequest{ProjectPath: projectPath, TaskID: taskID, TaskName: taskName})
	if err != nil {
		return err
	}

	resp, err := socketClient().Post("http://daemon/run", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}

	return nil
}

// SendStopRequest tells the daemon to permanently stop a persistent task.
func SendStopRequest(id string) error {
	data, err := json.Marshal(StopRequest{ID: id})
	if err != nil {
		return err
	}

	resp, err := socketClient().Post("http://daemon/stop", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon stop failed: %s", string(body))
	}
	return nil
}

// SendStartRequest tells the daemon to start a stopped persistent task.
func SendStartRequest(projectPath, taskID, taskName string) error {
	data, err := json.Marshal(StartRequest{ProjectPath: projectPath, TaskID: taskID, TaskName: taskName})
	if err != nil {
		return err
	}

	resp, err := socketClient().Post("http://daemon/start", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon start failed: %s", string(body))
	}
	return nil
}

// SendKillRequest sends a kill request to the daemon
func SendKillRequest(id string) error {
	data, err := json.Marshal(KillRequest{ID: id})
	if err != nil {
		return err
	}

	resp, err := socketClient().Post("http://daemon/kill", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon kill failed: %s", string(body))
	}

	return nil
}

// SendReloadRequest sends a reload request to the daemon to reload its config.
func SendReloadRequest() error {
	resp, err := socketClient().Post("http://daemon/reload", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon reload failed: %s", string(body))
	}

	return nil
}

// SendBudgetRequest retrieves budget usage for all persistent tasks.
// Returns a map of task_key -> seconds_used.
func SendBudgetRequest() (map[string]float64, error) {
	resp, err := socketClient().Get("http://daemon/budget")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SendResetBudgetRequest resets the budget usage for a task.
func SendResetBudgetRequest(taskKey string) error {
	payload := fmt.Sprintf(`{"task_key":%q}`, taskKey)
	resp, err := socketClient().Post("http://daemon/reset-budget", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reset budget failed: %s", string(body))
	}
	return nil
}
