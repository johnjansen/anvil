package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

type Daemon struct {
	config     *config.Config
	runner     *runner.Runner
	semaphores map[string]chan struct{} // keyed by taskID, controls per-task concurrency
	semMu      sync.Mutex
	stop       chan struct{}
	done       chan struct{}
	lastTick   time.Time // last minute we processed (truncated to minute)
	socketPath string
	tasks      map[string]*RunningTask
	tasksMu    sync.RWMutex
	mu         sync.Mutex      // guards busy
	busy       map[string]bool // tracks which projects have active dispatches
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
	return &Daemon{
		config:     cfg,
		runner:     runner.New(cfg.Runners, cfg.Timeout),
		semaphores: make(map[string]chan struct{}),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		socketPath: filepath.Join(config.Dir(), "daemon.sock"),
		tasks:      make(map[string]*RunningTask),
		busy:       make(map[string]bool),
	}
}

// getSemaphore returns (or lazily creates) the concurrency semaphore for a task.
// maxConcurrent controls the channel buffer size; once created the capacity is fixed.
func (d *Daemon) getSemaphore(taskKey string, maxConcurrent int) chan struct{} {
	d.semMu.Lock()
	defer d.semMu.Unlock()
	if _, ok := d.semaphores[taskKey]; !ok {
		d.semaphores[taskKey] = make(chan struct{}, maxConcurrent)
	}
	return d.semaphores[taskKey]
}

