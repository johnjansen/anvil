# Implementation Plan: Task Health Based on Run History (#335)

## Overview

Add task health checks based on consecutive run failures/successes. The daemon tracks run history and displays health status via CLI (`anvil task health`). Health is determined by:
- Number of consecutive failures (unhealthy after threshold)
- Number of consecutive successes (recover to healthy after threshold)
- Execution timeout (unhealthy if task runs too long)

## Implementation Steps

### Phase 1: Data Model (no dependencies)

**1.1 Add health check config to Todo struct**
- File: `internal/project/project.go`
- Add `HealthCheck *HealthCheckConfig` to Todo struct
- Define `HealthCheckConfig` struct with FailureThreshold, SuccessThreshold, Timeout
- Parse from frontmatter via yaml tags

**1.2 Add health status persistence directory**
- Create `.anvil/health/` directory for storing health status JSON files

### Phase 2: Health Status Management (no dependencies)

**2.1 Add TaskHealthStatus type and tracking map**
- File: `internal/daemon/daemon.go`
- Define `TaskHealthStatus` struct (TaskID, TaskName, Project, Healthy, ConsecutiveFailures, ConsecutiveSuccesses, LastCheck, LastError)
- Add `taskHealth map[string]*TaskHealthStatus` and `taskHealthMu sync.RWMutex` to Daemon struct
- Initialize map in `New()`

**2.2 Add health status persistence methods**
- File: `internal/daemon/daemon.go`
- `loadHealthStatus(taskID string)` - load from `.anvil/health/<taskID>.json`
- `saveHealthStatus(taskID string, status *TaskHealthStatus)` - save to disk
- `loadAllHealthStatus()` - load all on daemon startup
- `deleteHealthStatus(taskID string)` - remove when task deleted

### Phase 3: Run Completion Handler (depends on Phase 1 & 2)

**3.1 Hook into task completion**
- File: `internal/daemon/daemon.go`
- In `runTask` or `taskCompleted`, after run finishes:
  - Determine success/failure from run record
  - Update consecutive counts
  - Evaluate against thresholds
  - Save health status to disk

**3.2 Health status update logic**
- On failure: increment ConsecutiveFailures, reset ConsecutiveSuccesses
- On success: increment ConsecutiveSuccesses, reset ConsecutiveFailures
- Healthy = ConsecutiveSuccesses >= SuccessThreshold
- Unhealthy = ConsecutiveFailures >= FailureThreshold
- Handle timeout: if duration > Timeout, treat as failure

**3.3 Wire into daemon lifecycle**
- Load all health statuses on daemon startup
- Delete health status file when task removed

### Phase 4: CLI Command (depends on Phase 2)

**4.1 Add socket command handler for task-health**
- File: `internal/daemon/daemon.go`
- Register `/task-health` handler on socket mux
- Returns JSON array of TaskHealthStatus
- Supports query params: `task` (filter by name)

**4.2 Add `task health` subcommand**
- File: `cmd/anvil/main.go`
- New subcommand under `task` that calls daemon socket `/task-health`
- Formats response as table: TASK, HEALTH, CONSECUTIVE FAILURES, LAST_CHECK
- `--verbose` flag shows last error

**4.3 Add `task health <task>` detail view**
- File: `cmd/anvil/main.go`
- Show detailed health status for a specific task

**4.4 Add `--reset` flag**
- File: `cmd/anvil/main.go` and daemon handler
- Reset health status (clear consecutive counts, set healthy)

## Dependencies

- Phase 2 depends on Phase 1 (needs Todo.HealthCheck field)
- Phase 3 depends on Phase 1 & 2 (needs health status struct and storage)
- Phase 4 depends on Phase 2 (needs socket handler and taskHealth data)

## Testing Strategy

- Unit tests for consecutive count logic (failure -> increment failures, reset successes)
- Unit tests for threshold evaluation
- Manual testing:
  - Create task with `health_check.failure_threshold: 3`
  - Run task 3 times, verify it becomes unhealthy
  - Run task 2 more times successfully, verify it recovers to healthy
  - Verify `anvil task health` shows correct status
  - Verify `--reset` clears status

## Files to Modify

1. `internal/project/project.go` - HealthCheckConfig struct and Todo field
2. `internal/daemon/daemon.go` - TaskHealthStatus, health map, persistence methods, run completion handler, socket handler
3. `cmd/anvil/main.go` - `task health` CLI subcommand
