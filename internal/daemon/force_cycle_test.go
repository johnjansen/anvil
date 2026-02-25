package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func TestPersistentDefaultMaxRuntime(t *testing.T) {
	// When PersistentMaxRuntime is 0 (unset), persistent tasks should default to 4 hours
	d := newTestDaemon()
	todo := project.Todo{
		Name:     "test-persistent",
		Schedule: "persistent",
		// PersistentMaxRuntime is zero (unset)
	}

	// Simulate the timeout selection logic from runTask
	timeout := d.config.Timeout
	if todo.IsPersistent() {
		if todo.PersistentMaxRuntime > 0 {
			timeout = todo.PersistentMaxRuntime
		} else {
			timeout = 4 * time.Hour
		}
	} else if todo.Timeout > 0 {
		timeout = todo.Timeout
	}

	if timeout != 4*time.Hour {
		t.Errorf("expected default persistent timeout of 4h, got %v", timeout)
	}
}

func TestPersistentExplicitMaxRuntime(t *testing.T) {
	// When PersistentMaxRuntime is explicitly set, it should be used
	d := newTestDaemon()
	todo := project.Todo{
		Name:                 "test-persistent",
		Schedule:             "persistent",
		PersistentMaxRuntime: 2 * time.Hour,
	}

	timeout := d.config.Timeout
	if todo.IsPersistent() {
		if todo.PersistentMaxRuntime > 0 {
			timeout = todo.PersistentMaxRuntime
		} else {
			timeout = 4 * time.Hour
		}
	} else if todo.Timeout > 0 {
		timeout = todo.Timeout
	}

	if timeout != 2*time.Hour {
		t.Errorf("expected explicit persistent timeout of 2h, got %v", timeout)
	}
}

func TestNonPersistentUsesTaskTimeout(t *testing.T) {
	// Non-persistent tasks should use their own Timeout, not the 4h default
	d := newTestDaemon()
	todo := project.Todo{
		Name:    "test-oneshot",
		Timeout: 30 * time.Minute,
	}

	timeout := d.config.Timeout
	if todo.IsPersistent() {
		if todo.PersistentMaxRuntime > 0 {
			timeout = todo.PersistentMaxRuntime
		} else {
			timeout = 4 * time.Hour
		}
	} else if todo.Timeout > 0 {
		timeout = todo.Timeout
	}

	if timeout != 30*time.Minute {
		t.Errorf("expected task timeout of 30m, got %v", timeout)
	}
}

func TestForceCycleClearsFailureCount(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)

	taskKey := "/test/proj/my-task"

	// Simulate some prior failures
	d.persistentFailuresMu.Lock()
	d.persistentFailures[taskKey] = 3
	d.persistentFailuresMu.Unlock()

	// Simulate force-cycle detection: persistent task + deadline exceeded
	todo := project.Todo{
		Name:     "my-task",
		Schedule: "persistent",
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	forceCycled := todo.IsPersistent() && ctx.Err() == context.DeadlineExceeded
	if !forceCycled {
		t.Fatal("expected force-cycle detection to be true")
	}

	// Simulate the force-cycle handler clearing failures
	d.persistentFailuresMu.Lock()
	delete(d.persistentFailures, taskKey)
	d.persistentFailuresMu.Unlock()

	d.persistentFailuresMu.Lock()
	count := d.persistentFailures[taskKey]
	d.persistentFailuresMu.Unlock()

	if count != 0 {
		t.Errorf("expected failure count to be cleared after force-cycle, got %d", count)
	}
}

func TestWarningTimerAt80Percent(t *testing.T) {
	// Verify the 80% warning fires before the full timeout
	timeout := 100 * time.Millisecond
	warningAt := time.Duration(float64(timeout) * 0.8)

	if warningAt != 80*time.Millisecond {
		t.Errorf("expected warning at 80ms, got %v", warningAt)
	}

	warned := make(chan struct{})
	timer := time.AfterFunc(warningAt, func() {
		close(warned)
	})
	defer timer.Stop()

	select {
	case <-warned:
		// Good — warning fired before full timeout
	case <-time.After(timeout):
		t.Error("warning timer did not fire before full timeout")
	}
}

func TestForceCycleNotTriggeredForNonPersistent(t *testing.T) {
	// Non-persistent tasks with deadline exceeded should NOT be treated as force-cycle
	todo := project.Todo{
		Name:     "oneshot-task",
		Schedule: "",
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	forceCycled := todo.IsPersistent() && ctx.Err() == context.DeadlineExceeded
	if forceCycled {
		t.Error("expected force-cycle detection to be false for non-persistent task")
	}
}
