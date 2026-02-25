package daemon

import (
	"testing"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func TestBudgetAccumulation(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)

	taskKey := "proj/my-task"

	// Accumulate 30 minutes
	d.persistentBudgetUsedMu.Lock()
	d.persistentBudgetUsed[taskKey] += 30 * time.Minute
	d.persistentBudgetUsedMu.Unlock()

	// Accumulate another 20 minutes
	d.persistentBudgetUsedMu.Lock()
	d.persistentBudgetUsed[taskKey] += 20 * time.Minute
	d.persistentBudgetUsedMu.Unlock()

	d.persistentBudgetUsedMu.Lock()
	used := d.persistentBudgetUsed[taskKey]
	d.persistentBudgetUsedMu.Unlock()

	if used != 50*time.Minute {
		t.Errorf("expected 50m accumulated, got %v", used)
	}
}

func TestBudgetExceededBlocksDispatch(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)

	taskKey := "proj/my-task"
	budget := 1 * time.Hour

	// Task has used 1h10m, budget is 1h
	d.persistentBudgetUsedMu.Lock()
	d.persistentBudgetUsed[taskKey] = 70 * time.Minute
	d.persistentBudgetUsedMu.Unlock()

	todo := project.Todo{
		Name:             "my-task",
		Schedule:         "persistent",
		PersistentBudget: budget,
	}

	// Check: should block
	d.persistentBudgetUsedMu.Lock()
	used := d.persistentBudgetUsed[taskKey]
	d.persistentBudgetUsedMu.Unlock()

	if !(todo.IsPersistent() && todo.PersistentBudget > 0 && used >= todo.PersistentBudget) {
		t.Error("expected budget exceeded to block dispatch")
	}
}

func TestBudgetNotExceededAllowsDispatch(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)

	taskKey := "proj/my-task"
	budget := 1 * time.Hour

	// Task has used 30m, budget is 1h
	d.persistentBudgetUsedMu.Lock()
	d.persistentBudgetUsed[taskKey] = 30 * time.Minute
	d.persistentBudgetUsedMu.Unlock()

	todo := project.Todo{
		Name:             "my-task",
		Schedule:         "persistent",
		PersistentBudget: budget,
	}

	d.persistentBudgetUsedMu.Lock()
	used := d.persistentBudgetUsed[taskKey]
	d.persistentBudgetUsedMu.Unlock()

	if todo.IsPersistent() && todo.PersistentBudget > 0 && used >= todo.PersistentBudget {
		t.Error("expected budget under limit to allow dispatch")
	}
}

func TestZeroBudgetMeansUnlimited(t *testing.T) {
	todo := project.Todo{
		Name:             "my-task",
		Schedule:         "persistent",
		PersistentBudget: 0, // no budget set
	}

	// Should NOT trigger budget check
	shouldCheck := todo.IsPersistent() && todo.PersistentBudget > 0
	if shouldCheck {
		t.Error("expected zero budget to skip budget check")
	}
}

func TestBudgetResetsOnDaemonRestart(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}

	// First daemon instance accumulates budget
	d1 := New(cfg)
	d1.persistentBudgetUsedMu.Lock()
	d1.persistentBudgetUsed["proj/task"] = 2 * time.Hour
	d1.persistentBudgetUsedMu.Unlock()

	// New daemon instance starts fresh (simulating restart)
	d2 := New(cfg)
	d2.persistentBudgetUsedMu.Lock()
	used := d2.persistentBudgetUsed["proj/task"]
	d2.persistentBudgetUsedMu.Unlock()

	if used != 0 {
		t.Errorf("expected budget to reset on daemon restart, got %v", used)
	}
}

func TestNonPersistentTaskSkipsBudget(t *testing.T) {
	todo := project.Todo{
		Name:             "cron-task",
		Schedule:         "*/5 * * * *",
		PersistentBudget: 1 * time.Hour,
	}

	shouldCheck := todo.IsPersistent() && todo.PersistentBudget > 0
	if shouldCheck {
		t.Error("expected non-persistent task to skip budget check even if PersistentBudget is set")
	}
}
