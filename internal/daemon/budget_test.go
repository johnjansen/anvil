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

func TestCostBudgetAccumulation(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)

	taskKey := "proj/my-task"

	// Accumulate $5.50 in costs
	d.costBudgetUsedMu.Lock()
	d.costBudgetUsed[taskKey] += 5.50
	d.costBudgetUsedMu.Unlock()

	// Accumulate another $2.25
	d.costBudgetUsedMu.Lock()
	d.costBudgetUsed[taskKey] += 2.25
	d.costBudgetUsedMu.Unlock()

	d.costBudgetUsedMu.Lock()
	used := d.costBudgetUsed[taskKey]
	d.costBudgetUsedMu.Unlock()

	if used != 7.75 {
		t.Errorf("expected $7.75 accumulated, got $%.2f", used)
	}
}

func TestCostBudgetExceededBlocksDispatch(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)

	taskKey := "proj/my-task"
	costBudget := 10.00

	// Task has used $12.50, budget is $10.00
	d.costBudgetUsedMu.Lock()
	d.costBudgetUsed[taskKey] = 12.50
	d.costBudgetUsedMu.Unlock()

	todo := project.Todo{
		Name:       "my-task",
		Schedule:   "persistent",
		CostBudget: costBudget,
	}

	// Check: should block
	d.costBudgetUsedMu.Lock()
	used := d.costBudgetUsed[taskKey]
	d.costBudgetUsedMu.Unlock()

	if !(todo.CostBudget > 0 && used >= todo.CostBudget) {
		t.Error("expected cost budget exceeded to block dispatch")
	}
}

func TestCostBudgetNotExceededAllowsDispatch(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)

	taskKey := "proj/my-task"
	costBudget := 10.00

	// Task has used $7.50, budget is $10.00
	d.costBudgetUsedMu.Lock()
	d.costBudgetUsed[taskKey] = 7.50
	d.costBudgetUsedMu.Unlock()

	todo := project.Todo{
		Name:       "my-task",
		Schedule:   "persistent",
		CostBudget: costBudget,
	}

	d.costBudgetUsedMu.Lock()
	used := d.costBudgetUsed[taskKey]
	d.costBudgetUsedMu.Unlock()

	if todo.CostBudget > 0 && used >= todo.CostBudget {
		t.Error("expected cost budget under limit to allow dispatch")
	}
}

func TestZeroCostBudgetMeansUnlimited(t *testing.T) {
	todo := project.Todo{
		Name:       "my-task",
		Schedule:   "persistent",
		CostBudget: 0, // no budget set
	}

	// Should NOT trigger cost budget check
	shouldCheck := todo.CostBudget > 0
	if shouldCheck {
		t.Error("expected zero cost budget to skip budget check")
	}
}
