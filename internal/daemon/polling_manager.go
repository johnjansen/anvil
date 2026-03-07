package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

// PollingManager manages polling-based trigger conditions for tasks.
// It periodically evaluates conditions and notifies the daemon when conditions are met.
type PollingManager struct {
	mu       sync.Mutex
	pollers  map[string]*poller // taskKey -> poller
	daemon   *Daemon
	stopCh   chan struct{}
	stopped  bool
}

// poller tracks a single polling task.
type poller struct {
	taskKey   string
	proj      *project.Project
	todo      project.Todo
	trigger   project.TaskTrigger
	startedAt time.Time
	cancel    context.CancelFunc
	fired     bool // true if condition was met and task was triggered
}

// NewPollingManager creates a new PollingManager.
func NewPollingManager(d *Daemon) *PollingManager {
	return &PollingManager{
		pollers: make(map[string]*poller),
		daemon:  d,
		stopCh:  make(chan struct{}),
	}
}

// Register starts polling for a task if it has polling-based trigger conditions.
// Returns true if the task was registered for polling.
func (pm *PollingManager) Register(proj *project.Project, todo project.Todo) bool {
	if todo.Trigger == nil || todo.Trigger.PollingConfig == nil || !todo.Trigger.PollingConfig.Enabled {
		return false
	}

	taskKey := fmt.Sprintf("%s/%s", proj.Path, todo.Name)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Already polling this task
	if _, exists := pm.pollers[taskKey]; exists {
		return true
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &poller{
		taskKey:   taskKey,
		proj:      proj,
		todo:      todo,
		trigger:   *todo.Trigger,
		startedAt: time.Now(),
		cancel:    cancel,
	}
	pm.pollers[taskKey] = p

	go pm.pollLoop(ctx, p)
	return true
}

// pollLoop periodically evaluates trigger conditions for a task.
func (pm *PollingManager) pollLoop(ctx context.Context, p *poller) {
	interval := p.trigger.PollingConfig.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	timeout := p.trigger.PollingConfig.Timeout
	var deadline <-chan time.Time
	if timeout > 0 {
		deadline = time.After(timeout)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-deadline:
			dlog.Info("polling timeout for %s after %v", p.taskKey, timeout)
			pm.Unregister(p.taskKey)
			return
		case <-ticker.C:
			eval, err := project.EvaluateTriggerConditions(ctx, p.trigger, p.todo)
			if err != nil {
				dlog.Warn("polling evaluation error for %s: %v", p.taskKey, err)
				continue
			}

			if eval.ConditionsMet {
				dlog.Info("polling conditions met for %s, triggering task", p.taskKey)
				pm.daemon.dispatchPolledTask(p.proj, p.todo)

				// If run_once, stop polling
				if p.trigger.PollingConfig.RunOnce {
					p.fired = true
					pm.Unregister(p.taskKey)
					return
				}
			}
		}
	}
}

// Unregister stops polling for a task.
func (pm *PollingManager) Unregister(taskKey string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if p, exists := pm.pollers[taskKey]; exists {
		p.cancel()
		delete(pm.pollers, taskKey)
	}
}

// Stop stops all polling goroutines.
func (pm *PollingManager) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.stopped {
		return
	}
	pm.stopped = true
	close(pm.stopCh)

	for key, p := range pm.pollers {
		p.cancel()
		delete(pm.pollers, key)
	}
}

// ActiveCount returns the number of active pollers.
func (pm *PollingManager) ActiveCount() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return len(pm.pollers)
}
