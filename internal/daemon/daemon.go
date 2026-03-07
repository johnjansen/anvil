package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand"
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
	"text/template"
	"time"

	"github.com/johnjansen/anvil/internal/cache"
	"github.com/johnjansen/anvil/internal/cluster"
	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/health"
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

// calculateBackoff computes the retry delay for the given attempt using the specified strategy.
// It applies a 1-hour maximum cap, then applies jitter if jitter > 0, with a floor of 1 second.
// The attempt parameter is 0-based (0 = first retry delay).
func calculateBackoff(strategy string, baseDelay time.Duration, attempt int, jitter float64) time.Duration {
	if baseDelay <= 0 {
		baseDelay = time.Minute
	}
	if strategy == "" {
		strategy = "exponential"
	}

	var delay time.Duration
	switch strategy {
	case "linear":
		delay = baseDelay * time.Duration(attempt+1)
	case "constant":
		delay = baseDelay
	default: // exponential
		delay = baseDelay
		for i := 0; i < attempt; i++ {
			delay *= 2
		}
	}

	// Cap at 1 hour
	const maxDelay = time.Hour
	if delay > maxDelay {
		delay = maxDelay
	}

	// Apply jitter: delay * (1 + jitter * (2*rand - 1))
	if jitter > 0 {
		r := mathrand.Float64() // [0, 1)
		factor := 1.0 + jitter*(2*r-1)
		delay = time.Duration(float64(delay) * factor)
	}

	// Floor at 1 second
	if delay < time.Second {
		delay = time.Second
	}

	return delay
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
	project          *project.Project
	todo             project.Todo
	sla              slaResult                // SLA check result from dispatch time
	remoteAssignment *cluster.TaskAssignment  // non-nil if this is a remote task from leader
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
	// circuitBreakerStorage handles circuit breaker state persistence
	circuitBreakerStorage *CircuitBreakerStorage
	// persistentBudgetUsed tracks cumulative wall-clock time consumed by each
	// persistent task during this daemon lifetime.  Resets on daemon restart.
	persistentBudgetUsed   map[string]time.Duration
	persistentBudgetUsedMu sync.Mutex
	// costBudgetUsed tracks cumulative USD cost consumed by each
	// task during this daemon lifetime. Resets on daemon restart.
	costBudgetUsed   map[string]float64
	costBudgetUsedMu sync.Mutex
	// priorityAgingQueueTimes tracks when tasks were first added to the queue for priority aging.
	// Map value is the time when the task was first queued.
	priorityAgingQueueTimes map[string]time.Time
	priorityAgingQueueTimesMu sync.Mutex
	webhooks    *webhook.Sender
	taskHashes  map[string]string // taskName -> contentHash for change detection
	clusterNode *cluster.Node // nil when cluster mode disabled
	// rateLimitSemaphore limits concurrent LLM API calls (nil = no limit)
	rateLimitSemaphore chan struct{}
	// rateLimitCounters tracks execution counts for rate limiting per task
	rateLimitCounters   map[string]*RateLimitCounter // taskID -> counter
	rateLimitCountersMu sync.Mutex
	// groupInFlight tracks how many tasks from each concurrency group are currently running
	// Map value is the count of running tasks in that group
	groupInFlight   map[string]int // groupName -> count
	groupInFlightMu sync.Mutex
	// groupRateLimits tracks API call counts for rate limiting per group
	// Map value is the counter for that group
	groupRateLimits   map[string]*RateLimitCounter // groupName -> counter
	groupRateLimitsMu sync.Mutex
	// Metrics counters for Prometheus endpoint
	metricsSuccessCount int64 // atomic: total successful task runs
	metricsFailureCount int64 // atomic: total failed task runs
	// Histogram buckets for task duration (60s, 300s, 900s, +Inf)
	metricsDurBucket60   int64 // atomic: tasks completing in ≤60s
	metricsDurBucket300  int64 // atomic: tasks completing in ≤300s
	metricsDurBucket900  int64 // atomic: tasks completing in ≤900s
	metricsDurBucketInf  int64 // atomic: all completed tasks (≤+Inf)
	metricsDurSum        int64 // atomic: sum of all durations in milliseconds

	// HealthManager handles task health checks
	healthManager *health.Manager

	// AMQPConsumer handles AMQP message queue subscriptions
	amqpConsumer *AMQPConsumer

	// FSWatcher handles filesystem path subscriptions
	fsWatcher *FSWatcher

	// GitWatcher handles git ref polling subscriptions
	gitWatcher *GitWatcher

	// WebhookServer handles HTTP webhook subscriptions
	webhookServer *WebhookServer

	// cascadeFailures tracks cascading dependency failures for reporting
	// Map key is the failed dependency task name, value is list of affected tasks
	cascadeFailures   map[string][]string // dependencyTask -> []affectedTasks
	cascadeFailuresMu sync.Mutex

	// pollingManager handles polling-based trigger conditions
	pollingManager *PollingManager

	// throttle manages global pause, throttle rate, and label-based pause
	throttle *throttleManager
}

// RateLimitCounter tracks execution counts for rate limiting per task
type RateLimitCounter struct {
	// Hourly counters (keyed by YYYY-MM-DD HH)
	Hourly map[string]int
	// Daily counters (keyed by YYYY-MM-DD)
	Daily map[string]int
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
	Status            string // dynamic status reported by task via ##anvil:status
	WarningTriggered  bool          // whether the timeout warning has been triggered
	TimeoutExtensions int           // number of timeout extensions used
	OriginalTimeout   time.Duration // original timeout before extensions
	GracefulStop           chan struct{} // closed to signal graceful shutdown request (checkpoint kill)
	ShuttingDown           bool          // true if graceful shutdown is in progress
	CheckpointEnabled      bool          // whether task has checkpoint: true
	CheckpointGracePeriod  time.Duration // max wait after SIGTERM (default 30s)
}

type KillRequest struct {
	ID         string `json:"id"`
	Checkpoint bool   `json:"checkpoint"`
}

type TaskInfo struct {
	Project           string  `json:"project"`
	Name              string  `json:"name"`
	PID               int     `json:"pid"`
	Started           string  `json:"started"`
	Elapsed           string  `json:"elapsed"`
	Timeout           string  `json:"timeout,omitempty"`
	TimeRemaining     string  `json:"time_remaining,omitempty"`
	PercentUsed       float64 `json:"percent_used,omitempty"`
	LogPath           string  `json:"log_path,omitempty"`
	SessionID         string  `json:"session_id,omitempty"`
	Status            string  `json:"status,omitempty"`
	TimeoutExtensions int     `json:"timeout_extensions,omitempty"`
	WarningTriggered  bool    `json:"warning_triggered,omitempty"`
}

// TaskQueueInfo holds information about a task in the queue or its last skip reason.
type TaskQueueInfo struct {
	Project      string `json:"project"`
	Name         string `json:"name"`
	Priority     int    `json:"priority"`
	Schedule     string `json:"schedule"`
	Status       string `json:"status"`       // "running", "pending", "skipped"
	SkipReason   string `json:"skip_reason,omitempty"` // why task was skipped in last tick
	Boost        int    `json:"boost,omitempty"`       // priority boost from aging (0 = no boost)
	CascadeCount int    `json:"cascade_count,omitempty"` // number of tasks affected by cascading failure
	CascadeFrom  string `json:"cascade_from,omitempty"`  // which task caused the cascading failure
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
	d := &Daemon{
		config:       cfg,
		runner:       runner.New(cfg.Runners, cfg.Timeout),
		workQueue:    make(chan workItem, poolSize*4),
		inFlight:     make(map[string]int),
		groupInFlight: make(map[string]int),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
		reload:       make(chan struct{}, 1),
		socketPath:   filepath.Join(config.Dir(), "daemon.sock"),
		tasks:        make(map[string]*RunningTask),
		drainedTasks: make(map[string]bool),
		persistentFailures: make(map[string]int),
		taskHashes:         make(map[string]string),
		persistentCooldowns: make(map[string]time.Time),
		starvationTrackers: make(map[string]time.Time),
		runnerCooldowns: make(map[int]time.Time),
		pendingTasks:  make(map[string]string),
		stoppedTasks:         make(map[string]bool),
		circuitBreakerStorage: NewCircuitBreakerStorage(filepath.Join(config.Dir(), "circuits")),
		persistentBudgetUsed: make(map[string]time.Duration),
		costBudgetUsed:       make(map[string]float64),
		priorityAgingQueueTimes: make(map[string]time.Time),
		webhooks:             webhook.New(cfg.Webhooks),
		rateLimitCounters:    make(map[string]*RateLimitCounter),
		groupRateLimits:      make(map[string]*RateLimitCounter),
		cascadeFailures:      make(map[string][]string),
	}

	// Initialize health manager
	d.healthManager = health.NewManager()

	// Initialize AMQP consumer
	d.amqpConsumer = NewAMQPConsumer(d)

	// Initialize filesystem watcher
	d.fsWatcher = NewFSWatcher(d)

	// Initialize git watcher
	d.gitWatcher = NewGitWatcher(d)

	// Initialize webhook server with configurable port (default 9090)
	webhookPort := cfg.WebhookPort
	if webhookPort == 0 {
		webhookPort = 9090
	}
	d.webhookServer = NewWebhookServer(fmt.Sprintf(":%d", webhookPort), d)

	// Initialize polling manager
	d.pollingManager = NewPollingManager(d)

	// Initialize throttle manager
	d.throttle = newThrottleManager()

	return d
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

	// Load persistent throttle state
	if err := d.throttle.load(); err != nil {
		dlog.Warn("failed to load throttle state: %v", err)
	}

	poolSize := d.config.MaxWorkers
	if poolSize < 1 {
		poolSize = 1
	}

	ticker := time.NewTicker(d.config.TickInterval)
	defer ticker.Stop()

	// Set up health check ticker (run every 30 seconds)
	healthTicker := time.NewTicker(30 * time.Second)
	defer healthTicker.Stop()

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

	// Webhook server starts on-demand when first webhook subscription is registered

	// Start cluster node if enabled
	if d.config.Cluster.Enabled {
		node, err := cluster.NewNode(config.Dir(), cluster.Config{
			Listen:            d.config.Cluster.Listen,
			Peers:             d.config.Cluster.Peers,
			HeartbeatInterval: d.config.Cluster.HeartbeatInterval,
			ElectionTimeout:   d.config.Cluster.ElectionTimeout,
		})
		if err != nil {
			dlog.Warn("cluster: failed to create node: %v", err)
		} else {
			node.Logf = func(format string, args ...any) {
				dlog.Info(format, args...)
			}
			if err := node.Start(); err != nil {
				dlog.Warn("cluster: failed to start: %v", err)
			} else {
				d.clusterNode = node
				dlog.Info("cluster: node %s started", node.ID())

				// T13: Worker report callback
				node.WorkerReportFn = func() cluster.WorkerReport {
					total := cap(d.workQueue) / 4 // pool size = cap/4
					busy := len(d.workQueue)
					return cluster.WorkerReport{
						TotalWorkers: total,
						BusyWorkers:  busy,
						IdleWorkers:  total - busy,
					}
				}

				// T14: Task assignment callback (follower receives task from leader)
				node.OnTaskAssign = func(assignment cluster.TaskAssignment) {
					dlog.Info("cluster: received task %s from leader", assignment.TaskName)
					// Create a minimal Todo for local execution
					todo := project.Todo{
						Name:    assignment.TaskName,
						ID:      assignment.TaskID,
						Content: assignment.Content,
						Runner:  assignment.Runner,
						RunnerChain: assignment.RunnerChain,
						Timeout: assignment.Timeout,
						Env:     assignment.Env,
						Priority: assignment.Priority,
						Labels:  assignment.Labels,
					}
					// Load the project if it exists locally, otherwise create a minimal one
					proj, err := project.Load(assignment.ProjectPath)
					if err != nil {
						dlog.Warn("cluster: cannot load project %s: %v", assignment.ProjectPath, err)
						// Send failure result back
						if d.clusterNode != nil {
							for _, addr := range d.clusterNode.PeerAddresses() {
								d.clusterNode.ReportResult(addr, cluster.TaskResult{
									AssignmentID: assignment.AssignmentID,
									TaskName:     assignment.TaskName,
									TaskID:       assignment.TaskID,
									NodeID:       node.ID(),
									ProjectPath:  assignment.ProjectPath,
									Success:      false,
									Error:        "cannot load project: " + err.Error(),
									Started:      time.Now(),
									Finished:     time.Now(),
								})
							}
						}
						return
					}
					// Enqueue for local execution
					select {
					case d.workQueue <- workItem{project: proj, todo: todo, remoteAssignment: &assignment}:
						dlog.Info("cluster: enqueued task %s for local execution", assignment.TaskName)
					default:
						dlog.Warn("cluster: work queue full, rejecting task %s", assignment.TaskName)
					}
				}

				// T15: Task result callback (leader receives result from follower)
				node.OnTaskResult = func(result cluster.TaskResult) {
					dlog.Info("cluster: received result for %s from %s (success=%v)", result.TaskName, result.NodeID, result.Success)
					// Write RunRecord on the leader
					rec := project.RunRecord{
						RunID:         result.AssignmentID,
						TaskID:        result.TaskID,
						Started:       result.Started,
						Finished:      result.Finished,
						Success:       result.Success,
						Error:         result.Error,
						OutputSummary: result.OutputSummary,
						NodeID:        result.NodeID,
					}
					if err := project.WriteRunRecord(result.ProjectPath, rec); err != nil {
						dlog.Warn("cluster: failed to write run record: %v", err)
					}
				}
			}
		}
	}

	// Load watched paths and start AMQP subscriptions
	paths := loadWatchedPaths()
	var projects []*project.Project
	for _, p := range paths {
		proj, err := project.Load(p)
		if err != nil {
			dlog.Warn("skip %s: %v", p, err)
			continue
		}
		projects = append(projects, proj)
	}
	d.startSubscriptions(projects)

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
			// Stop all AMQP consumers
			if d.amqpConsumer != nil {
				d.amqpConsumer.StopAll()
			}
			// Stop all filesystem watchers
			if d.fsWatcher != nil {
				d.fsWatcher.StopAll()
			}
			if d.gitWatcher != nil {
				d.gitWatcher.StopAll()
			}
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
		case <-healthTicker.C:
			// Run health checks in a goroutine to avoid blocking the main loop
			go d.healthManager.RunAllHealthChecks(context.Background(), loadWatchedPaths())
		}
	}
}

