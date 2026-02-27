# Tasks: Task Health Check Endpoint (#338)

## Task Breakdown

### Phase 1: Data Model

- [ ] **1.1** Add `HealthCheck string` field to Todo struct in `internal/project/project.go`
  - yaml tag: `yaml:"health_check,omitempty"`
  - Shell command whose exit code determines health (0=healthy)
- [ ] **1.2** Add `HealthCheckInterval time.Duration` field to Todo struct
  - yaml tag: `yaml:"health_check_interval,omitempty"`
  - Default: 60s when health_check is set
- [ ] **1.3** Add `HealthConfig` struct to `internal/config/config.go`
  - Fields: `IncludeTasks bool` (yaml: "include_tasks"), `UnhealthyThreshold int` (yaml: "unhealthy_threshold")
- [ ] **1.4** Add `Health HealthConfig` field to Config struct in `internal/config/config.go`
  - yaml tag: `yaml:"health"`

### Phase 2: Daemon Health Check Runner

- [ ] **2.1** Define `TaskHealthStatus` struct in `internal/daemon/daemon.go`
  - Fields: TaskID, TaskName, Project (strings), Healthy (*bool), LastCheck (time.Time), LastError (string)
  - JSON tags for API response
- [ ] **2.2** Add `taskHealth map[string]*TaskHealthStatus` and `taskHealthMu sync.RWMutex` to Daemon struct
- [ ] **2.3** Initialize `taskHealth` map in `New()` constructor
- [ ] **2.4** Implement `runHealthChecks()` method on Daemon
  - Tick every 5 seconds
  - For each watched project, load todos
  - For each todo with HealthCheck set: skip if disabled, skip if interval not elapsed
  - Execute command with 30s timeout via `exec.CommandContext`
  - Update taskHealth map: healthy=true if exit 0, healthy=false + capture stderr if non-zero
  - Exit on `d.stop` channel
- [ ] **2.5** Launch `go d.runHealthChecks()` in `Daemon.Run()` after socket server starts

### Phase 3: /health Endpoint Integration

- [ ] **3.1** Add `TaskHealth []TaskHealthStatus` field to `HealthResponse` struct (json: "task_health,omitempty")
- [ ] **3.2** In `handleHealth`, when detailed=true and config.Health.IncludeTasks:
  - Lock taskHealthMu, copy statuses into response, unlock
- [ ] **3.3** In `handleHealth`, when config.Health.UnhealthyThreshold > 0:
  - Count tasks where Healthy != nil && *Healthy == false
  - Set healthy=false if count >= UnhealthyThreshold

### Phase 4: CLI Command

- [ ] **4.1** Register `/task-health` handler on socket mux in `startSocketServer()`
  - Accept query params: `task` (filter), `check` (trigger immediate)
  - Return JSON array of TaskHealthStatus
- [ ] **4.2** For `check` param: run health check immediately for named task, return updated status
- [ ] **4.3** Add `task health` subcommand in `cmd/anvil/main.go`
  - Call daemon socket `/task-health`
  - Format as table: TASK, PROJECT, HEALTH, LAST_CHECK
  - Health column: "healthy" (green), "unhealthy" (red), "unknown" (yellow)
  - LAST_CHECK: relative time ("10s ago", "never")
- [ ] **4.4** Add `--check` flag to force immediate health check for a named task
- [ ] **4.5** Add `--verbose` flag to show LastError for unhealthy tasks

### Phase 5: Testing

- [ ] **5.1** Unit tests for HealthCheckInterval defaulting (60s when health_check set, 0 otherwise)
- [ ] **5.2** Unit tests for unhealthy threshold logic (0 = ignore, N = fail when N+ unhealthy)
- [ ] **5.3** Manual testing:
  - Task with `health_check: "true"` shows healthy
  - Task with `health_check: "false"` shows unhealthy
  - `anvil task health` displays correct table
  - `/health?detailed=true` includes task_health array
  - `--check` flag triggers and returns result

## Implementation Order

1. Phase 1 (Data Model) - no dependencies
2. Phase 2 (Health Check Runner) - depends on Phase 1
3. Phase 3 + Phase 4 in parallel - both depend on Phase 2
4. Phase 5 (Testing) - after Phase 3 and 4

## Notes

- Health status is in-memory only; resets on daemon restart (by design)
- Health check commands run in the task's project directory with the task's env vars
- 30s hardcoded timeout prevents hung health checks from blocking the runner
- Tasks without `health_check` are not tracked (don't appear in output)
- Disabled tasks are skipped for health checks
