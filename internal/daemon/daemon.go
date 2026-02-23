package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/internal/runner"

	"gopkg.in/yaml.v3"
)

// workItem is a single unit of work dispatched to the worker pool.
type workItem struct {
	project *project.Project
	todo    project.Todo
}

type Daemon struct {
	config     *config.Config
	runner     *runner.Runner
	workQueue  chan workItem
	inFlight   map[string]bool // taskKey -> true; set when queued, cleared when done
	inFlightMu sync.Mutex
	stop       chan struct{}
	done       chan struct{}
	lastTick   time.Time // last minute we processed (truncated to minute)
	socketPath string
	tasks      map[string]*RunningTask
	tasksMu    sync.RWMutex
	httpServer *http.Server
}

type RunningTask struct {
	Project string
	Name    string
	TaskID  string
	PID     int
	Started time.Time
	Cancel  context.CancelFunc
}

type KillRequest struct {
	ID string `json:"id"`
}

type TaskInfo struct {
	Project string `json:"project"`
	Name    string `json:"name"`
	PID     int    `json:"pid"`
	Started string `json:"started"`
	Elapsed string `json:"elapsed"`
}

func New(cfg *config.Config) *Daemon {
	poolSize := cfg.MaxWorkers
	if poolSize < 1 {
		poolSize = 1
	}
	return &Daemon{
		config:     cfg,
		runner:     runner.New(cfg.Runners, cfg.Timeout),
		workQueue:  make(chan workItem, poolSize*4),
		inFlight:   make(map[string]bool),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		socketPath: filepath.Join(config.Dir(), "daemon.sock"),
		tasks:      make(map[string]*RunningTask),
	}
}

func (d *Daemon) Run() {
	defer close(d.done)

	poolSize := d.config.MaxWorkers
	if poolSize < 1 {
		poolSize = 1
	}

	ticker := time.NewTicker(d.config.TickInterval)
	defer ticker.Stop()

	dlog.Startup(d.config.TickInterval.String(), d.config.Runner, poolSize)

	// Start socket server
	go d.startSocketServer()

	// Start worker pool
	var workerWg sync.WaitGroup
	for i := 0; i < poolSize; i++ {
		workerWg.Add(1)
		go func(id int) {
			defer workerWg.Done()
			d.worker(id)
		}(i)
	}

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
		case now := <-ticker.C:
			d.tick(now)
		}
	}
}

func (d *Daemon) Stop() {
	close(d.stop)
	<-d.done
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

// runTask executes a single todo task and handles all bookkeeping.
func (d *Daemon) runTask(workerID int, proj *project.Project, t project.Todo) {
	taskKey := fmt.Sprintf("%s/%s", proj.Path, t.Name)

	// Clear in-flight entry when the task completes
	defer func() {
		d.inFlightMu.Lock()
		delete(d.inFlight, taskKey)
		d.inFlightMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), d.config.Timeout)
	defer cancel()

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

	// For one-shot tasks, write a lock file before execution so that if the
	// daemon crashes mid-run the task is not silently re-dispatched on restart.
	// The lock file is removed on normal completion (success or failure).
	// A stale lock file (left by a crash) causes tick() to skip the todo and
	// log a warning; the user can unblock by removing the lock file manually.
	// One-shot tasks therefore have at-least-once delivery semantics: a clean
	// shutdown guarantees exactly-once, but a daemon crash may leave the task
	// incomplete with a stale lock that prevents automatic retry.
	if t.Schedule == "" {
		lockPath := t.Path + ".lock"
		if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0600); err != nil {
			dlog.Warn("could not write lock file %s: %v", lockPath, err)
		} else {
			defer os.Remove(lockPath)
		}
	}

	// Run the task
	projName := filepath.Base(proj.Path)
	taskLabel := projName + "/" + t.Name
	var childPID int
	logDir := filepath.Join(proj.Path, ".anvil", "logs", t.ID)
	usedSessionID, _, err := d.runner.Run(ctx, proj.Path, sessionToResume, resume, t.Content, taskLabel, logDir, func(pid int) {
		childPID = pid
		d.tasksMu.Lock()
		if task, ok := d.tasks[taskKey]; ok {
			task.PID = pid
		}
		d.tasksMu.Unlock()
	})

	// Write run record after completion
	runRecord := project.RunRecord{
		RunID:     runID,
		TaskID:    t.ID,
		SessionID: usedSessionID,
		PID:       childPID,
		Started:   startTime,
	}
	if writeErr := project.WriteRunRecord(proj.Path, runRecord); writeErr != nil {
		dlog.Warn("failed to write run record for %s: %v", t.Name, writeErr)
	}

	elapsed := time.Since(startTime)
	if err != nil {
		dlog.WorkerFail(workerID, projName, t.Name, err)
	} else {
		dlog.WorkerDone(workerID, projName, t.Name, elapsed)
		// Remove the todo file after successful execution (one-shot only)
		if t.Schedule == "" {
			if removeErr := os.Remove(t.Path); removeErr != nil {
				dlog.Warn("could not remove %s: %v", t.Path, removeErr)
			}
		}
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
		result = append(result, TaskInfo{
			Project: task.Project,
			Name:    task.Name,
			PID:     task.PID,
			Started: task.Started.Format(time.RFC3339),
			Elapsed: elapsed.Round(time.Second).String(),
		})
	}

	// Sort by started time (oldest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Started < result[j].Started
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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
		if task.TaskID == req.ID || task.Name == req.ID || strings.Contains(task.Name, req.ID) || task.Project == req.ID {
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

func (d *Daemon) tick(now time.Time) {
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
			busyNames = append(busyNames, fmt.Sprintf("%s (%s)", taskLogLink(task), elapsed))
		}
		d.tasksMu.RUnlock()
		if len(busyNames) > 0 {
			dlog.TickRunning(now, strings.Join(busyNames, ", "))
		} else {
			dlog.TickIdle(now)
		}
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

	for _, proj := range projects {
		projName := filepath.Base(proj.Path)
		allTodos, err := proj.LoadTodos()
		if err != nil {
			dlog.Warn("%s: error loading todos: %v", projName, err)
			continue
		}
		totalTodos += len(allTodos)

		for _, t := range allTodos {
			if t.Schedule == "" || cron.Matches(t.Schedule, thisMinute) {
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

	dispatched := 0
	for _, pt := range dueTodos {
		taskKey := fmt.Sprintf("%s/%s", pt.proj.Path, pt.todo.Name)
		projName := filepath.Base(pt.proj.Path)

		// Skip if already in-flight (queued or executing)
		d.inFlightMu.Lock()
		if d.inFlight[taskKey] {
			d.inFlightMu.Unlock()
			dlog.Info("skip %s/%s — already in-flight", projName, pt.todo.Name)
			continue
		}
		d.inFlight[taskKey] = true
		d.inFlightMu.Unlock()

		dlog.Dispatch(projName, pt.todo.Name, pt.todo.Priority, pt.todo.Schedule)

		// Non-blocking send; if queue is full, clear in-flight and warn
		select {
		case d.workQueue <- workItem{project: pt.proj, todo: pt.todo}:
			dispatched++
		default:
			dlog.Warn("work queue full, dropping %s/%s", projName, pt.todo.Name)
			d.inFlightMu.Lock()
			delete(d.inFlight, taskKey)
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
