# SPEC-331.md - Task Priority Aging

## Project Overview
- **Project**: anvil
- **Feature**: Task queuing priority boost based on wait time
- **Issue**: #331
- **Goal**: Prevent low-priority tasks from being starved by automatically boosting their effective priority after they wait too long

## Problem Statement

Low-priority tasks can get starved in a busy system. High-priority tasks always run first, but a low-priority task that's been waiting a long time never gets a chance. Users need a way to ensure tasks don't wait forever.

## Proposed Solution

### 1. Global Config

Add `priority_aging` configuration to `~/.anvil/config.yaml`:

```yaml
priority_aging:
  enabled: true
  threshold: 30m    # after waiting 30 minutes
  boost_by: 2       # lower priority by 2 levels (p5 → p3)
  max_boost: 4      # never boost more than 4 levels
```

### 2. Per-Task Override

Tasks can disable priority aging:

```yaml
---
schedule: "*/30 * * *"
priority_aging: false
---
```

### 3. Effective Priority Calculation

- `base_priority`: The task's configured priority (0-9)
- `wait_time`: Time since task was queued (for scheduled tasks, time since due)
- `boost`: Min((wait_time - threshold) / threshold, max_boost) * boost_by
- `effective_priority`: max(0, base_priority + boost)

Example:
- p5 task waits 45 minutes
- threshold = 30m, boost_by = 2, max_boost = 4
- boost = (45-30)/30 = 0.5 → 1 * 2 = 2
- effective_priority = 5 - 2 = p3

### 4. Visibility

Show effective priority in `anvil task queue`:

```bash
$ anvil task queue
TASK           WAITED   PRIORITY   EFFECTIVE
low-task       45m      p5         p3 (aged)
high-task      1m       p1         p1
```

## Technical Design

### Data Model

**New fields in Config struct** (`internal/project/project.go`):
```go
type PriorityAgingConfig struct {
    Enabled   bool          `yaml:"enabled"`
    Threshold time.Duration `yaml:"threshold"`
    BoostBy   int           `yaml:"boost_by"`
    MaxBoost  int           `yaml:"max_boost"`
}
```

**New field in Config struct**:
```go
type Config struct {
    Defaults      TaskDefaults       `yaml:"defaults"`
    PriorityAging PriorityAgingConfig `yaml:"priority_aging"`
}
```

**New field in Todo struct**:
```go
type Todo struct {
    // ... existing fields ...
    PriorityAging *bool `yaml:"priority_aging,omitempty"` // nil = use global, false = disabled, true = enabled
}
```

**New method on Todo**:
```go
func (t *Todo) EffectivePriority(configPriorityAging PriorityAgingConfig, waitTime time.Duration) int {
    // Returns base priority with aging boost applied
}
```

### Priority Calculation in Daemon

The daemon already sorts tasks by priority in `daemon.go:2234`. The sorting logic needs to use `EffectivePriority()` instead of directly accessing `todo.Priority`.

Key location: `daemon.go:2234` - the priority comparison function used when building `dueTodos`.

### Wait Time Tracking

- For scheduled tasks: `wait_time = now - next_scheduled_time` when task becomes due
- For manual tasks: `wait_time = now - queued_time`
- Track `queued_at` time in the task's pending state in daemon

### Storage

- Global config in `.anvil/config.yaml`
- Per-task override in task frontmatter (already stored in todo file)

## Acceptance Criteria

- [ ] Tasks waiting longer than threshold get automatic priority boost
- [ ] `priority_aging` config in config.yaml
- [ ] `boost_by` and `max_boost` configurable
- [ ] Visible in `anvil task queue`
- [ ] Can disable per-task with `priority_aging: false`

## Files to Modify

1. `internal/project/project.go` - Add PriorityAgingConfig, update Config, add EffectivePriority method
2. `internal/config/config.go` - Load priority_aging from global config if exists
3. `internal/daemon/daemon.go` - Use EffectivePriority in sorting logic, track wait time
4. `cmd/anvil/task_queue.go` - Show effective priority column (or update existing queue command)

## Edge Cases

- **Aging disabled globally, enabled per-task**: Per-task setting takes precedence
- **Task just became due (waited 0 time)**: No boost applied
- **Very long wait (exceeds max_boost)**: Cap at max_boost
- **Priority would go below 0**: Clamp to 0 (highest priority)
- **Daemon restart**: Reset wait tracking on restart (expected behavior)
