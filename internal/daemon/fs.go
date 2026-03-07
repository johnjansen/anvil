package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/johnjansen/anvil/internal/project"
)

// FileEvent represents a single file change event collected during a debounce window.
type FileEvent struct {
	Path  string `json:"path"`
	Event string `json:"event"`
}

// debouncer accumulates file events and flushes them after a configurable quiet period.
type debouncer struct {
	mu       sync.Mutex
	timer    *time.Timer
	duration time.Duration
	events   []FileEvent
	onFlush  func([]FileEvent)
}

// newDebouncer creates a debouncer with the given duration and flush callback.
func newDebouncer(duration time.Duration, onFlush func([]FileEvent)) *debouncer {
	return &debouncer{
		duration: duration,
		onFlush:  onFlush,
	}
}

// add adds an event to the debouncer and resets the timer.
func (d *debouncer) add(event FileEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.events = append(d.events, event)

	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.duration, d.flush)
}

// flush fires the callback with accumulated events and resets the batch.
func (d *debouncer) flush() {
	d.mu.Lock()
	events := d.events
	d.events = nil
	d.mu.Unlock()

	if len(events) > 0 && d.onFlush != nil {
		d.onFlush(events)
	}
}

// stop cancels any pending timer.
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.events = nil
}

// eventNameToOp maps user-facing event names to fsnotify operations.
func eventNameToOp(name string) (fsnotify.Op, bool) {
	switch name {
	case "create":
		return fsnotify.Create, true
	case "modify":
		return fsnotify.Write, true
	case "delete":
		return fsnotify.Remove, true
	case "rename":
		return fsnotify.Rename, true
	default:
		return 0, false
	}
}

// opToEventName maps fsnotify operations to user-facing event names.
func opToEventName(op fsnotify.Op) string {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return "create"
	case op&fsnotify.Write == fsnotify.Write:
		return "modify"
	case op&fsnotify.Remove == fsnotify.Remove:
		return "delete"
	case op&fsnotify.Rename == fsnotify.Rename:
		return "rename"
	default:
		return op.String()
	}
}

// matchGlob checks if a filename matches a glob pattern.
func matchGlob(pattern, name string) bool {
	matched, err := filepath.Match(pattern, filepath.Base(name))
	if err != nil {
		return false
	}
	return matched
}

// validFsEvents is the set of valid event type names.
var validFsEvents = map[string]bool{
	"create": true,
	"modify": true,
	"delete": true,
	"rename": true,
}

// FSWatcher handles watching filesystem paths and triggering tasks
type FSWatcher struct {
	daemon     *Daemon
	watchers   map[string]*watcher // taskID -> watcher
	watchersMu sync.RWMutex
}

