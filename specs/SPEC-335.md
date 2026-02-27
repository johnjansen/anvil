# SPEC-335.md - Task Health Based on Run History

## Project Overview
- **Project**: anvil
- **Feature**: Task health status based on consecutive run failures/successes
- **Issue**: #335
- **Goal**: Track task run history and display health status based on consecutive failures/successes

## Problem Statement

Currently, there's no way to define health checks for tasks. Users can't easily determine if a task is "healthy" or "unhealthy" based on its recent behavior. They need to manually check logs to see if a task has been failing repeatedly.

## Proposed Solution

### 1. Health Check Configuration (Task Frontmatter)

Add a `health_check` field to task frontmatter:

```yaml
---
schedule: "0 9 * * *"
health_check:
  failure_threshold: 3    # consider unhealthy after 3 consecutive failures
  success_threshold: 2    # consider healthy after 2 consecutive successes
  timeout: 5m            # consider unhealthy if task runs longer than this
---
```

- `failure_threshold`: Number of consecutive failures before marking unhealthy (default: 3)
- `success_threshold`: Number of consecutive successes to recover to healthy (default: 2)
- `timeout`: Maximum execution time - mark unhealthy if task exceeds this (default: no timeout)

### 2. Health Status Storage

Store health status per task in the daemon:

```go
type TaskHealthStatus struct {
    TaskID           string
    TaskName         string
    Project          string
    Healthy          bool
    ConsecutiveFailures int
    ConsecutiveSuccesses int
    LastCheck        time.Time
    LastError        string
}
```

Health status persisted to disk in `.anvil/health/<task-id>.json`:

```json
{
  "task_id": "fetch-data",
  "healthy": true,
  "consecutive_failures": 0,
  "consecutive_successes": 2,
  "last_check": "2026-02-28T10:00:00Z",
  "last_error": ""
}
```

### 3. CLI Visibility

```bash
# Show health status of all tasks with health_check defined
$ anvil task health
TASK           HEALTH    CONSECUTIVE FAILURES    LAST_CHECK
fetch-data    healthy   0                       10s ago
process-data  unhealthy 5                       30s ago

# Show specific task health
$ anvil task health my-task
Task: my-task
Health: unhealthy
Consecutive Failures: 5
Consecutive Successes: 0
Last Check: 30s ago
Last Error: task failed: exit code 1

# Reset health status
$ anvil task health my-task --reset
Health status reset
```

### 4. Health Status Logic

- **Initial state**: Unknown (no runs yet)
- **After first failure**: Unhealthy (failure_threshold met immediately)
- **After first success**: Healthy (start building consecutive successes)
- **Recovery**: Must have `success_threshold` consecutive successes to become healthy again
- **Timeout**: If task execution exceeds timeout, increment failure count

### 5. Integration with Run Records

The daemon reads run records from `.anruns/<task-id>/` to determine consecutive failures/successes:
- On each task completion, update health status
- Count only the most recent consecutive outcomes
- Reset counter when status flips

## Technical Design

### Data Model

**New fields in Todo struct** (`internal/project/project.go`):
```go
type HealthCheckConfig struct {
    FailureThreshold    int           `yaml:"failure_threshold,omitempty"`    // default 3
    SuccessThreshold    int           `yaml:"success_threshold,omitempty"`    // default 2
    Timeout             time.Duration `yaml:"timeout,omitempty"`              // default 0 (no timeout)
}

type Todo struct {
    // ... existing fields ...
    HealthCheck *HealthCheckConfig `yaml:"health_check,omitempty"`
}
```

**New struct for health status** (`internal/daemon/daemon.go`):
```go
type TaskHealthStatus struct {
    TaskID               string    `json:"task_id"`
    TaskName             string    `json:"task"`
    Project              string    `json:"project"`
    Healthy              bool      `json:"healthy"`
    ConsecutiveFailures  int       `json:"consecutive_failures"`
    ConsecutiveSuccesses int       `json:"consecutive_successes"`
    LastCheck            time.Time `json:"last_check"`
    LastError            string    `json:"error,omitempty"`
}
```

**New fields in Daemon struct**:
```go
type Daemon struct {
    // ... existing fields ...
    taskHealth     map[string]*TaskHealthStatus  // key: "project:taskName"
    taskHealthMu   sync.RWMutex
}
```

### Health Check Runner

A goroutine that runs on task completion:
1. When task completes, read run record to determine success/failure
2. Update consecutive count in health status
3. Evaluate against thresholds to set healthy/unhealthy
4. Persist health status to disk

### Socket Command for CLI

Add a new socket command handler for `task-health`:
- Returns JSON array of TaskHealthStatus
- Supports `--reset` flag to reset health status for a task
- CLI formats the response as a table

## Acceptance Criteria

- [ ] Tasks support `health_check` configuration in frontmatter
- [ ] Tasks marked unhealthy after consecutive failures >= failure_threshold
- [ ] Tasks recover to healthy after consecutive successes >= success_threshold
- [ ] `anvil task health` shows health status table
- [ ] `anvil task health <task>` shows detailed status
- [ ] Health status persisted to disk and restored on daemon restart
- [ ] Timeout field marks task unhealthy if execution exceeds threshold
- [ ] `--reset` clears health status for a task

## Files to Modify

1. `internal/project/project.go` - Add HealthCheckConfig struct and field to Todo
2. `internal/daemon/daemon.go` - Add TaskHealthStatus, health status management, run completion handler
3. `cmd/anvil/main.go` - Add `task health` subcommand with table output

## Edge Cases

- **First run**: Task starts as unknown, becomes unhealthy after first failure
- **No health_check config**: Task not included in health output
- **All tasks succeed**: All show healthy with consecutive_successes
- **Daemon restart**: Load persisted health status from disk
- **Task deleted**: Remove health status from memory and disk
- **Timeout**: Check task duration against timeout threshold