func (d *Daemon) Run() {
	defer close(d.done)

	ticker := time.NewTicker(d.config.TickInterval)
	defer ticker.Stop()

	log.Printf("daemon started (tick=%s, runner=%q, max_todos=%d)",
		d.config.TickInterval, d.config.Runner, d.config.MaxTodos)

	// Start socket server
	go d.startSocketServer()

	for {
		select {
		case <-d.stop:
			log.Println("daemon stopping")
			// Clean up socket file
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

func (d *Daemon) startSocketServer() {
	// Remove any existing socket file
	os.Remove(d.socketPath)

	// Create unix socket
	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		log.Printf("failed to start socket server: %v", err)
		return
	}
	defer listener.Close()

	// Set socket permissions for read/write by user
	os.Chmod(d.socketPath, 0600)

	mux := http.NewServeMux()
	mux.HandleFunc("/ps", d.handlePs)
	mux.HandleFunc("/kill", d.handleKill)

	server := &http.Server{
		Handler: mux,
	}

	log.Printf("socket server listening on %s", d.socketPath)
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Printf("socket server error: %v", err)
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
		if task.Name == req.ID || strings.Contains(task.Name, req.ID) || task.Project == req.ID {
			found = task
			break
		}
		// Check if the task name contains the ID as a UUID suffix
		if strings.HasSuffix(task.Name, ".md") {
			baseName := task.Name[:len(task.Name)-3]
			if strings.Contains(baseName, req.ID) {
				found = task
				break
			}
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
			log.Printf("tick %s — no watched projects", now.Format("15:04:05"))
		}
		return
	}

	// Load all projects
	var projects []*project.Project
	for _, p := range paths {
		proj, err := project.Load(p)
		if err != nil {
			log.Printf("skip %s: %v", p, err)
			continue
		}
		projects = append(projects, proj)
	}

	// Sort projects alphabetically by path (no project priority)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})

	// On non-cron ticks, log heartbeat and any busy projects
	if !cronTick {
		d.tasksMu.RLock()
		var busyNames []string
		for _, task := range d.tasks {
			elapsed := time.Since(task.Started).Round(time.Second)
			busyNames = append(busyNames, fmt.Sprintf("%s (%s)", taskLogLink(task), elapsed))
		}
		d.tasksMu.RUnlock()
		if len(busyNames) > 0 {
			log.Printf("tick %s — running: %s", now.Format("15:04:05"), strings.Join(busyNames, ", "))
		} else {
			log.Printf("tick %s — idle", now.Format("15:04:05"))
		}
		return
	}

	totalTodos := 0
	totalMatched := 0
	dispatched := 0

	for _, proj := range projects {
		projName := filepath.Base(proj.Path)

		// Skip if busy from a previous tick
		d.mu.Lock()
		busy := d.busy[proj.Path]
		d.mu.Unlock()
		if busy {
			log.Printf("  %s: busy (previous dispatch still running)", projName)
			continue
		}

		// Load all todos
		allTodos, err := proj.LoadTodos()
		if err != nil {
			log.Printf("  %s: error loading todos: %v", projName, err)
			continue
		}
		totalTodos += len(allTodos)

		// Select scheduled todos that match this minute + any one-shot (no schedule) todos
		var todos []project.Todo
		for _, t := range allTodos {
			if t.Schedule == "" || cron.Matches(t.Schedule, thisMinute) {
				todos = append(todos, t)
			}
		}
		totalMatched += len(todos)

		if len(todos) == 0 {
			continue
		}

		for _, t := range todos {
			log.Printf("  dispatch %s/%s (p%d, schedule=%q)", projName, t.Name, t.Priority, t.Schedule)
		}
		dispatched += len(todos)

		d.mu.Lock()
		d.busy[proj.Path] = true
		d.mu.Unlock()

		go d.processProject(proj, todos)
	}

	log.Printf("tick %s — %d projects, %d todos, %d matched, %d dispatched",
		now.Format("15:04:05"), len(projects), totalTodos, totalMatched, dispatched)
}

func (d *Daemon) processProject(proj *project.Project, todos []project.Todo) {
	defer func() {
		d.mu.Lock()
		d.busy[proj.Path] = false
		d.mu.Unlock()
		log.Printf("finished %s", proj.Path)
	}()

	batchSize := d.config.MaxTodos
	if batchSize < 1 {
		batchSize = 1
	}

	for i := 0; i < len(todos); i += batchSize {
		end := i + batchSize
		if end > len(todos) {
			end = len(todos)
		}
		batch := todos[i:end]

		var wg sync.WaitGroup
		for _, todo := range batch {
			wg.Add(1)
			go func(t project.Todo) {
				defer wg.Done()

				log.Printf("run %s: %s (p%d)", proj.Path, t.Name, t.Priority)

				ctx, cancel := context.WithTimeout(context.Background(), d.config.Timeout)
				defer cancel()

				// Track the running task
				taskKey := fmt.Sprintf("%s/%s", proj.Path, t.Name)
				d.tasksMu.Lock()
				d.tasks[taskKey] = &RunningTask{
					Project: proj.Path,
					Name:    t.Name,
					TaskID:  t.ID,
					PID:     os.Getpid(),
					Started: time.Now(),
					Cancel:  cancel,
				}
				d.tasksMu.Unlock()

				// Clean up task tracking when done
				defer func() {
					d.tasksMu.Lock()
					delete(d.tasks, taskKey)
					d.tasksMu.Unlock()
				}()

				// Determine resume behavior:
			// - Explicit frontmatter resume: true/false takes priority
			// - Default: recurring tasks resume, one-shots don't
			var resume bool
			if t.Resume != nil {
				resume = *t.Resume && sessionExists(proj.Path, t.ID)
			} else {
				resume = t.Schedule != "" && sessionExists(proj.Path, t.ID)
			}
			_, err := d.runner.Run(ctx, proj.Path, t.ID, resume, t.Content, func(pid int) {
					d.tasksMu.Lock()
					if task, ok := d.tasks[taskKey]; ok {
						task.PID = pid
					}
					d.tasksMu.Unlock()
				})
				if err != nil {
					log.Printf("fail %s: %s: %v", proj.Path, t.Name, err)
				} else {
					log.Printf("done %s: %s", proj.Path, t.Name)
					// Remove the todo file after successful execution (one-shot only)
					if t.Schedule == "" {
						if removeErr := os.Remove(t.Path); removeErr != nil {
							log.Printf("warn: could not remove %s: %v", t.Path, removeErr)
						}
					}
				}
			}(todo)
		}
		wg.Wait()
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
// sessionExists checks if a Claude session file exists for this todo
func sessionExists(projectPath string, todoID string) bool {
	_, err := os.Stat(project.SessionPath(projectPath, todoID))
	return err == nil
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