// watcher represents a single filesystem watcher for a task
type watcher struct {
	watcher   *fsnotify.Watcher
	task      project.Todo
	project   *project.Project
	path      string
	glob      string        // glob pattern for filtering
	events    map[fsnotify.Op]bool // allowed event ops
	debouncer *debouncer
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewFSWatcher creates a new filesystem watcher manager
func NewFSWatcher(daemon *Daemon) *FSWatcher {
	return &FSWatcher{
		daemon:   daemon,
		watchers: make(map[string]*watcher),
	}
}

// StartSubscription starts watching a filesystem path for a task with fs subscription
func (fw *FSWatcher) StartSubscription(proj *project.Project, task project.Todo) error {
	if task.Subscription == nil || task.Subscription.Type != "fs" {
		return nil // Not a filesystem subscription
	}

	// Validate required fields
	if task.Subscription.FsPath == "" {
		return fmt.Errorf("filesystem subscription requires fs_path")
	}

	// Validate fs_events values
	for _, ev := range task.Subscription.FsEvents {
		if !validFsEvents[ev] {
			return fmt.Errorf("invalid fs_events value %q: must be create, modify, delete, or rename", ev)
		}
	}

	// Parse debounce duration (default 1s)
	debounceDur := time.Second
	if task.Subscription.FsDebounce != "" {
		d, err := time.ParseDuration(task.Subscription.FsDebounce)
		if err != nil {
			return fmt.Errorf("invalid fs_debounce %q: %w", task.Subscription.FsDebounce, err)
		}
		if d < 100*time.Millisecond {
			return fmt.Errorf("fs_debounce must be at least 100ms, got %s", d)
		}
		debounceDur = d
	}

	// Validate glob pattern
	glob := task.Subscription.FsGlob
	if glob == "" {
		glob = "*"
	}
	if _, err := filepath.Match(glob, "test"); err != nil {
		return fmt.Errorf("invalid fs_glob pattern %q: %w", glob, err)
	}

	fw.watchersMu.Lock()
	defer fw.watchersMu.Unlock()

	// Check if already watching
	if _, exists := fw.watchers[task.ID]; exists {
		return nil // Already watching
	}

	// Resolve the path to an absolute path
	path, err := filepath.Abs(task.Subscription.FsPath)
	if err != nil {
		return fmt.Errorf("failed to resolve fs_path to absolute path: %w", err)
	}

	// Check if path exists
	info, statErr := os.Stat(path)
	if statErr != nil && os.IsNotExist(statErr) {
		log.Printf("Warning: filesystem path %s does not exist for task %s, will poll for creation", path, task.Name)
		ctx, cancel := context.WithCancel(context.Background())
		w := &watcher{
			task:    task,
			project: proj,
			path:    path,
			glob:    glob,
			cancel:  cancel,
		}
		fw.watchers[task.ID] = w

		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			fw.pollForDirectory(ctx, proj, task, path)
		}()
		return nil
	}

	// Build allowed events map
	allowedEvents := buildAllowedEvents(task.Subscription.FsEvents)

	// Create new watcher
	ctx, cancel := context.WithCancel(context.Background())
	w := &watcher{
		task:    task,
		project: proj,
		path:    path,
		glob:    glob,
		events:  allowedEvents,
		cancel:  cancel,
	}

	// Create debouncer
	w.debouncer = newDebouncer(debounceDur, func(events []FileEvent) {
		fw.handleBatchEvent(w, events)
	})

	// Create fsnotify watcher
	log.Printf("Creating filesystem watcher for path %s for task %s (glob=%s, debounce=%s)", path, task.Name, glob, debounceDur)
	watcherObj, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create filesystem watcher for task %s: %v", task.Name, err)
		cancel()
		return fmt.Errorf("failed to create filesystem watcher: %w", err)
	}
	w.watcher = watcherObj

	// Add path to watcher
	if err := watcherObj.Add(path); err != nil {
		log.Printf("Failed to add path %s to filesystem watcher for task %s: %v", path, task.Name, err)
		watcherObj.Close()
		cancel()
		return fmt.Errorf("failed to add path to watcher: %w", err)
	}

	// If recursive, walk subdirectories and add them too
	if task.Subscription.FsRecursive && info != nil && info.IsDir() {
		filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && p != path {
				if addErr := watcherObj.Add(p); addErr != nil {
					log.Printf("Warning: failed to add subdirectory %s to watcher for task %s: %v", p, task.Name, addErr)
				}
			}
			return nil
		})
	}

	// Store watcher
	fw.watchers[task.ID] = w

	// Start watching events
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		fw.processEvents(ctx, w)
	}()

	log.Printf("Started filesystem watcher for task %s on path %s", task.Name, path)
	return nil
}

// buildAllowedEvents builds a map of allowed fsnotify operations from event name list.
// If the list is empty, all events are allowed.
func buildAllowedEvents(eventNames []string) map[fsnotify.Op]bool {
	if len(eventNames) == 0 {
		return map[fsnotify.Op]bool{
			fsnotify.Create: true,
			fsnotify.Write:  true,
			fsnotify.Remove: true,
			fsnotify.Rename: true,
		}
	}
	allowed := make(map[fsnotify.Op]bool)
	for _, name := range eventNames {
		if op, ok := eventNameToOp(name); ok {
			allowed[op] = true
		}
	}
	return allowed
}

// pollForDirectory polls for a non-existent directory to appear, then starts watching.
func (fw *FSWatcher) pollForDirectory(ctx context.Context, proj *project.Project, task project.Todo, path string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				log.Printf("Directory %s now exists for task %s, starting watcher", path, task.Name)
				// Remove the placeholder watcher entry
				fw.watchersMu.Lock()
				delete(fw.watchers, task.ID)
				fw.watchersMu.Unlock()
				// Start real subscription
				if err := fw.StartSubscription(proj, task); err != nil {
					log.Printf("Failed to start watcher after directory creation for task %s: %v", task.Name, err)
				}
				return
			}
		}
	}
}