func (d *Daemon) Stop() {
	d.stopOnce.Do(func() {
		// Stop all AMQP consumers
		if d.amqpConsumer != nil {
			d.amqpConsumer.StopAll()
		}
		// Stop all filesystem watchers
		if d.fsWatcher != nil {
			d.fsWatcher.StopAll()
		}
		if d.gitWatcher != nil {
			d.gitWatcher.StopAll()
		}
		close(d.stop)
	})
	<-d.done
}

// Done returns a channel that is closed when the daemon has fully stopped.
func (d *Daemon) Done() <-chan struct{} {
	return d.done
}

// startSubscriptions starts all message subscriptions (AMQP and filesystem) for tasks
func (d *Daemon) startSubscriptions(projects []*project.Project) {
	dlog.Info("Starting message subscriptions for %d projects", len(projects))
	for _, proj := range projects {
		dlog.Info("Loading todos for project: %s", proj.Path)
		todos, err := proj.LoadTodos()
		if err != nil {
			dlog.Warn("Failed to load todos for project %s: %v", proj.Path, err)
			continue
		}
		dlog.Info("Loaded %d todos for project: %s", len(todos), proj.Path)

		for _, todo := range todos {
			if todo.Subscription != nil {
				switch todo.Subscription.Type {
				case "amqp":
					dlog.Info("Starting AMQP subscription for task: %s", todo.Name)
					if err := d.amqpConsumer.StartSubscription(proj, todo); err != nil {
						dlog.Warn("Failed to start AMQP subscription for task %s: %v", todo.Name, err)
					}
				case "fs":
					dlog.Info("Starting filesystem subscription for task: %s", todo.Name)
					if err := d.fsWatcher.StartSubscription(proj, todo); err != nil {
						dlog.Warn("Failed to start filesystem subscription for task %s: %v", todo.Name, err)
					}
				case "webhook":
					dlog.Info("Starting webhook subscription for task: %s", todo.Name)
					if err := d.webhookServer.StartSubscription(proj, todo); err != nil {
						dlog.Warn("Failed to start webhook subscription for task %s: %v", todo.Name, err)
					}
				case "git":
					dlog.Info("Starting git subscription for task: %s", todo.Name)
					if err := d.gitWatcher.StartSubscription(proj, todo); err != nil {
						dlog.Warn("Failed to start git subscription for task %s: %v", todo.Name, err)
					}
				}
			}
		}
	}
}

