# Implementation Plan: Task Health Check Endpoint (#338)

## Overview

Add per-task health checks to anvil. Tasks define a `health_check` shell command in frontmatter. The daemon runs these periodically, tracks results in memory, and exposes them via CLI (`anvil task health`) and the existing `/health` endpoint.

## Implementation Steps

### Phase 1: Data Model (no dependencies)

**1.1 Add health check fields to Todo struct**
- File: `internal/project/project.go`
- Add `HealthCheck string` and `HealthCheckInterval time.Duration` to Todo struct
- Parse from frontmatter via yaml tags

**1.2 Add HealthConfig to Config struct**
- File: `internal/config/config.go`
- Add `HealthConfig` struct with `IncludeTasks bool` and `UnhealthyThreshold int`
- Add `Health HealthConfig` field to Config struct
- Load from `.anvil/config.yaml` under `health:` key

### Phase 2: Daemon Health Check Runner (depends on Phase 1)

**2.1 Add TaskHealthStatus type and tracking map**
- File: `internal/daemon/daemon.go`
- Define `TaskHealthStatus` struct (TaskID, TaskName, Project, Healthy *bool, LastCheck, LastError)
- Add `taskHealth map[string]*TaskHealthStatus` and `taskHealthMu sync.RWMutex` to Daemon struct
- Initialize map in `New()`

**2.2 Implement health check runner goroutine**
- File: `internal/daemon/daemon.go`
- New method `runHealthChecks()` launched as goroutine from `Run()`
- Ticks every 5 seconds
- For each watched project, iterates todos with `HealthCheck` set
- Skips disabled tasks and tasks whose interval hasn't elapsed
- Executes health check command with 30s timeout using `exec.CommandContext`
- Updates `taskHealth` map with result

**2.3 Wire into daemon lifecycle**
- Launch `go d.runHealthChecks()` in `Run()` after socket server starts
- Goroutine exits on `d.stop` channel

### Phase 3: /health Endpoint Integration (depends on Phase 2)

**3.1 Extend HealthResponse struct**
- File: `internal/daemon/daemon.go`
- Add `TaskHealth []TaskHealthStatus` field to HealthResponse (json: "task_health,omitempty")

**3.2 Update handleHealth**
- File: `internal/daemon/daemon.go`
- When `detailed=true` and `config.Health.IncludeTasks` is true, populate `TaskHealth` from `taskHealth` map
- When `config.Health.UnhealthyThreshold > 0`, count unhealthy tasks and set `healthy = false` if threshold exceeded

### Phase 4: CLI Command (depends on Phase 2)

**4.1 Add socket command handler for task-health**
- File: `internal/daemon/daemon.go`
- Register `/task-health` handler on socket mux
- Returns JSON array of TaskHealthStatus
- Supports query params: `task` (filter by name), `check` (trigger immediate check)

**4.2 Add `task health` subcommand**
- File: `cmd/anvil/main.go`
- New subcommand under `task` that calls daemon socket `/task-health`
- Formats response as table: TASK, PROJECT, HEALTH, LAST_CHECK
- `--check` flag triggers immediate check for named task
- `--verbose` flag shows last error for unhealthy tasks

## Dependencies

- Phase 2 depends on Phase 1 (needs Todo.HealthCheck field and Config.Health)
- Phase 3 depends on Phase 2 (needs taskHealth map populated)
- Phase 4 depends on Phase 2 (needs socket handler and taskHealth data)
- Phase 3 and Phase 4 are independent of each other

## Testing Strategy

- Unit tests for health check interval logic (should check vs should skip)
- Unit tests for unhealthy threshold aggregation
- Manual testing:
  - Create task with `health_check: "true"` (always healthy)
  - Create task with `health_check: "false"` (always unhealthy)
  - Verify `anvil task health` shows correct status
  - Verify `/health?detailed=true` includes task health
  - Test `--check` flag triggers immediate check

## Files to Modify

1. `internal/project/project.go` - Todo struct fields
2. `internal/config/config.go` - HealthConfig struct and Config field
3. `internal/daemon/daemon.go` - TaskHealthStatus, runner goroutine, /health changes, socket handler
4. `cmd/anvil/main.go` - `task health` CLI subcommand
