package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/johnjansen/anvil/internal/project"
)

// FSWatcher handles watching filesystem paths and triggering tasks
type FSWatcher struct {
	daemon    *Daemon
	watchers  map[string]*watcher // taskID -> watcher
	watchersMu sync.RWMutex
}

// watcher represents a single filesystem watcher for a task
type watcher struct {
	watcher *fsnotify.Watcher
	task    project.Todo
	project *project.Project
	path    string
	cancel  context.CancelFunc
	wg      sync.WaitGroup
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
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("filesystem path does not exist: %s", path)
	}

	// Create new watcher
	ctx, cancel := context.WithCancel(context.Background())
	w := &watcher{
		task:    task,
		project: proj,
		path:    path,
		cancel:  cancel,
	}

	// Create fsnotify watcher
	log.Printf("Creating filesystem watcher for path %s for task %s", path, task.Name)
	watcherObj, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create filesystem watcher for task %s: %v", task.Name, err)
		cancel()
		return fmt.Errorf("failed to create filesystem watcher: %w", err)
	}
	w.watcher = watcherObj

	// Add path to watcher
	log.Printf("Adding path %s to filesystem watcher for task %s", path, task.Name)
	if err := watcherObj.Add(path); err != nil {
		log.Printf("Failed to add path %s to filesystem watcher for task %s: %v", path, task.Name, err)
		watcherObj.Close()
		cancel()
		return fmt.Errorf("failed to add path to watcher: %w", err)
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

	// Close watcher
	if w.watcher != nil {
		w.watcher.Close()
	}

	// Wait for processing to finish
	w.wg.Wait()

	// Remove from watchers map
	delete(fw.watchers, taskID)

	return nil
}

// StopAll stops all active watchers
func (fw *FSWatcher) StopAll() {
	fw.watchersMu.Lock()
	defer fw.watchersMu.Unlock()

	for taskID, w := range fw.watchers {
		// Cancel context to stop event processing
		w.cancel()

		// Close watcher
		if w.watcher != nil {
			w.watcher.Close()
		}

		// Wait for processing to finish
		w.wg.Wait()

		// Remove from watchers map
		delete(fw.watchers, taskID)
	}
}

// processEvents processes filesystem events
func (fw *FSWatcher) processEvents(ctx context.Context, w *watcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				// Channel closed
				return
			}

			// Process the event by triggering the task
			// We trigger on write, create, and rename events
			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Rename == fsnotify.Rename {
				fw.handleEvent(w, event)
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				// Channel closed
				return
			}
			log.Printf("Filesystem watcher error for task %s: %v", w.task.Name, err)
		}
	}
}

// handleEvent handles a single filesystem event by triggering the associated task
func (fw *FSWatcher) handleEvent(w *watcher, event fsnotify.Event) {
	// Create environment variables with event information
	env := make(map[string]string)
	for k, v := range w.task.Env {
		env[k] = v
	}

	// Add event information as environment variables
	env["ANVIL_FS_EVENT"] = event.Op.String()
	env["ANVIL_FS_PATH"] = event.Name

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
		log.Printf("Enqueued task %s from filesystem subscription (event: %s, path: %s)", taskWithEnv.Name, event.Op.String(), event.Name)
	default:
		log.Printf("Work queue full, dropping task %s from filesystem subscription", taskWithEnv.Name)
	}
}