// triggerWebhookTask triggers a task execution from a webhook request
func (d *Daemon) triggerWebhookTask(proj *project.Project, task project.Todo, payload []byte, headers map[string]string) {
	dlog.Info("Triggering task %s via webhook", task.Name)

	// Add webhook payload and metadata to environment variables
	if task.Env == nil {
		task.Env = make(map[string]string)
	}
	task.Env["ANVIL_WEBHOOK_PAYLOAD"] = string(payload)

	if ct, ok := headers[http.CanonicalHeaderKey("Content-Type")]; ok {
		task.Env["ANVIL_WEBHOOK_CONTENT_TYPE"] = ct
	}
	// Support GitHub and generic webhook event headers (use canonical form)
	if ev := headers[http.CanonicalHeaderKey("X-GitHub-Event")]; ev != "" {
		task.Env["ANVIL_WEBHOOK_EVENT"] = ev
	} else if ev := headers[http.CanonicalHeaderKey("X-Webhook-Event")]; ev != "" {
		task.Env["ANVIL_WEBHOOK_EVENT"] = ev
	}
	// Pass all headers as JSON
	if len(headers) > 0 {
		headerJSON, err := json.Marshal(headers)
		if err == nil {
			task.Env["ANVIL_WEBHOOK_HEADERS"] = string(headerJSON)
		}
	}

	// Create work item and queue it
	item := workItem{
		project: proj,
		todo:    task,
	}

	select {
	case d.workQueue <- item:
		dlog.Info("Queued task %s for execution via webhook", task.Name)
	default:
		dlog.Warn("Work queue full, could not queue task %s via webhook", task.Name)
	}
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
	if d.clusterNode != nil {
		d.clusterNode.Stop()
	}
	// Stop all AMQP consumers
	if d.amqpConsumer != nil {
		d.amqpConsumer.StopAll()
	}
	// Stop all filesystem watchers
	if d.fsWatcher != nil {
		d.fsWatcher.StopAll()
	}
	if d.gitWatcher != nil {
		d.gitWatcher.StopAll()
	}
	// Stop webhook server
	if d.webhookServer != nil {
		d.webhookServer.Stop(context.Background())
	}
	// Stop all polling managers
	if d.pollingManager != nil {
		d.pollingManager.Stop()
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
	for {
		select {
		case <-d.stop:
			dlog.WorkerStopped(id)
			return
		case item, ok := <-d.workQueue:
			if !ok {
				dlog.WorkerStopped(id)
				return
			}
			// Acquire rate limit slot if configured
			if d.rateLimitSemaphore != nil {
				select {
				case <-d.stop:
					dlog.WorkerStopped(id)
					return
				case d.rateLimitSemaphore <- struct{}{}:
					// Slot acquired, proceed with task
				}
			}
			projName := filepath.Base(item.project.Path)
			dlog.WorkerPickup(id, projName, item.todo.Name, item.todo.Priority)
			d.runTask(id, item.project, item.todo, item.sla)
			// Release rate limit slot
			if d.rateLimitSemaphore != nil {
				<-d.rateLimitSemaphore
			}
			// T17: Report result to leader if this was a remote assignment
		if item.remoteAssignment != nil && d.clusterNode != nil {
			result := cluster.TaskResult{
				AssignmentID: item.remoteAssignment.AssignmentID,
				TaskName:     item.todo.Name,
				TaskID:       item.todo.ID,
				NodeID:       d.clusterNode.ID(),
				ProjectPath:  item.remoteAssignment.ProjectPath,
				Success:      true, // Will be updated from run record
				Started:      time.Now(), // Approximate
				Finished:     time.Now(),
			}
			// Send to all peers (leader will process)
			for _, addr := range d.clusterNode.PeerAddresses() {
				d.clusterNode.ReportResult(addr, result)
			}
		}
		dlog.WorkerIdle(id)
		}
	}
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

// depFailInfo holds details about a dependency that prevented a task from running.
type depFailInfo struct {
	Reason    string    // human-readable reason (e.g., "dependency failed: fetch-data.md")
	DepName   string    // name of the failed dependency
	DepError  string    // error message from the failed dependency run
	Finished  time.Time // when the dependency last ran
	ExitCode  int       // 0 if success, 1 if failed (no explicit exit code in RunRecord)
}

// checkDependenciesMet verifies dependencies have completed successfully in the current cycle,
// according to the specified dependency policy.
// Supports both local dependencies ("task.md") and cross-project dependencies ("project:task.md").
// Returns (true, nil) if dependencies are met according to policy, (false, info) if not.
func checkDependenciesMet(projectPath string, dependsOn []string, policy project.DependencyPolicyConfig) (met bool, info *depFailInfo) {
	// Default policy is "skip" - all dependencies must succeed
	onFailure := "skip"
	if policy.OnFailure != "" {
		onFailure = policy.OnFailure
	}

	switch onFailure {
	case "require_all":
		// All dependencies must succeed (original behavior)
		for _, dep := range dependsOn {
			runRecord, err := project.ResolveDependencyRunRecord(projectPath, dep)
			if err != nil {
				// No run record found - dependency hasn't run yet or can't be resolved
				return false, &depFailInfo{
					Reason:  "dependency not run: " + dep,
					DepName: dep,
				}
			}
			if !runRecord.Success {
				return false, &depFailInfo{
					Reason:   "dependency failed: " + dep,
					DepName:  dep,
					DepError: runRecord.Error,
					Finished: runRecord.Finished,
					ExitCode: 1,
				}
			}
			// Check if the run finished in this daemon cycle (within last ~1 minute)
			// This ensures dependencies from previous cycles are re-run
			elapsed := time.Since(runRecord.Finished)
			if elapsed > 2*time.Minute {
				// Dependency completed more than 2 minutes ago, re-run to ensure fresh data
				return false, &depFailInfo{
					Reason:   "dependency stale: " + dep,
					DepName:  dep,
					Finished: runRecord.Finished,
				}
			}
		}
		return true, nil

	case "require_any":
		// At least one dependency must succeed
		successCount := 0
		var firstFailure *depFailInfo
		for _, dep := range dependsOn {
			runRecord, err := project.ResolveDependencyRunRecord(projectPath, dep)
			if err != nil {
				// No run record found - dependency hasn't run yet or can't be resolved
				if firstFailure == nil {
					firstFailure = &depFailInfo{
						Reason:  "dependency not run: " + dep,
						DepName: dep,
					}
				}
				continue
			}
			if runRecord.Success {
				// Check if the run finished in this daemon cycle (within last ~1 minute)
				// This ensures dependencies from previous cycles are re-run
				elapsed := time.Since(runRecord.Finished)
				if elapsed <= 2*time.Minute {
					successCount++
				} else {
					// Dependency completed more than 2 minutes ago, re-run to ensure fresh data
					if firstFailure == nil {
						firstFailure = &depFailInfo{
							Reason:   "dependency stale: " + dep,
							DepName:  dep,
							Finished: runRecord.Finished,
						}
					}
				}
			} else {
				// Dependency failed
				if firstFailure == nil {
					firstFailure = &depFailInfo{
						Reason:   "dependency failed: " + dep,
						DepName:  dep,
						DepError: runRecord.Error,
						Finished: runRecord.Finished,
						ExitCode: 1,
					}
				}
			}
		}

		// If at least one dependency succeeded, we're good
		if successCount > 0 {
			return true, nil
		}

		// No dependencies succeeded, return the first failure
		if firstFailure != nil {
			return false, firstFailure
		}

		// All dependencies failed to resolve
		return false, &depFailInfo{
			Reason: "all dependencies failed to resolve",
		}

	default: // "skip" (default behavior)
		// Original behavior - all dependencies must succeed
		for _, dep := range dependsOn {
			runRecord, err := project.ResolveDependencyRunRecord(projectPath, dep)
			if err != nil {
				// No run record found - dependency hasn't run yet or can't be resolved
				return false, &depFailInfo{
					Reason:  "dependency not run: " + dep,
					DepName: dep,
				}
			}
			if !runRecord.Success {
				return false, &depFailInfo{
					Reason:   "dependency failed: " + dep,
					DepName:  dep,
					DepError: runRecord.Error,
					Finished: runRecord.Finished,
					ExitCode: 1,
				}
			}
			// Check if the run finished in this daemon cycle (within last ~1 minute)
			// This ensures dependencies from previous cycles are re-run
			elapsed := time.Since(runRecord.Finished)
			if elapsed > 2*time.Minute {
				// Dependency completed more than 2 minutes ago, re-run to ensure fresh data
				return false, &depFailInfo{
					Reason:   "dependency stale: " + dep,
					DepName:  dep,
					Finished: runRecord.Finished,
				}
			}
		}
		return true, nil
	}
}

// runTask executes a single todo task and handles all bookkeeping.
func (d *Daemon) runTask(workerID int, proj *project.Project, t project.Todo, sla slaResult) {
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

		// Decrement group in-flight count if task belongs to a group
		if t.ConcurrencyGroup != "" {
			d.groupInFlightMu.Lock()
			d.groupInFlight[t.ConcurrencyGroup]--
			if d.groupInFlight[t.ConcurrencyGroup] <= 0 {
				delete(d.groupInFlight, t.ConcurrencyGroup)
			}
			d.groupInFlightMu.Unlock()
		}
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

	// For adaptive timeouts, use context.WithCancel so we can extend the deadline.
	// A deadlineTimer manages the actual timeout, allowing resets on checkpoint progress.
	var ctx context.Context
	var cancel context.CancelFunc
	var deadlineTimer *time.Timer
	var adaptiveTimedOut int32 // atomic flag: 1 = deadline timer fired (adaptive timeout)
	if t.AdaptiveTimeout != nil && t.AdaptiveTimeout.Enabled {
		ctx, cancel = context.WithCancel(context.Background())
		deadlineTimer = time.AfterFunc(timeout, func() {
			atomic.StoreInt32(&adaptiveTimedOut, 1)
			cancel()
		})
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()
	if deadlineTimer != nil {
		defer deadlineTimer.Stop()
	}

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

	// Track the running task — use runID suffix so multiple instances of the
	// same task each get their own entry in the map.
	instanceKey := taskKey + "/" + runID
	d.tasksMu.Lock()
	d.tasks[instanceKey] = &RunningTask{
		Project:         proj.Path,
		Name:            t.Name,
		TaskID:          t.ID,
		PID:             os.Getpid(),
		Started:         startTime,
		Timeout:         timeout,
		OriginalTimeout: timeout,
		Cancel:                cancel,
		GracefulStop:          make(chan struct{}),
		CheckpointEnabled:     t.Checkpoint,
		CheckpointGracePeriod: t.CheckpointGracePeriod,
	}
	d.tasksMu.Unlock()

	// Timeout warning timer: fires on_timeout_warning hook before actual timeout
	if t.TimeoutWarning > 0 && t.TimeoutWarning < timeout {
		warningAt := timeout - t.TimeoutWarning
		taskLabel := filepath.Base(proj.Path) + "/" + t.Name
		timeoutWarningTimer := time.AfterFunc(warningAt, func() {
			dlog.Warn("task %s timeout warning — %v remaining", taskLabel, t.TimeoutWarning.Round(time.Second))
			d.tasksMu.Lock()
			if rt, ok := d.tasks[instanceKey]; ok {
				rt.WarningTriggered = true
			}
			d.tasksMu.Unlock()
			if t.OnTimeoutWarning != "" {
				d.runTimeoutHook("on_timeout_warning", t.OnTimeoutWarning, proj.Path, t, timeout, t.TimeoutWarning)
			}
		})
		defer timeoutWarningTimer.Stop()
	}

	defer func() {
		d.tasksMu.Lock()
		delete(d.tasks, instanceKey)
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

	// Precondition check: if set, evaluate all conditions and skip if any fail.
	// Both pre_check and precondition must pass for task to run.
	if t.Precondition != nil {
		if shouldProceed, skipReason := t.EvaluatePrecondition(proj.Path); !shouldProceed {
			taskLabel := filepath.Base(proj.Path) + "/" + t.Name
			dlog.Info("precondition skipped %s: %s", taskLabel, skipReason)
			return
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

	// Retry loop with configurable backoff strategy
	var usedSessionID string
	var logPath string
	var usedRunnerIdx int
	var stderrOutput string
	var err error
	var finalAttempt int // tracks which attempt we ended on (0-based)
	var retryDelaysUsed []string // actual delays used between attempts
	retryStartTime := time.Now() // for max_total_time tracking

	// Track checkpoint data emitted by the task (last one wins)
	var checkpointMu sync.Mutex
	var lastCheckpointData string
	// Track result data emitted by the task via ##anvil:result (last one wins)
	var resultMu sync.Mutex
	var lastResultData string
	// Track whether a graceful checkpoint stop was triggered
	var gracefulStopTriggered bool
	var graceExpired bool

	// State file path and storage config (used for persisting task state)
	var stateFilePath string
	var stateBucket string
	var stateKey string

	// Pinned run: if task has a pinned_run, use the historical output instead of executing
	if t.PinnedRun != "" {
		pinnedRecord, pinnedErr := project.ReadRunRecord(proj.Path, t.ID, t.PinnedRun)
		if pinnedErr == nil && pinnedRecord != nil {
			dlog.Info("pinned run %s for %s — using historical output", t.PinnedRun[:min(8, len(t.PinnedRun))], taskLabel)
			stderrOutput = pinnedRecord.OutputSummary
			usedSessionID = "pinned-" + t.PinnedRun[:min(8, len(t.PinnedRun))]
			logPath = ""
			usedRunnerIdx = -1
			err = nil
			goto skipExecution
		}
		dlog.Warn("pinned run %s not found for %s, executing normally", t.PinnedRun, t.Name)
	}

	// Check cache before running task
	if cache.IsCacheEnabled(t) {
		cacheKey, err := cache.CalculateCacheKey(t, proj.Path)
		if err == nil {
			cacheEntry, err := cache.GetCache(proj.Path, cacheKey)
			if err == nil && cacheEntry != nil {
				// Cache hit - use cached output instead of running the task
				taskLabel := filepath.Base(proj.Path) + "/" + t.Name
				dlog.Info("cache HIT for %s (expires in %v)", taskLabel, time.Until(cacheEntry.ExpiresAt).Round(time.Second))

				// Set the cached output as if it came from the runner
				stderrOutput = cacheEntry.Content
				usedSessionID = "cached-" + cacheKey[:8]
				logPath = ""
				usedRunnerIdx = -1
				err = nil

				// Skip the retry loop since we have cached output
				goto skipExecution
			} else {
				dlog.Info("cache MISS for %s", filepath.Base(proj.Path)+"/"+t.Name)
			}
		}
	}

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

		// If task has dependencies, collect and inject dependency results
		var collectedDepResults map[string]json.RawMessage
		if len(t.DependsOn) > 0 {
			depResults, depErr := project.CollectDependencyResults(proj.Path, t.DependsOn)
			if depErr == nil && len(depResults) > 0 {
				collectedDepResults = depResults
				depJSON, jsonErr := json.Marshal(depResults)
				if jsonErr == nil {
					if mergedEnv == nil {
						mergedEnv = make(map[string]string)
					}
					mergedEnv["ANVIL_DEPENDENCY_RESULTS"] = string(depJSON)
				}
			}
		}

		// If state is configured, inject ANVIL_STATE_FILE with current state
		if t.State != nil {
			stateBucket = t.State.Bucket
			// Resolve key template (e.g., {{ .TaskID }} -> actual task ID)
			stateKey = strings.ReplaceAll(t.State.Key, "{{ .TaskID }}", t.ID)
			// Read existing state
			existingState, err := project.ReadTaskState(proj.Path, stateBucket, stateKey)
			if err == nil && existingState != nil {
				// Create temp file for task to read/write
				tmpFile, err := os.CreateTemp("", "anvil-state-*.json")
				if err == nil {
					defer os.Remove(tmpFile.Name()) // Clean up on error
					stateData, _ := json.Marshal(existingState)
					tmpFile.Write(stateData)
					tmpFile.Close()
					stateFilePath = tmpFile.Name()
					if mergedEnv == nil {
						mergedEnv = make(map[string]string)
					}
					mergedEnv["ANVIL_STATE_FILE"] = stateFilePath
				}
			} else if os.IsNotExist(err) || err == nil {
				// No existing state - create empty temp file
				tmpFile, err := os.CreateTemp("", "anvil-state-*.json")
				if err == nil {
					defer os.Remove(tmpFile.Name()) // Clean up on error
					tmpFile.Write([]byte("{}"))
					tmpFile.Close()
					stateFilePath = tmpFile.Name()
					if mergedEnv == nil {
						mergedEnv = make(map[string]string)
					}
					mergedEnv["ANVIL_STATE_FILE"] = stateFilePath
				}
			}
		}

		// Render task content through Go templates if dependency results are available
		taskContent := t.Content
		if collectedDepResults != nil && strings.Contains(taskContent, "{{") {
			tmplData := map[string]interface{}{
				"DependencyResults": collectedDepResults,
			}
			tmpl, tmplErr := template.New("task").Option("missingkey=zero").Parse(taskContent)
			if tmplErr == nil {
				var rendered strings.Builder
				if execErr := tmpl.Execute(&rendered, tmplData); execErr == nil {
					taskContent = rendered.String()
				}
			}
		}

		// Graceful stop watcher: monitors GracefulStop channel and sends SIGTERM to child
		runnerDone := make(chan struct{})
		go func() {
			d.tasksMu.Lock()
			rt := d.tasks[instanceKey]
			d.tasksMu.Unlock()
			if rt == nil {
				return
			}
			select {
			case <-rt.GracefulStop:
				gracefulStopTriggered = true
				// Send SIGTERM to the child process
				if childPID > 0 {
					if proc, procErr := os.FindProcess(childPID); procErr == nil {
						_ = proc.Signal(syscall.SIGTERM)
						dlog.Info("sent SIGTERM to child process %d for checkpoint stop of %s", childPID, taskLabel)
					}
				}
				// Wait for grace period, then escalate
				gracePeriod := rt.CheckpointGracePeriod
				if gracePeriod <= 0 {
					gracePeriod = 30 * time.Second
				}
				select {
				case <-runnerDone:
					// Child exited within grace period
				case <-time.After(gracePeriod):
					// Grace period expired, force-kill
					graceExpired = true
					dlog.Warn("grace period expired for %s, force-killing", taskLabel)
					cancel()
				}
			case <-runnerDone:
				// Runner completed normally, no graceful stop needed
			}
		}()

		usedSessionID, logPath, usedRunnerIdx, stderrOutput, err = d.runner.Run(ctx, proj.Path, sessionToResume, resume, t.SkipPermissions, t.AllowedTools, taskContent, taskLabel, logDir, skipIndices, mergedEnv, t.RunnerChain, t.RunnerOnTimeout, func(pid int, lp string, sid string) {
			childPID = pid
			d.tasksMu.Lock()
			if task, ok := d.tasks[instanceKey]; ok {
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
			if task, ok := d.tasks[instanceKey]; ok {
				task.Status = status
			}
			d.tasksMu.Unlock()
		}, func(data string) {
			if t.Checkpoint {
				checkpointMu.Lock()
				lastCheckpointData = data
				checkpointMu.Unlock()
			}
			// Adaptive timeout: extend deadline when checkpoint data arrives
			if deadlineTimer != nil && t.AdaptiveTimeout != nil && t.AdaptiveTimeout.Enabled && t.AdaptiveTimeout.ExtendIf == "checkpoint_exists" {
				d.tasksMu.Lock()
				rt := d.tasks[instanceKey]
				canExtend := rt != nil && (t.AdaptiveTimeout.MaxExtensions == 0 || rt.TimeoutExtensions < t.AdaptiveTimeout.MaxExtensions)
				if canExtend {
					ext := t.AdaptiveTimeout.ExtensionDuration
					if ext == 0 {
						ext = rt.OriginalTimeout
					}
					rt.TimeoutExtensions++
					rt.Timeout += ext
					elapsed := time.Since(rt.Started)
					remaining := rt.Timeout - elapsed
					if remaining > 0 {
						deadlineTimer.Reset(remaining)
					}
					dlog.Info("adaptive timeout extended for %s/%s (extension %d, +%v)", filepath.Base(proj.Path), t.Name, rt.TimeoutExtensions, ext.Round(time.Second))
				}
				d.tasksMu.Unlock()
			}
		}, func(data string) {
			if t.CaptureOutput {
				const maxResultSize = 1 << 20 // 1MB
				if len(data) > maxResultSize {
					data = data[:maxResultSize]
					dlog.Warn("result data truncated to 1MB for %s", taskLabel)
				}
				resultMu.Lock()
				lastResultData = data
				resultMu.Unlock()
			}
		})
		close(runnerDone)

		// If graceful stop was triggered, exit retry loop immediately (don't retry)
		if gracefulStopTriggered {
			break
		}

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

		// Check max_total_time before retrying
		if t.RetryMaxTime > 0 && time.Since(retryStartTime) >= t.RetryMaxTime {
			dlog.Info("retry time budget exhausted for %s after %v (max %v)", taskLabel, time.Since(retryStartTime).Round(time.Second), t.RetryMaxTime)
			break
		}

		// Calculate backoff using configured strategy
		strategy := t.RetryStrategy
		if strategy == "" {
			strategy = "exponential"
		}
		backoffDuration := calculateBackoff(strategy, retryDelay, attempt, t.RetryJitter)
		retryDelaysUsed = append(retryDelaysUsed, backoffDuration.Round(time.Millisecond).String())

		jitterTag := ""
		if t.RetryJitter > 0 {
			jitterTag = " +jitter"
		}
		dlog.Info("retry %d/%d for %s after %v (%s backoff %v%s)", attempt+1, t.Retry, taskLabel, err, strategy, backoffDuration.Round(time.Millisecond), jitterTag)

		// Wait for backoff duration, but respect context cancellation
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break
		case <-time.After(backoffDuration):
			// Continue to next retry attempt
		}
	}

skipExecution:

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

	// Cache the output if caching is enabled and the task succeeded
	if cache.IsCacheEnabled(t) && err == nil {
		cacheKey, calcErr := cache.CalculateCacheKey(t, proj.Path)
		if calcErr == nil {
			ttl, ttlErr := cache.GetCacheTTL(t)
			if ttlErr == nil {
				cacheErr := cache.PutCache(proj.Path, cacheKey, stderrOutput, ttl)
				if cacheErr != nil {
					dlog.Warn("failed to cache output for %s: %v", filepath.Base(proj.Path)+"/"+t.Name, cacheErr)
				} else {
					dlog.Info("cached output for %s (TTL: %v)", filepath.Base(proj.Path)+"/"+t.Name, ttl)
				}
			}
		}
	}

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

	resultMu.Lock()
	resData := lastResultData
	resultMu.Unlock()

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
		ResultData:       resData,
		Attempt:          finalAttempt + 1, // convert 0-based to 1-based
		MaxRetries:       t.Retry,
		RetryDelay:       retryDelay.String(),
		RetryStrategy:    t.RetryStrategy,
		RetryDelaysUsed:  retryDelaysUsed,
		ScheduledTime:    sla.ScheduledTime,
		DispatchDelay:    sla.Delay,
		SLAViolation:     sla.Violation,
		SLAMaxDelay:      sla.MaxDelay,
		RunnerIndex:      usedRunnerIdx,
	}
	// T18: Set NodeID for cluster tracking
	if d.clusterNode != nil {
		runRecord.NodeID = d.clusterNode.ID()
	}

	// Resolve which runner command was actually used for the run record
	{
		commands := d.runner.Commands
		if len(t.RunnerChain) > 0 {
			commands = t.RunnerChain
		}
		idx := usedRunnerIdx
		if idx >= 100 {
			// 100+ indicates timeout fallback runner
			runRecord.RunnerCommand = t.RunnerOnTimeout
		} else if idx >= 0 && idx < len(commands) {
			runRecord.RunnerCommand = commands[idx]
		}
	}

	// Evaluate assertions if configured and task succeeded
	if t.Assert != nil && err == nil {
		// Read stdout from log file if available
		var stdoutContent string
		if logPath != "" {
			if logData, readErr := os.ReadFile(logPath); readErr == nil {
				stdoutContent = string(logData)
			}
		}

		// Evaluate assertions
		assertionResults, assertErr := runner.EvaluateAssertions(&t, stdoutContent, stderrOutput)
		if assertErr != nil {
			dlog.Warn("assertion evaluation failed for %s: %v", t.Name, assertErr)
		} else if assertionResults.Failed {
			// Assertion failed - mark task as failed
			err = fmt.Errorf("assertion failed")
			runRecord.Error = "assertion failed"
			runRecord.Success = false

			// Log specific assertion failures
			for _, result := range assertionResults.Results {
				if !result.Passed {
					if result.Error != nil {
						dlog.Warn("assertion error for %s: %v", t.Name, result.Error)
					} else {
						dlog.Warn("assertion failed for %s: %s", t.Name, result.Message)
					}
				}
			}
		} else if len(assertionResults.Results) > 0 {
			// Log successful assertions or warnings for soft assertions
			for _, result := range assertionResults.Results {
				if !result.Passed && t.Assert.Soft {
					dlog.Warn("soft assertion failed for %s: %s", t.Name, result.Message)
				}
			}
		}
	}

	if gracefulStopTriggered {
		if graceExpired {
			runRecord.Error = "killed-after-grace-period"
			runRecord.Success = false
		} else {
			runRecord.Error = "stopped-with-checkpoint"
			runRecord.Success = false
		}
	} else if err != nil {
		runRecord.Error = err.Error()
	}

	// Capture output summary from the runner log file
	if logPath != "" {
		if summary, sumErr := captureOutputSummary(logPath); sumErr == nil {
			runRecord.OutputSummary = summary
		}
	}


	// Write run record
	if writeErr := project.WriteRunRecord(proj.Path, runRecord); writeErr != nil {
		dlog.Warn("failed to write run record for %s: %v", t.Name, writeErr)
	}

	// Save snapshot for replay-enabled tasks on success
	if t.Replay && err == nil {
		mergedEnv := mergeEnv(d.config.Env, t.Env)
		if snapErr := project.WriteSnapshot(proj.Path, t.ID, runID, t, mergedEnv, t.Content, proj.Path, runRecord); snapErr != nil {
			dlog.Warn("failed to write replay snapshot for %s: %v", t.Name, snapErr)
		} else {
			dlog.Info("saved replay snapshot for %s (run %s)", t.Name, runID[:min(8, len(runID))])
		}
	}

	alertCheck := checkAlerts(t, runRecord, d.config.Alerts)
	if len(alertCheck.AlertsFired) > 0 {
		dlog.Info("alerts triggered for task %s: %d alert(s)", t.Name, len(alertCheck.AlertsFired))
		alertStorage := NewAlertStorage(filepath.Join(proj.Path, ".anvil", "alerts"))
		for _, alert := range alertCheck.AlertsFired {
			if err := alertStorage.SaveAlert(t.Name, alert); err != nil {
				dlog.Warn("failed to save alert for %s: %v", t.Name, err)
			}
			// Execute alert actions asynchronously
			go ExecuteAlertAction(alert, getAlertAction(t, alert.RuleName), d.config.Alerts.DefaultWebhook)
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
	// Detect timeout: either native context.DeadlineExceeded or adaptive timeout timer fired
	isTimedOut := ctx.Err() == context.DeadlineExceeded || atomic.LoadInt32(&adaptiveTimedOut) == 1
	// Detect force-cycle: persistent task hit its max runtime (context deadline exceeded)
	forceCycled := t.IsPersistent() && err != nil && isTimedOut
	whPayload := webhook.BuildPayload(t.Name, proj.Path, runID, startTime, runRecord.Finished, runRecord.EstimatedCostUSD, runRecord.Error)

	// Record circuit breaker state for this task run
	if t.CircuitBreaker.Failures > 0 || d.config.CircuitBreaker.DefaultFailures > 0 {
		record, cbErr := d.circuitBreakerStorage.LoadCircuit(t.ID)
		if cbErr == nil {
			now := time.Now()
			if err != nil {
				// Record failure
				oldState := record.State
				recordFailure(t, record, now, d.config.CircuitBreaker)
				newState := record.State

				// Save updated circuit state
				if saveErr := d.circuitBreakerStorage.SaveCircuit(t.ID, *record); saveErr != nil {
					dlog.Warn("failed to save circuit breaker state for %s: %v", t.Name, saveErr)
				}

				// Execute on_circuit_open hook if circuit just opened
				if oldState != Open && newState == Open {
					go runCircuitOpenHook(t, record)
				}
			} else {
				// Record success
				oldState := record.State
				recordSuccess(t, record, now, d.config.CircuitBreaker)
				newState := record.State

				// Save updated circuit state
				if saveErr := d.circuitBreakerStorage.SaveCircuit(t.ID, *record); saveErr != nil {
					dlog.Warn("failed to save circuit breaker state for %s: %v", t.Name, saveErr)
				}

				// Execute on_circuit_close hook if circuit just closed
				if oldState == Open && newState == Closed {
					go runCircuitCloseHook(t, record)
				}
			}
		}
	}

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
		// Run on_timeout hook if defined and task timed out
		if isTimedOut && t.OnTimeout != "" {
			d.runTimeoutHook("on_timeout", t.OnTimeout, proj.Path, t, timeout, 0)
		}
		// Run on_failure hook if defined
		if t.OnFailure != "" {
			d.runHook("on_failure", t.OnFailure, proj.Path, t, logPath, usedSessionID, startTime, elapsed, finalAttempt+1, t.Retry, retryDelay)
		}
		// Fire failure webhook (use timeout event if deadline exceeded)
		whEvent := webhook.EventFailure
		if isTimedOut {
			whEvent = webhook.EventTimeout
		}
		d.webhooks.Fire(whEvent, whPayload)
		if t.Webhook != "" {
			d.webhooks.FireURL(t.Webhook, whEvent, whPayload)
		}
		// Send desktop notification on failure
		if shouldNotifyFailure(d.config.Notifications, t.NotifyOnFailure) {
			go sendNotification(d.config.Notifications, "anvil: task failed", fmt.Sprintf("%s/%s failed after %s", projName, t.Name, elapsed.Round(time.Second)))
		}
		// For persistent tasks, track failures and apply exponential backoff
		if t.IsPersistent() {
			d.persistentFailuresMu.Lock()
			d.persistentFailures[taskKey]++
			failCount := d.persistentFailures[taskKey]
			d.persistentFailuresMu.Unlock()
			// Calculate backoff using configured strategy (default exponential)
			strategy := t.RetryStrategy
			if strategy == "" {
				strategy = "exponential"
			}
			backoffDuration := calculateBackoff(strategy, t.RetryDelay, failCount-1, t.RetryJitter)
			d.persistentCooldownsMu.Lock()
			d.persistentCooldowns[taskKey] = time.Now().Add(backoffDuration)
			d.persistentCooldownsMu.Unlock()
			dlog.Info("persistent task %s failed (attempt %d) — backing off for %v (%s)", t.Name, failCount, backoffDuration.Round(time.Millisecond), strategy)
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
		// Send desktop notification on success
		if shouldNotifySuccess(d.config.Notifications, t.NotifyOnSuccess) {
			go sendNotification(d.config.Notifications, "anvil: task completed", fmt.Sprintf("%s/%s completed in %s", projName, t.Name, elapsed.Round(time.Second)))
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

	// Accumulate cost budget usage for tasks with cost budgets
	if t.CostBudget > 0 {
		d.costBudgetUsedMu.Lock()
		d.costBudgetUsed[taskKey] += runRecord.EstimatedCostUSD
		costUsed := d.costBudgetUsed[taskKey]
		d.costBudgetUsedMu.Unlock()
		// Emit warning when cost budget is approaching limit
		if t.CostBudget > 0 {
			remaining := t.CostBudget - costUsed
			pctUsed := costUsed / t.CostBudget * 100
			if pctUsed >= 80 && remaining > 0 {
				dlog.Warn("##anvil:status Cost budget low for %s: $%.2f remaining (%.0f%% of $%.2f used)", t.Name, remaining, pctUsed, t.CostBudget)
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

	// If state was configured, save the state file back to persistent storage
	if stateFilePath != "" && stateBucket != "" && stateKey != "" {
		if stateData, err := os.ReadFile(stateFilePath); err == nil {
			var state map[string]interface{}
			if err := json.Unmarshal(stateData, &state); err == nil {
				if err := project.WriteTaskState(proj.Path, stateBucket, stateKey, state); err != nil {
					dlog.Warn("failed to save state for %s: %v", t.Name, err)
				}
			}
		}
	}

	if writeErr := project.WriteRunRecord(proj.Path, runRecord); writeErr != nil {
		dlog.Warn("failed to write run record for %s: %v", t.Name, writeErr)
	}


	// Evaluate alert rules and execute actions for fired alerts
	if len(t.Alerts.Rules) > 0 {
		alertResult := checkAlerts(t, runRecord, d.config.Alerts)
		if len(alertResult.AlertsFired) > 0 {
			alertStorage := NewAlertStorage(filepath.Join(proj.Path, ".anvil", "alerts"))
			for _, alert := range alertResult.AlertsFired {
				if err := alertStorage.SaveAlert(t.Name, alert); err != nil {
					dlog.Warn("failed to save alert for %s: %v", t.Name, err)
				}
				// Execute alert actions (async)
				for _, rule := range t.Alerts.Rules {
					if rule.Name == alert.RuleName {
						go ExecuteAlertAction(alert, rule.Action, d.config.Alerts.DefaultWebhook)
						break
					}
				}
			}
			dlog.Info("alerts fired for %s: %d alerts", t.Name, len(alertResult.AlertsFired))
		}
	}

	// Log run activity
	runDetails := map[string]string{
		"run_id":    runID,
		"success":   fmt.Sprintf("%t", runRecord.Success),
		"duration":  elapsed.Round(time.Second).String(),
	}
	if runRecord.Error != "" {
		runDetails["error"] = runRecord.Error
	}
	project.WriteActivity(proj.Path, project.ActivityEntry{
		Timestamp: time.Now(),
		Action:    "run",
		TaskID:    t.ID,
		TaskName:  t.Name,
		Details:   runDetails,
	})
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
		"ANVIL_RETRY_STRATEGY="+t.RetryStrategy,
		"ANVIL_WILL_RETRY="+willRetry,
	)

	if hookErr := hookCmd.Run(); hookErr != nil {
		dlog.Warn("%s hook failed for %s: %v", hookName, t.Name, hookErr)
	} else {
		dlog.Info("%s hook completed for %s", hookName, t.Name)
	}
}

// runTimeoutHook executes a timeout-related hook (on_timeout_warning or on_timeout).
// Hook errors are logged as warnings but do not affect the task outcome.
func (d *Daemon) runTimeoutHook(hookName, command, projectPath string, t project.Todo, timeout, remaining time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	hookCmd := exec.CommandContext(ctx, "sh", "-c", command)
	hookCmd.Dir = projectPath

	hookCmd.Env = append(os.Environ(),
		"ANVIL_TASK_NAME="+t.Name,
		"ANVIL_PROJECT="+projectPath,
		"ANVIL_TIMEOUT="+timeout.String(),
		"ANVIL_TIMEOUT_REMAINING="+remaining.String(),
	)

	if hookErr := hookCmd.Run(); hookErr != nil {
		dlog.Warn("%s hook failed for %s: %v", hookName, t.Name, hookErr)
	} else {
		dlog.Info("%s hook completed for %s", hookName, t.Name)
	}
}

// runSLAViolationHook executes an on_sla_violation hook as a shell command.
// Hook errors are logged as warnings but do not affect task dispatch.
func (d *Daemon) runSLAViolationHook(t project.Todo, projectPath string, sla slaResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	hookCmd := exec.CommandContext(ctx, "sh", "-c", t.OnSLAViolation)
	hookCmd.Dir = projectPath

	hookCmd.Env = append(os.Environ(),
		"ANVIL_TASK_NAME="+t.Name,
		"ANVIL_PROJECT="+projectPath,
		"ANVIL_SLA_SCHEDULED_TIME="+sla.ScheduledTime.Format(time.RFC3339),
		"ANVIL_SLA_ACTUAL_TIME="+time.Now().Format(time.RFC3339),
		"ANVIL_SLA_DELAY="+sla.Delay.String(),
		"ANVIL_SLA_MAX_DELAY="+sla.MaxDelay.String(),
	)

	if hookErr := hookCmd.Run(); hookErr != nil {
		dlog.Warn("on_sla_violation hook failed for %s: %v", t.Name, hookErr)
	} else {
		dlog.Info("on_sla_violation hook completed for %s", t.Name)
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
	mux.HandleFunc("/rate-limits", d.handleRateLimits)
	mux.HandleFunc("/reset-rate-limits", d.handleResetRateLimits)
	mux.HandleFunc("/cluster/status", d.handleClusterStatus)
	mux.HandleFunc("/cluster/leave", d.handleClusterLeave)
	mux.HandleFunc("/throttle/pause", d.handleThrottlePause)
	mux.HandleFunc("/throttle/resume", d.handleThrottleResume)
	mux.HandleFunc("/throttle/rate", d.handleThrottleRate)
	mux.HandleFunc("/throttle/state", d.handleThrottleState)

	d.httpServer = &http.Server{
		Handler: mux,
	}

	dlog.SocketListening(d.socketPath)
	if err := d.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		dlog.SocketError(err)
	}
}

// handleClusterStatus returns the current cluster leadership status.
func (d *Daemon) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if d.clusterNode == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"enabled": false, "error": "cluster mode not enabled"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.clusterNode.Status())
}

// handleClusterLeave gracefully removes this node from the cluster.
func (d *Daemon) handleClusterLeave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if d.clusterNode == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"left": false, "error": "cluster mode not enabled"})
		return
	}
	nodeID := d.clusterNode.ID()
	d.clusterNode.Stop()
	d.clusterNode = nil
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"left": true, "node_id": nodeID})
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
			Project:           task.Project,
			Name:              task.Name,
			PID:               task.PID,
			Started:           task.Started.Format(time.RFC3339),
			Elapsed:           elapsed.Round(time.Second).String(),
			Timeout:           timeout.String(),
			TimeRemaining:     (timeout - elapsed).String(),
			PercentUsed:       elapsed.Round(time.Second).Seconds() / timeout.Seconds() * 100,
			LogPath:           task.LogPath,
			SessionID:         task.SessionID,
			Status:            task.Status,
			TimeoutExtensions: task.TimeoutExtensions,
			WarningTriggered:  task.WarningTriggered,
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
			Project:           task.Project,
			Name:              task.Name,
			PID:               task.PID,
			Started:           task.Started.Format(time.RFC3339),
			Elapsed:           elapsed.Round(time.Second).String(),
			Timeout:           timeout.String(),
			TimeRemaining:     (timeout - elapsed).String(),
			PercentUsed:       elapsed.Round(time.Second).Seconds() / timeout.Seconds() * 100,
			LogPath:           task.LogPath,
			SessionID:         task.SessionID,
			TimeoutExtensions: task.TimeoutExtensions,
			WarningTriggered:  task.WarningTriggered,
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

		// Add cascade information if this task was skipped due to dependency failure
		if strings.HasPrefix(skipReason, "dependency failed: ") {
			// Extract the dependency name from the skip reason
			depName := strings.TrimPrefix(skipReason, "dependency failed: ")
			d.cascadeFailuresMu.Lock()
			if affected, exists := d.cascadeFailures[depName]; exists && len(affected) > 0 {
				// Count how many tasks are affected by this dependency failure
				cascadeCount := len(affected)
				if cascadeCount > 0 {
					// Update the last added TaskQueueInfo with cascade info
					lastIndex := len(result) - 1
					result[lastIndex].CascadeCount = cascadeCount
					result[lastIndex].CascadeFrom = depName
				}
			}
			d.cascadeFailuresMu.Unlock()
		}
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

	// Create a combined result with both time and cost budgets
	result := make(map[string]interface{})

	// Add persistent time budget usage
	d.persistentBudgetUsedMu.Lock()
	timeBudgets := make(map[string]float64, len(d.persistentBudgetUsed))
	for k, v := range d.persistentBudgetUsed {
		timeBudgets[k] = v.Seconds()
	}
	d.persistentBudgetUsedMu.Unlock()
	result["time_budgets"] = timeBudgets

	// Add cost budget usage
	d.costBudgetUsedMu.Lock()
	costBudgets := make(map[string]float64, len(d.costBudgetUsed))
	for k, v := range d.costBudgetUsed {
		costBudgets[k] = v
	}
	d.costBudgetUsedMu.Unlock()
	result["cost_budgets"] = costBudgets

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

	// Reset persistent time budget
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

	// Reset cost budget as well
	d.costBudgetUsedMu.Lock()
	d.costBudgetUsed[req.TaskKey] = 0
	d.costBudgetUsedMu.Unlock()

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

// RateLimitStatus represents the rate limit status for a task
type RateLimitStatus struct {
	TaskName  string `json:"task_name"`
	ThisHour  string `json:"this_hour"`
	HourLimit string `json:"hour_limit"`
	ThisDay   string `json:"this_day"`
	DayLimit  string `json:"day_limit"`
}

// handleRateLimits returns rate limit status for all tasks
func (d *Daemon) handleRateLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.rateLimitCountersMu.Lock()
	defer d.rateLimitCountersMu.Unlock()

	var statuses []RateLimitStatus

	// Get all watched projects to find tasks
	paths := loadWatchedPaths()
	now := time.Now()
	hourKey := now.Format("2006-01-02 15")
	dayKey := now.Format("2006-01-02")

	for _, path := range paths {
		proj, err := project.Load(path)
		if err != nil {
			continue
		}

		todos, err := proj.LoadTodos()
		if err != nil {
			continue
		}

		for _, todo := range todos {
			if todo.RateLimit.MaxPerHour == 0 && todo.RateLimit.MaxPerDay == 0 {
				continue // Skip tasks without rate limits
			}

			status := RateLimitStatus{
				TaskName: strings.TrimSuffix(todo.Name, ".md"),
			}

			// Get counter for this task
			counter, exists := d.rateLimitCounters[todo.ID]
			if !exists {
				counter = &RateLimitCounter{
					Hourly: make(map[string]int),
					Daily:  make(map[string]int),
				}
			}

			// Format hourly status
			thisHour := counter.Hourly[hourKey]
			if todo.RateLimit.MaxPerHour > 0 {
				status.ThisHour = fmt.Sprintf("%d/%d", thisHour, todo.RateLimit.MaxPerHour)
				status.HourLimit = fmt.Sprintf("%.0f%%", float64(thisHour)/float64(todo.RateLimit.MaxPerHour)*100)
			} else {
				status.ThisHour = fmt.Sprintf("%d/unlimited", thisHour)
				status.HourLimit = "0%"
			}

			// Format daily status
			thisDay := counter.Daily[dayKey]
			if todo.RateLimit.MaxPerDay > 0 {
				status.ThisDay = fmt.Sprintf("%d/%d", thisDay, todo.RateLimit.MaxPerDay)
				status.DayLimit = fmt.Sprintf("%.0f%%", float64(thisDay)/float64(todo.RateLimit.MaxPerDay)*100)
			} else {
				status.ThisDay = fmt.Sprintf("%d/unlimited", thisDay)
				status.DayLimit = "0%"
			}

			statuses = append(statuses, status)
		}
	}

	json.NewEncoder(w).Encode(statuses)
}

// handleResetRateLimits resets all rate limit counters
func (d *Daemon) handleResetRateLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	d.rateLimitCountersMu.Lock()
	defer d.rateLimitCountersMu.Unlock()

	// Clear all counters
	d.rateLimitCounters = make(map[string]*RateLimitCounter)

	fmt.Fprintf(w, "rate limit counters reset")
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

	if req.Checkpoint {
		// Validate checkpoint preconditions
		if !found.CheckpointEnabled {
			http.Error(w, "task does not have checkpoint enabled", http.StatusBadRequest)
			return
		}
		if found.ShuttingDown {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "already-shutting-down", "name": found.Name})
			return
		}

		// Signal graceful shutdown
		found.ShuttingDown = true
		close(found.GracefulStop)

		// Log checkpoint kill activity
		project.WriteActivity(found.Project, project.ActivityEntry{
			Timestamp: time.Now(),
			Action:    "killed",
			TaskID:    found.TaskID,
			TaskName:  found.Name,
			Details:   map[string]string{"method": "checkpoint"},
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "stopping", "name": found.Name, "checkpoint": "true"})
		return
	}

	// Cancel the task's context
	found.Cancel()

	// Log kill activity
	project.WriteActivity(found.Project, project.ActivityEntry{
		Timestamp: time.Now(),
		Action:    "killed",
		TaskID:    found.TaskID,
		TaskName:  found.Name,
		Details:   map[string]string{"method": "cancel"},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "killed", "name": found.Name})
}