// StopSubscription stops watching a filesystem path for a task
func (fw *FSWatcher) StopSubscription(taskID string) error {
	fw.watchersMu.Lock()
	defer fw.watchersMu.Unlock()

	w, exists := fw.watchers[taskID]
	if !exists {
		return nil // Not watching
	}

	// Cancel context to stop event processing
	w.cancel()

	// Stop debouncer
	if w.debouncer != nil {
		w.debouncer.stop()
	}

	// Close watcher
	if w.watcher != nil {
		w.watcher.Close()
	}

	// Wait for processing to finish
	w.wg.Wait()

	// Remove from watchers map
	delete(fw.watchers, taskID)

	log.Printf("Stopped filesystem watcher for task %s", w.task.Name)
	return nil
}

// StopAll stops all active watchers
func (fw *FSWatcher) StopAll() {
	fw.watchersMu.Lock()
	defer fw.watchersMu.Unlock()

	for taskID, w := range fw.watchers {
		// Cancel context to stop event processing
		w.cancel()

		// Stop debouncer
		if w.debouncer != nil {
			w.debouncer.stop()
		}

		// Close watcher
		if w.watcher != nil {
			w.watcher.Close()
		}

		// Wait for processing to finish
		w.wg.Wait()

		// Remove from watchers map
		delete(fw.watchers, taskID)
	}
	log.Printf("Stopped all filesystem watchers")
}

// processEvents processes filesystem events with filtering and debouncing
func (fw *FSWatcher) processEvents(ctx context.Context, w *watcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// If recursive watching, auto-add new subdirectories
			if w.task.Subscription != nil && w.task.Subscription.FsRecursive &&
				event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := w.watcher.Add(event.Name); err != nil {
						log.Printf("Warning: failed to add new subdirectory %s to watcher for task %s: %v", event.Name, w.task.Name, err)
					} else {
						log.Printf("Added new subdirectory %s to watcher for task %s", event.Name, w.task.Name)
					}
				}
			}

			// Check if event type is allowed
			if !fw.matchesEventFilter(w, event.Op) {
				continue
			}

			// Check if file matches glob pattern
			if !matchGlob(w.glob, event.Name) {
				continue
			}

			// Route through debouncer
			fe := FileEvent{
				Path:  event.Name,
				Event: opToEventName(event.Op),
			}
			log.Printf("File event detected for task %s: %s %s", w.task.Name, fe.Event, fe.Path)
			w.debouncer.add(fe)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Filesystem watcher error for task %s: %v", w.task.Name, err)
		}
	}
}

// matchesEventFilter checks if an fsnotify operation is in the watcher's allowed set.
func (fw *FSWatcher) matchesEventFilter(w *watcher, op fsnotify.Op) bool {
	// Check each possible op flag against the allowed set
	for allowedOp := range w.events {
		if op&allowedOp == allowedOp {
			return true
		}
	}
	return false
}

// handleBatchEvent handles a batch of debounced file events by triggering the associated task.
func (fw *FSWatcher) handleBatchEvent(w *watcher, events []FileEvent) {
	if len(events) == 0 {
		return
	}

	// Create environment variables with event information
	env := make(map[string]string)
	for k, v := range w.task.Env {
		env[k] = v
	}

	// Last event for backward compatibility
	last := events[len(events)-1]
	env["ANVIL_FS_EVENT"] = last.Event
	env["ANVIL_FS_PATH"] = last.Path

	// Batch information
	env["ANVIL_FS_EVENT_COUNT"] = strconv.Itoa(len(events))

	// JSON array of all events
	eventsJSON, err := json.Marshal(events)
	if err == nil {
		env["ANVIL_FS_EVENTS"] = string(eventsJSON)
	}

	// Create a copy of the task with updated environment
	taskWithEnv := w.task
	taskWithEnv.Env = env

	// Check if daemon is shutting down
	select {
	case <-fw.daemon.stop:
		return
	default:
	}

	// Add to work queue
	select {
	case fw.daemon.workQueue <- workItem{project: w.project, todo: taskWithEnv}:
		log.Printf("Enqueued task %s from filesystem subscription (%d events, last: %s %s)", taskWithEnv.Name, len(events), last.Event, last.Path)
	default:
		log.Printf("Work queue full, dropping task %s from filesystem subscription", taskWithEnv.Name)
	}
}