// DaemonStatus holds runtime state about the daemon.
type DaemonStatus struct {
	Draining       bool `json:"draining"`
	RateLimited    bool `json:"rate_limited"`   // true if rate limiting is configured
	RateLimitSlots int  `json:"rate_limit_slots"` // total slots configured (0 if not limited)
	RateInUse      int  `json:"rate_in_use"`    // current slots in use
	// Throttle state
	Paused       bool            `json:"paused"`                  // global pause active
	ThrottleRate int             `json:"throttle_rate,omitempty"`  // tasks per minute (0 = unlimited)
	PausedLabels []string        `json:"paused_labels,omitempty"`  // labels currently paused
}

func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := DaemonStatus{
		Draining: atomic.LoadInt32(&d.draining) == 1,
	}
	// Populate rate limit status if configured
	if d.rateLimitSemaphore != nil {
		status.RateLimited = true
		status.RateLimitSlots = cap(d.rateLimitSemaphore)
		// Count slots currently in use (len of channel)
		status.RateInUse = len(d.rateLimitSemaphore)
	}
	// Populate throttle state
	ts := d.throttle.GetState()
	status.Paused = ts.Paused
	status.ThrottleRate = ts.RatePerMin
	for label := range ts.PausedLabels {
		status.PausedLabels = append(status.PausedLabels, label)
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
	TaskHealth       []*TaskHealth     `json:"task_health,omitempty"`
	Detailed         bool              `json:"-"`
}

// TaskHealth represents the health status of a single task
type TaskHealth struct {
	Name       string    `json:"name"`
	Healthy    bool      `json:"healthy"`
	LastCheck  time.Time `json:"last_check"`
	ExitCode   int       `json:"exit_code"`
	Error      string    `json:"error,omitempty"`
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

	// Include task health if configured
	if d.config.Health.IncludeTasks {
		// Get all task health statuses
		taskHealthMap := d.healthManager.GetAllHealthStatus()

		// Convert to slice for JSON serialization
		var taskHealth []*TaskHealth
		unhealthyCount := 0

		for taskName, healthStatus := range taskHealthMap {
			taskHealthItem := &TaskHealth{
				Name:       taskName,
				Healthy:    healthStatus.Healthy,
				LastCheck:  healthStatus.LastCheck,
				ExitCode:   healthStatus.ExitCode,
				Error:      healthStatus.Error,
			}
			taskHealth = append(taskHealth, taskHealthItem)

			if !healthStatus.Healthy {
				unhealthyCount++
			}
		}

		// Add task health to response
		resp.TaskHealth = taskHealth

		// Override daemon health if unhealthy threshold is exceeded
		if d.config.Health.UnhealthyThreshold > 0 && unhealthyCount >= d.config.Health.UnhealthyThreshold {
			resp.Healthy = false
			if resp.Components == nil {
				resp.Components = make(map[string]string)
			}
			resp.Components["task_health"] = fmt.Sprintf("unhealthy tasks: %d (threshold: %d)", unhealthyCount, d.config.Health.UnhealthyThreshold)
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

func (d *Daemon) handleThrottlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ThrottlePauseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	var err error
	if req.Label != "" {
		err = d.throttle.PauseLabel(req.Label)
		if err == nil {
			dlog.Info("throttle: paused tasks with label %q", req.Label)
		}
	} else {
		err = d.throttle.SetPaused(true)
		if err == nil {
			dlog.Info("throttle: global pause activated")
		}
	}
	if err != nil {
		http.Error(w, "failed to save throttle state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (d *Daemon) handleThrottleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ThrottleResumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	var err error
	if req.Label != "" {
		err = d.throttle.ResumeLabel(req.Label)
		if err == nil {
			dlog.Info("throttle: resumed tasks with label %q", req.Label)
		}
	} else {
		err = d.throttle.SetPaused(false)
		if err == nil {
			dlog.Info("throttle: global pause deactivated")
		}
	}
	if err != nil {
		http.Error(w, "failed to save throttle state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (d *Daemon) handleThrottleRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ThrottleRateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := d.throttle.SetRate(req.RatePerMin); err != nil {
		http.Error(w, "failed to save throttle state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if req.RatePerMin > 0 {
		dlog.Info("throttle: rate set to %d/m", req.RatePerMin)
	} else {
		dlog.Info("throttle: rate limit disabled")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (d *Daemon) handleThrottleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.throttle.GetState())
}

// RunRequest is the JSON payload for /run (force-trigger a task).
type RunRequest struct {
	ProjectPath string `json:"project_path"`
	TaskID      string `json:"task_id"`
	TaskName    string `json:"task_name"`
	Force       bool   `json:"force,omitempty"`
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
	if req.Force {
		todo.ForceWindow = true // Bypass time window and quiet hours checks
	}

	taskKey := fmt.Sprintf("%s/%s", proj.Path, todo.Name)

	d.inFlightMu.Lock()
	d.inFlight[taskKey]++
	d.inFlightMu.Unlock()

	projName := filepath.Base(proj.Path)
	dlog.Info("force-run requested for %s/%s — dispatching immediately", projName, todo.Name)

	// Log force-run activity
	project.WriteActivity(proj.Path, project.ActivityEntry{
		Timestamp: time.Now(),
		Action:    "force-run",
		TaskID:    todo.ID,
		TaskName:  todo.Name,
	})

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

	// Check for timed-out processes on every tick
	d.checkForTimedOutProcesses(now)

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

	// T20: Skip scheduling if cluster enabled and not leader
	if d.clusterNode != nil && !d.clusterNode.IsLeader() {
		return
	}

	// Skip scheduling if globally paused
	if d.throttle.IsPaused() {
		dlog.Info("skip scheduling — globally paused")
		return
	}

	// Collect all due todos across all projects for global priority ordering
	type projectTodo struct {
		proj *project.Project
		todo project.Todo
	}
	var dueTodos []projectTodo
	totalTodos := 0
	totalMatched := 0

	// Load all todos for cross-project cycle detection and validation
	allProjectTodos := make(map[string][]project.Todo)
	for _, proj := range projects {
		projName := filepath.Base(proj.Path)
		allTodos, err := proj.LoadTodos()
		if err != nil {
			dlog.Warn("%s: error loading todos: %v", projName, err)
			continue
		}
		allProjectTodos[projName] = allTodos

		// Auto-version task files that have changed
		for _, t := range allTodos {
			taskName := strings.TrimSuffix(t.Name, ".md")
			content, err := os.ReadFile(t.Path)
			if err != nil {
				continue
			}
			hash := project.ComputeFileHash(string(content))
			prevHash, known := d.taskHashes[proj.Path+"/"+taskName]
			if !known {
				// First time seeing this task - check if a version already exists
				existing, _ := project.ReadAllVersions(proj.Path, taskName)
				if len(existing) == 0 {
					// No versions exist yet - create initial version
					author := project.GetAuthor(proj.Path)
					project.WriteTaskVersion(proj.Path, taskName, string(content), author, "initial version")
				}
				d.taskHashes[proj.Path+"/"+taskName] = hash
			} else if hash != prevHash {
				// Content changed - create new version
				author := project.GetAuthor(proj.Path)
				project.WriteTaskVersion(proj.Path, taskName, string(content), author, "")
				d.taskHashes[proj.Path+"/"+taskName] = hash
			}
		}

		// Validate cross-project dependencies
		for _, t := range allTodos {
			for _, dep := range t.DependsOn {
				parsed := project.ParseDependency(dep)
				if !parsed.IsLocal {
					if err := project.ValidateDependency(parsed, proj.Path); err != nil {
						dlog.Warn("%s/%s: invalid cross-project dependency %q: %v", projName, t.Name, dep, err)
					}
				}
			}
		}
	}

	// Detect cross-project circular dependencies
	if hasCycle, cyclePath := project.DetectCrossProjectCycles(allProjectTodos); hasCycle {
		dlog.Warn("cross-project circular dependency detected: %s", strings.Join(cyclePath, " -> "))
	}

	for _, proj := range projects {
		projName := filepath.Base(proj.Path)
		allTodos := allProjectTodos[projName]
		if allTodos == nil {
			continue
		}
		totalTodos += len(allTodos)

		for _, t := range allTodos {
			// Skip disabled tasks silently
			if t.Disabled {
				continue
			}
			// Skip tasks with a paused label
			if len(t.Labels) > 0 && d.throttle.IsLabelPaused(t.Labels) {
				continue
			}
			// Include task if:
			// - one-shot (empty schedule)
			// - cron schedule matches current minute
			// - persistent task (run immediately on tick)
			// - backfill is enabled and there are missed runs within max_delay
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
				// Track queue time for priority aging
				taskKey := fmt.Sprintf("%s/%s", proj.Path, t.Name)
				d.priorityAgingQueueTimesMu.Lock()
				if _, exists := d.priorityAgingQueueTimes[taskKey]; !exists {
					d.priorityAgingQueueTimes[taskKey] = time.Now()
				}
				d.priorityAgingQueueTimesMu.Unlock()

				dueTodos = append(dueTodos, projectTodo{proj, t})
				totalMatched++
			} else if t.Backfill != nil && t.Backfill.Enabled {
				// Check for backfill opportunities
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
					if err == nil {
						// Count missed runs between last run and now
						missedCount, err := parser.CountMissed(lastRunTime, thisMinute)
						if err == nil && missedCount > 0 {
							// There are missed runs, check if within max_delay
							// For simplicity, we'll check if the most recent missed run is within max_delay
							// Find the previous scheduled time before now
							prevScheduled, err := parser.Prev(thisMinute)
							if err == nil && !prevScheduled.IsZero() {
								delay := thisMinute.Sub(prevScheduled)
								if t.Backfill.MaxDelay == 0 || delay <= t.Backfill.MaxDelay {
									// Within backfill window, add to due todos
									// Track queue time for priority aging
									taskKey := fmt.Sprintf("%s/%s", proj.Path, t.Name)
									d.priorityAgingQueueTimesMu.Lock()
									if _, exists := d.priorityAgingQueueTimes[taskKey]; !exists {
										d.priorityAgingQueueTimes[taskKey] = time.Now()
									}
									d.priorityAgingQueueTimesMu.Unlock()

									dueTodos = append(dueTodos, projectTodo{proj, t})
									totalMatched++
								}
							}
						}
					}
				}
			}
		}
	}

	// Sort globally by priority (p0 first), then by name (oldest timestamp first)
	// With priority aging: boost priority based on wait time
	currentTime := time.Now()
	sort.SliceStable(dueTodos, func(i, j int) bool {
		// Calculate effective priority with aging
		getEffectivePriority := func(pt projectTodo) int {
			priority := pt.todo.Priority

			// Check if priority aging is enabled for this task or globally
			agingEnabled := false
			var maxWait time.Duration
			maxBoost := 0

			// Check task-level configuration
			if pt.todo.PriorityAging != nil && pt.todo.PriorityAging.Enabled != nil && *pt.todo.PriorityAging.Enabled {
				agingEnabled = true
				maxWait = pt.todo.PriorityAging.MaxWait
				maxBoost = pt.todo.PriorityAging.MaxBoost
			} else if pt.todo.PriorityAging != nil && pt.todo.PriorityAging.Enabled == nil {
				// Check project-level defaults
				// (Implementation would go here)
			} else if d.config.PriorityAging.Enabled {
				// Check global configuration
				agingEnabled = true
				maxWait = d.config.PriorityAging.DefaultMaxWait
				maxBoost = d.config.PriorityAging.DefaultMaxBoost
			}

			if agingEnabled {
				// Get queue time for this task
				taskKey := fmt.Sprintf("%s/%s", pt.proj.Path, pt.todo.Name)
				d.priorityAgingQueueTimesMu.Lock()
				queueTime, exists := d.priorityAgingQueueTimes[taskKey]
				d.priorityAgingQueueTimesMu.Unlock()

				if exists {
					waitTime := currentTime.Sub(queueTime)
					if waitTime > maxWait && maxWait > 0 {
						// Calculate boost level based on how much we've exceeded maxWait
						boostLevels := int(waitTime/maxWait)
						if boostLevels > maxBoost {
							boostLevels = maxBoost
						}
						// Apply boost (lower number means higher priority)
						priority -= boostLevels
						if priority < 0 {
							priority = 0
						}
					}
				}
			}

			return priority
		}

		effectivePriorityI := getEffectivePriority(dueTodos[i])
		effectivePriorityJ := getEffectivePriority(dueTodos[j])

		if effectivePriorityI != effectivePriorityJ {
			return effectivePriorityI < effectivePriorityJ
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

		// Check concurrency group limits if task belongs to a group
		if pt.todo.ConcurrencyGroup != "" {
			groupName := pt.todo.ConcurrencyGroup
			groupConfig, exists := d.config.ConcurrencyGroups[groupName]

			if exists {
				// Check group max_concurrent limit
				if groupConfig.MaxConcurrent > 0 {
					d.groupInFlightMu.Lock()
					groupInFlight := d.groupInFlight[groupName]
					d.groupInFlightMu.Unlock()

					if groupInFlight >= groupConfig.MaxConcurrent {
						dlog.Info("skip %s/%s — group %s at max concurrent (%d)", projName, pt.todo.Name, groupName, groupConfig.MaxConcurrent)
						d.pendingTasksMu.Lock()
						d.pendingTasks[taskKey] = fmt.Sprintf("group %s at max concurrent", groupName)
						d.pendingTasksMu.Unlock()
						continue
					}
				}

				// Check if respecting min_available would leave enough workers for non-group tasks
				if groupConfig.MinAvailable > 0 {
					// Get current in-flight counts
					d.inFlightMu.Lock()
					totalInFlight := 0
					for _, count := range d.inFlight {
						totalInFlight += count
					}
					d.inFlightMu.Unlock()

					d.groupInFlightMu.Lock()
					totalGroupInFlight := 0
					for _, count := range d.groupInFlight {
						totalGroupInFlight += count
					}
					d.groupInFlightMu.Unlock()

					// Calculate available workers
					availableWorkers := d.config.MaxWorkers - totalInFlight

					// Check if adding this task would violate min_available
					if availableWorkers-groupConfig.MinAvailable < 0 {
						dlog.Info("skip %s/%s — would violate min_available for non-group tasks", projName, pt.todo.Name)
						d.pendingTasksMu.Lock()
						d.pendingTasks[taskKey] = "would violate min_available for non-group tasks"
						d.pendingTasksMu.Unlock()
						continue
					}
				}

				// Check group rate limits if configured
				if groupConfig.RateLimit.RequestsPerMinute > 0 || groupConfig.RateLimit.TokenRateLimit > 0 {
					// For now, we'll skip the detailed rate limit implementation
					// This would need a more complex counter system similar to individual task rate limits
				}
			}
		}

		// Determine how many instances we can dispatch
		maxConcurrent := pt.todo.MaxConcurrent
		if maxConcurrent < 1 {
			maxConcurrent = 1
		}
		d.inFlightMu.Lock()
		currentInFlight := d.inFlight[taskKey]
		d.inFlightMu.Unlock()
		if currentInFlight >= maxConcurrent {
			dlog.Info("skip %s/%s — already in-flight", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "max concurrent"
			d.pendingTasksMu.Unlock()
			continue
		}

		// Check rate limits if configured
		if pt.todo.RateLimit.MaxPerHour > 0 || pt.todo.RateLimit.MaxPerDay > 0 {
			counter, err := project.ReadRateLimitCounter(pt.proj.Path, pt.todo.ID)
			if err != nil {
				dlog.Warn("failed to read rate limit counter for %s/%s: %v", projName, pt.todo.Name, err)
				// Continue with task execution if we can't read the counter
			} else {
				now := time.Now()
				skipReason := ""

				// Check hourly limit
				if pt.todo.RateLimit.MaxPerHour > 0 {
					// Reset hourly counter if needed
					if counter.ThisHourStart.Add(time.Hour).Before(now) {
						counter.ThisHourCount = 0
						counter.ThisHourStart = now.Truncate(time.Hour)
					}

					if counter.ThisHourCount >= pt.todo.RateLimit.MaxPerHour {
						skipReason = fmt.Sprintf("hourly limit reached (%d/%d)", counter.ThisHourCount, pt.todo.RateLimit.MaxPerHour)
					}
				}

				// Check daily limit
				if pt.todo.RateLimit.MaxPerDay > 0 && skipReason == "" {
					// Reset daily counter if needed
					if counter.ThisDayStart.Add(24 * time.Hour).Before(now) {
						counter.ThisDayCount = 0
						counter.ThisDayStart = now.Truncate(24 * time.Hour)
					}

					if counter.ThisDayCount >= pt.todo.RateLimit.MaxPerDay {
						skipReason = fmt.Sprintf("daily limit reached (%d/%d)", counter.ThisDayCount, pt.todo.RateLimit.MaxPerDay)
					}
				}

				if skipReason != "" {
					dlog.Info("skip %s/%s — %s", projName, pt.todo.Name, skipReason)
					d.pendingTasksMu.Lock()
					d.pendingTasks[taskKey] = skipReason
					d.pendingTasksMu.Unlock()
					// Release the in-flight slot we reserved above
					d.inFlightMu.Lock()
					d.inFlight[taskKey]--
					d.inFlightMu.Unlock()
					continue
				}
			}
		}

		instancesToDispatch := maxConcurrent - currentInFlight
		// Reserve one slot now so skip-check rollback stays simple
		d.inFlightMu.Lock()
		d.inFlight[taskKey]++
		d.inFlightMu.Unlock()

		// Skip if task has dependencies that haven't completed successfully
		// Unless the task is being force-run (ForceWindow is set)
		if len(pt.todo.DependsOn) > 0 && !pt.todo.ForceWindow {
			depMet, depInfo := checkDependenciesMet(pt.proj.Path, pt.todo.DependsOn, pt.todo.DependencyPolicy)
			if !depMet {
				reason := "dependency not met"
				if depInfo != nil {
					reason = depInfo.Reason
				}
				dlog.Info("skip %s/%s — %s", projName, pt.todo.Name, reason)
				d.pendingTasksMu.Lock()
				d.pendingTasks[taskKey] = reason
				d.pendingTasksMu.Unlock()

				// Fire skipped webhook for dependency failures
				if depInfo != nil && strings.HasPrefix(depInfo.Reason, "dependency failed") {
					skipPayload := webhook.BuildSkippedPayload(
						pt.todo.Name, pt.proj.Path, "dependency_failed",
						depInfo.DepName, depInfo.ExitCode, depInfo.Finished,
					)
					d.webhooks.Fire(webhook.EventSkipped, skipPayload)
					if pt.todo.Webhook != "" {
						d.webhooks.FireURL(pt.todo.Webhook, webhook.EventSkipped, skipPayload)
					}

					// Record cascading failure for reporting
					d.cascadeFailuresMu.Lock()
					d.cascadeFailures[depInfo.DepName] = append(d.cascadeFailures[depInfo.DepName], pt.todo.Name)
					d.cascadeFailuresMu.Unlock()
				}

				d.inFlightMu.Lock()
				d.inFlight[taskKey]--
				if d.inFlight[taskKey] <= 0 {
					delete(d.inFlight, taskKey)
				}
				d.inFlightMu.Unlock()
				continue
			}
		}

		// Register polling tasks instead of dispatching them directly
		if pt.todo.Trigger != nil && pt.todo.Trigger.PollingConfig != nil && pt.todo.Trigger.PollingConfig.Enabled {
			if d.pollingManager.Register(pt.proj, pt.todo) {
				d.inFlightMu.Lock()
				d.inFlight[taskKey]--
				if d.inFlight[taskKey] <= 0 {
					delete(d.inFlight, taskKey)
				}
				d.inFlightMu.Unlock()
				continue
			}
		}

		// Skip dispatch if trigger conditions are not met
		if pt.todo.Trigger != nil && len(pt.todo.Trigger.Conditions) > 0 && !pt.todo.ForceWindow {
			shouldTrigger, triggerErr := project.ShouldTriggerTask(context.Background(), pt.todo, *pt.todo.Trigger)
			if triggerErr != nil {
				dlog.Warn("trigger evaluation failed for %s/%s: %v", projName, pt.todo.Name, triggerErr)
			}
			if !shouldTrigger {
				reason := "trigger conditions not met"
				if triggerErr != nil {
					reason = fmt.Sprintf("trigger evaluation error: %v", triggerErr)
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

		// Skip dispatch if task is outside its allowed time window or in quiet hours
		if !isTaskInWindow(pt.todo, time.Now()) {
			dlog.Info("skip %s/%s — outside allowed time window", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "outside time window"
			d.pendingTasksMu.Unlock()
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
			continue
		}
		if isInQuietHours(time.Now(), d.config.QuietHours, pt.todo.Priority) {
			dlog.Info("skip %s/%s — quiet hours active", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "quiet hours"
			d.pendingTasksMu.Unlock()
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
			continue
		}

		// Check SLA: detect if task dispatch is delayed beyond threshold
		slaCheck := checkSLA(pt.todo, d.config.SLA, time.Now())
		if slaCheck.HasSLA && slaCheck.Violation && slaCheck.Strict {
			dlog.Info("skip %s/%s — SLA strict: %v late (max %v)", projName, pt.todo.Name, slaCheck.Delay.Round(time.Second), slaCheck.MaxDelay)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = fmt.Sprintf("SLA strict: %v late", slaCheck.Delay.Round(time.Second))
			d.pendingTasksMu.Unlock()
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
			// Fire SLA violation hook even for strict-skipped tasks
			if pt.todo.OnSLAViolation != "" {
				go d.runSLAViolationHook(pt.todo, pt.proj.Path, slaCheck)
			}
			continue
		}
		if slaCheck.HasSLA && slaCheck.Violation {
			dlog.Info("SLA violation %s/%s — %v late (max %v)", projName, pt.todo.Name, slaCheck.Delay.Round(time.Second), slaCheck.MaxDelay)
			if pt.todo.OnSLAViolation != "" {
				go d.runSLAViolationHook(pt.todo, pt.proj.Path, slaCheck)
			}
		}

		// Check circuit breaker: skip task if circuit is open
		if pt.todo.CircuitBreaker.Failures > 0 || d.config.CircuitBreaker.DefaultFailures > 0 {
			record, err := d.circuitBreakerStorage.LoadCircuit(pt.todo.ID)
			if err == nil {
				now := time.Now()
				if checkCircuit(pt.todo, record, now, d.config.CircuitBreaker) {
					dlog.Info("skip %s/%s — circuit breaker OPEN", projName, pt.todo.Name)
					d.pendingTasksMu.Lock()
					d.pendingTasks[taskKey] = "circuit breaker OPEN"
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

		// Skip dispatch if task has exhausted its cost budget
		if pt.todo.CostBudget > 0 {
			d.costBudgetUsedMu.Lock()
			costUsed := d.costBudgetUsed[taskKey]
			d.costBudgetUsedMu.Unlock()
			if costUsed >= pt.todo.CostBudget {
				dlog.Info("skip %s/%s — cost budget exhausted ($%.2f / $%.2f)", projName, pt.todo.Name, costUsed, pt.todo.CostBudget)
				d.pendingTasksMu.Lock()
				d.pendingTasks[taskKey] = "cost budget exhausted"
				d.pendingTasksMu.Unlock()
				d.inFlightMu.Lock()
				d.inFlight[taskKey]--
				if d.inFlight[taskKey] <= 0 {
					delete(d.inFlight, taskKey)
				}
				d.inFlightMu.Unlock()
				// Also mark as stopped so it won't be re-dispatched until reset
				d.stoppedMu.Lock()
				d.stoppedTasks[pt.todo.ID] = true
				d.stoppedMu.Unlock()
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

		// T16+T19+T20: Check for remote dispatch via cluster
		if d.clusterNode != nil && d.clusterNode.IsLeader() {
			assignedRemotely := false
			peerWorkers := d.clusterNode.PeerWorkers()

			// T19: Check node affinity
			if pt.todo.NodeAffinity != "" {
				// Must run on specific node
				if pt.todo.NodeAffinity == d.clusterNode.ID() {
					// T20: Affinity matches leader, dispatch locally
					// Fall through to local dispatch below
				} else {
					// Must run on a specific remote node
					for _, addr := range d.clusterNode.PeerAddresses() {
						// Try to send to any peer (we track by address, not ID currently)
						assignment := cluster.TaskAssignment{
							AssignmentID: fmt.Sprintf("%s-%d", pt.todo.Name, time.Now().UnixNano()),
							TaskName:     pt.todo.Name,
							TaskID:       pt.todo.ID,
							ProjectPath:  pt.proj.Path,
							Content:      pt.todo.Content,
							Runner:       pt.todo.Runner,
							RunnerChain:  pt.todo.RunnerChain,
							Timeout:      pt.todo.Timeout,
							Env:          pt.todo.Env,
							Priority:     pt.todo.Priority,
							Labels:       pt.todo.Labels,
							NodeAffinity: pt.todo.NodeAffinity,
							TargetNodeID: pt.todo.NodeAffinity,
						}
						if err := d.clusterNode.AssignTask(addr, assignment); err == nil {
							assignedRemotely = true
							break
						}
					}
					if !assignedRemotely {
						d.pendingTasksMu.Lock()
						d.pendingTasks[taskKey] = "affinity node unavailable"
						d.pendingTasksMu.Unlock()
						d.inFlightMu.Lock()
						d.inFlight[taskKey]--
						if d.inFlight[taskKey] <= 0 {
							delete(d.inFlight, taskKey)
						}
						d.inFlightMu.Unlock()
					}
					if assignedRemotely {
						dispatched++
					}
					continue
				}
			} else {
				// No affinity: find best node (most idle workers)
				bestAddr := ""
				bestIdle := 0
				for _, addr := range d.clusterNode.PeerAddresses() {
					// Look up idle count by iterating peerWorkers
					for nodeID, idle := range peerWorkers {
						_ = nodeID
						if idle > bestIdle {
							bestIdle = idle
							bestAddr = addr
						}
					}
				}

				// Check local capacity
				localIdle := cap(d.workQueue)/4 - len(d.workQueue)
				if bestIdle > localIdle && bestAddr != "" {
					assignment := cluster.TaskAssignment{
						AssignmentID: fmt.Sprintf("%s-%d", pt.todo.Name, time.Now().UnixNano()),
						TaskName:     pt.todo.Name,
						TaskID:       pt.todo.ID,
						ProjectPath:  pt.proj.Path,
						Content:      pt.todo.Content,
						Runner:       pt.todo.Runner,
						RunnerChain:  pt.todo.RunnerChain,
						Timeout:      pt.todo.Timeout,
						Env:          pt.todo.Env,
						Priority:     pt.todo.Priority,
						Labels:       pt.todo.Labels,
					}
					if err := d.clusterNode.AssignTask(bestAddr, assignment); err == nil {
						assignedRemotely = true
						dispatched++
					}
				}
			}
			if assignedRemotely {
				continue
			}
		}

		// Check rate limiting: skip task if hourly or daily limit would be exceeded
		if pt.todo.RateLimit.MaxPerHour > 0 || pt.todo.RateLimit.MaxPerDay > 0 {
			if d.isRateLimited(pt.todo, time.Now()) {
				dlog.Info("skip %s/%s — rate limit exceeded", projName, pt.todo.Name)
				d.pendingTasksMu.Lock()
				d.pendingTasks[taskKey] = "rate limit exceeded"
				d.pendingTasksMu.Unlock()
				d.inFlightMu.Lock()
				d.inFlight[taskKey]--
				if d.inFlight[taskKey] <= 0 {
					delete(d.inFlight, taskKey)
				}
				d.inFlightMu.Unlock()
				continue
			}
			// Increment counters for this execution
			d.incrementRateCounters(pt.todo.ID, time.Now())
		}

		// Local dispatch (original behavior)
		// Check global throttle rate before dispatching
		if !d.throttle.AllowDispatch(time.Now()) {
			dlog.Info("skip %s/%s — throttle rate limit reached", projName, pt.todo.Name)
			d.pendingTasksMu.Lock()
			d.pendingTasks[taskKey] = "throttle rate limit"
			d.pendingTasksMu.Unlock()
			d.inFlightMu.Lock()
			d.inFlight[taskKey]--
			if d.inFlight[taskKey] <= 0 {
				delete(d.inFlight, taskKey)
			}
			d.inFlightMu.Unlock()
			continue
		}

		// Dispatch up to instancesToDispatch instances of this task.
		// The first slot was already reserved above; reserve additional slots as we go.
		for inst := 0; inst < instancesToDispatch; inst++ {
			if inst > 0 {
				// Reserve additional in-flight slot for instances beyond the first
				d.inFlightMu.Lock()
				d.inFlight[taskKey]++
				d.inFlightMu.Unlock()
			}

			// Increment group in-flight counter if task belongs to a group
			if pt.todo.ConcurrencyGroup != "" {
				d.groupInFlightMu.Lock()
				d.groupInFlight[pt.todo.ConcurrencyGroup]++
				d.groupInFlightMu.Unlock()
			}

			// Non-blocking send; if queue is full, clear in-flight and warn
			select {
			case d.workQueue <- workItem{project: pt.proj, todo: pt.todo, sla: slaCheck}:
				// Increment rate limit counters when task is dispatched
				if pt.todo.RateLimit.MaxPerHour > 0 || pt.todo.RateLimit.MaxPerDay > 0 {
					go func() {
						counter, err := project.ReadRateLimitCounter(pt.proj.Path, pt.todo.ID)
						if err != nil {
							dlog.Warn("failed to read rate limit counter for %s/%s: %v", projName, pt.todo.Name, err)
							return
						}

						now := time.Now()
						// Reset hourly counter if needed
						if counter.ThisHourStart.Add(time.Hour).Before(now) {
							counter.ThisHourCount = 0
							counter.ThisHourStart = now.Truncate(time.Hour)
						}
						// Reset daily counter if needed
						if counter.ThisDayStart.Add(24 * time.Hour).Before(now) {
							counter.ThisDayCount = 0
							counter.ThisDayStart = now.Truncate(24 * time.Hour)
						}

						// Increment counters
						if pt.todo.RateLimit.MaxPerHour > 0 {
							counter.ThisHourCount++
						}
						if pt.todo.RateLimit.MaxPerDay > 0 {
							counter.ThisDayCount++
						}

						// Write updated counter back
						if err := project.WriteRateLimitCounter(pt.proj.Path, pt.todo.ID, counter); err != nil {
							dlog.Warn("failed to write rate limit counter for %s/%s: %v", projName, pt.todo.Name, err)
						}
					}()
				}

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
				break // Stop trying to dispatch more instances
			}
		}
	}

	dlog.TickSummary(now, len(projects), totalTodos, totalMatched, dispatched)
}

// dispatchPolledTask dispatches a task that had its polling conditions met.
func (d *Daemon) dispatchPolledTask(proj *project.Project, todo project.Todo) {
	taskKey := fmt.Sprintf("%s/%s", proj.Path, todo.Name)

	d.inFlightMu.Lock()
	d.inFlight[taskKey]++
	d.inFlightMu.Unlock()

	select {
	case d.workQueue <- workItem{project: proj, todo: todo}:
		dlog.Info("dispatched polled task %s", taskKey)
	default:
		dlog.Warn("work queue full, dropping polled task %s", taskKey)
		d.inFlightMu.Lock()
		d.inFlight[taskKey]--
		if d.inFlight[taskKey] <= 0 {
			delete(d.inFlight, taskKey)
		}
		d.inFlightMu.Unlock()
	}
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
func SendRunRequest(projectPath, taskID, taskName string, force bool) error {
	data, err := json.Marshal(RunRequest{ProjectPath: projectPath, TaskID: taskID, TaskName: taskName, Force: force})
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
func SendKillRequest(id string, checkpoint bool) error {
	data, err := json.Marshal(KillRequest{ID: id, Checkpoint: checkpoint})
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
		return fmt.Errorf("%s", strings.TrimSpace(string(body)))
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

// SendHealthRequest queries the daemon's /health endpoint.
func SendHealthRequest() (*HealthResponse, error) {
	resp, err := socketClient().Get("http://daemon/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, err
	}

	return &health, nil
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

// SendClusterStatusRequest retrieves the cluster status from the daemon.
func SendClusterStatusRequest() (map[string]any, error) {
	resp, err := socketClient().Get("http://daemon/cluster/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SendClusterLeaveRequest tells the daemon to leave the cluster.
func SendClusterLeaveRequest() (map[string]any, error) {
	resp, err := socketClient().Post("http://daemon/cluster/leave", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SendRateLimitsRequest retrieves rate limit status for all tasks
func SendRateLimitsRequest() ([]RateLimitStatus, error) {
	resp, err := socketClient().Get("http://daemon/rate-limits")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rate limits request failed: %s", string(body))
	}

	var result []RateLimitStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SendResetRateLimitsRequest resets all rate limit counters
func SendResetRateLimitsRequest() error {
	resp, err := socketClient().Post("http://daemon/reset-rate-limits", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reset rate limits failed: %s", string(body))
	}
	return nil
}

// isRateLimited checks if a task would exceed its hourly or daily rate limits
func (d *Daemon) isRateLimited(todo project.Todo, now time.Time) bool {
	if todo.RateLimit.MaxPerHour == 0 && todo.RateLimit.MaxPerDay == 0 {
		return false
	}

	d.rateLimitCountersMu.Lock()
	defer d.rateLimitCountersMu.Unlock()

	counter, exists := d.rateLimitCounters[todo.ID]
	if !exists {
		return false
	}

	// Check hourly limit
	if todo.RateLimit.MaxPerHour > 0 {
		hourKey := now.Format("2006-01-02 15")
		if counter.Hourly[hourKey] >= todo.RateLimit.MaxPerHour {
			return true
		}
	}

	// Check daily limit
	if todo.RateLimit.MaxPerDay > 0 {
		dayKey := now.Format("2006-01-02")
		if counter.Daily[dayKey] >= todo.RateLimit.MaxPerDay {
			return true
		}
	}

	return false
}

// incrementRateCounters increments the hourly and daily counters for a task
func (d *Daemon) incrementRateCounters(taskID string, now time.Time) {
	d.rateLimitCountersMu.Lock()
	defer d.rateLimitCountersMu.Unlock()

	counter, exists := d.rateLimitCounters[taskID]
	if !exists {
		counter = &RateLimitCounter{
			Hourly: make(map[string]int),
			Daily:  make(map[string]int),
		}
		d.rateLimitCounters[taskID] = counter
	}

	// Increment hourly counter
	hourKey := now.Format("2006-01-02 15")
	counter.Hourly[hourKey]++

	// Increment daily counter
	dayKey := now.Format("2006-01-02")
	counter.Daily[dayKey]++
}

// getGroupRateLimitCounter gets or creates a rate limit counter for a concurrency group
func (d *Daemon) getGroupRateLimitCounter(groupName string) (*RateLimitCounter, error) {
	d.groupRateLimitsMu.Lock()
	defer d.groupRateLimitsMu.Unlock()

	counter, exists := d.groupRateLimits[groupName]
	if !exists {
		counter = &RateLimitCounter{
			Hourly: make(map[string]int),
			Daily:  make(map[string]int),
		}
		d.groupRateLimits[groupName] = counter
	}

	return counter, nil
}
// checkForTimedOutProcesses checks all running tasks and kills any that have exceeded their timeout.
// This prevents zombie worker processes from running indefinitely.
func (d *Daemon) checkForTimedOutProcesses(now time.Time) {
	d.tasksMu.RLock()
	defer d.tasksMu.RUnlock()

	for _, task := range d.tasks {
		// Skip tasks without a valid PID or timeout
		if task.PID <= 0 || task.Timeout <= 0 {
			continue
		}

		// Calculate elapsed time
		elapsed := now.Sub(task.Started)

		// Check if task has exceeded its timeout
		if elapsed > task.Timeout {
			dlog.Warn("Task %s/%s (PID %d) exceeded timeout (%v > %v) - killing process",
				filepath.Base(task.Project), task.Name, task.PID, elapsed, task.Timeout)

			// Kill the process
			if err := d.killProcess(task.PID); err != nil {
				dlog.Warn("Failed to kill timed-out process %d for task %s/%s: %v",
					task.PID, filepath.Base(task.Project), task.Name, err)
				continue
			}

			// Log the kill with task information
			dlog.Info("Killed timed-out task %s/%s (PID %d) after %v",
				filepath.Base(task.Project), task.Name, task.PID, elapsed)

			// Mark task as failed in run record
			d.markTaskAsFailed(task, "timeout exceeded")

			// Reset issue labels if needed (remove in-progress, re-add ready)
			d.resetTaskLabels(task)
		}
	}
}

// killProcess attempts to kill a process gracefully with SIGTERM, then forcefully with SIGKILL
func (d *Daemon) killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// First try graceful termination with SIGTERM
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		if err := proc.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to kill process %d with SIGTERM and SIGKILL: %w", pid, err)
		}
		dlog.Info("Sent SIGKILL to process %d", pid)
		return nil
	}

	dlog.Info("Sent SIGTERM to process %d", pid)

	// Wait a bit for graceful shutdown
	time.Sleep(5 * time.Second)

	// Check if process is still running
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		// Process is still running, send SIGKILL
		if err := proc.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to kill process %d with SIGKILL after SIGTERM: %w", pid, err)
		}
		dlog.Info("Sent SIGKILL to process %d after SIGTERM timeout", pid)
	}

	return nil
}

// markTaskAsFailed creates a run record marking the task as failed due to timeout
func (d *Daemon) markTaskAsFailed(task *RunningTask, reason string) {
	runRecord := project.RunRecord{
		RunID:     newRunID(),
		TaskID:    task.TaskID,
		PID:       task.PID,
		Started:   task.Started,
		Finished:  time.Now(),
		Success:   false,
		Error:     reason,
		Attempt:   1,
		MaxRetries: 0,
	}

	if err := project.WriteRunRecord(task.Project, runRecord); err != nil {
		dlog.Warn("Failed to write run record for timed-out task %s/%s: %v",
			filepath.Base(task.Project), task.Name, err)
	}
}

// resetTaskLabels updates GitHub issue labels to reflect that the task has timed out
func (d *Daemon) resetTaskLabels(task *RunningTask) {
	// This would require GitHub API access and is task-specific
	// For now, we'll just log that this should happen
	dlog.Info("TODO: Reset labels for task %s/%s (remove in-progress, add ready)",
		filepath.Base(task.Project), task.Name)
}

// SendThrottlePauseRequest tells the daemon to pause (globally or by label).
func SendThrottlePauseRequest(label string) error {
	data, err := json.Marshal(ThrottlePauseRequest{Label: label})
	if err != nil {
		return err
	}
	resp, err := socketClient().Post("http://daemon/throttle/pause", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("throttle pause failed: %s", string(body))
	}
	return nil
}

// SendThrottleResumeRequest tells the daemon to resume (globally or by label).
func SendThrottleResumeRequest(label string) error {
	data, err := json.Marshal(ThrottleResumeRequest{Label: label})
	if err != nil {
		return err
	}
	resp, err := socketClient().Post("http://daemon/throttle/resume", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("throttle resume failed: %s", string(body))
	}
	return nil
}

// SendThrottleRateRequest tells the daemon to set the throttle rate.
func SendThrottleRateRequest(ratePerMin int) error {
	data, err := json.Marshal(ThrottleRateRequest{RatePerMin: ratePerMin})
	if err != nil {
		return err
	}
	resp, err := socketClient().Post("http://daemon/throttle/rate", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("throttle rate failed: %s", string(body))
	}
	return nil
}

// SendThrottleStateRequest queries the daemon's throttle state.
func SendThrottleStateRequest() (*ThrottleState, error) {
	resp, err := socketClient().Get("http://daemon/throttle/state")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("throttle state request failed: %s", resp.Status)
	}
	var state ThrottleState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}