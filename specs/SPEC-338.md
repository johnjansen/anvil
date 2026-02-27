# SPEC-338.md - Task Health Check Endpoint with Custom Health Logic

## Project Overview
- **Project**: anvil
- **Feature**: Custom per-task health checks with CLI visibility and /health endpoint integration
- **Issue**: #338
- **Goal**: Let users define shell-based health checks on tasks, run them periodically, surface status via CLI and HTTP endpoint

## Problem Statement

The existing `/health` endpoint only checks daemon-level health (workers available, draining state). Users need task-level health checks to verify that external dependencies are reachable, business conditions are satisfied, or task-specific infrastructure is healthy. There is no mechanism to define, run, or view per-task health status.

## Proposed Solution

### 1. Health Check Script (Task Frontmatter)

Add a `health_check` field to task frontmatter:

```yaml
---
schedule: "*/30 * * *"
health_check: "curl -sf https://api.example.com/health"
health_check_interval: 60s   # how often to run (default: 60s)
---
```

- Exit code 0 = healthy, non-zero = unhealthy
- The command runs in the task's working directory with the task's env vars
- Timeout for health check command: 30s (hardcoded, not configurable)

### 2. Health Status Storage

Health check results are stored in-memory on the daemon (not persisted to disk):

```go
type TaskHealthStatus struct {
    TaskID    string
    TaskName  string
    Project   string
    Healthy   bool
    LastCheck time.Time
    LastError string    // stderr from last failed check
}
```

On daemon restart, all tasks start as "unknown" until their first health check runs.

### 3. CLI Visibility

```bash
# Show health status of all tasks with health_check defined
$ anvil task health
TASK           PROJECT     HEALTH      LAST_CHECK
api-task       myproject   healthy     10s ago
data-task      myproject   unhealthy   30s ago
other-task     myproject   unknown     never

# Force an immediate health check
$ anvil task health my-task --check

# Show detailed output (includes last error)
$ anvil task health my-task --verbose
```

### 4. /health Endpoint Integration

When `detailed=true`, the existing `/health` endpoint includes task health:

```json
{
  "healthy": true,
  "workers_available": 3,
  "workers_total": 4,
  "tasks_running": 1,
  "task_health": [
    {"task": "api-task", "project": "myproject", "healthy": true, "last_check": "2026-02-28T10:00:00Z"},
    {"task": "data-task", "project": "myproject", "healthy": false, "last_check": "2026-02-28T09:59:30Z", "error": "connection refused"}
  ]
}
```

### 5. Global Config for Aggregation

```yaml
# .anvil/config.yaml
health:
  include_tasks: true          # include task health in /health endpoint
  unhealthy_threshold: 0       # 0 = task health doesn't affect daemon health status
```

When `unhealthy_threshold > 0`, the daemon's overall health becomes unhealthy if that many tasks are unhealthy.

## Technical Design

### Data Model

**New fields in Todo struct** (`internal/project/project.go`):
```go
type Todo struct {
    // ... existing fields ...
    HealthCheck         string        `yaml:"health_check,omitempty"`          // shell command
    HealthCheckInterval time.Duration `yaml:"health_check_interval,omitempty"` // default 60s
}
```

**New struct for health status** (`internal/daemon/daemon.go`):
```go
type TaskHealthStatus struct {
    TaskID    string    `json:"task_id"`
    TaskName  string    `json:"task"`
    Project   string    `json:"project"`
    Healthy   *bool     `json:"healthy"`     // nil = unknown/never checked
    LastCheck time.Time `json:"last_check"`
    LastError string    `json:"error,omitempty"`
}
```

**New fields in Daemon struct**:
```go
type Daemon struct {
    // ... existing fields ...
    taskHealth   map[string]*TaskHealthStatus  // key: "project:taskName"
    taskHealthMu sync.RWMutex
}
```

**New fields in Config struct** (`internal/config/config.go`):
```go
type HealthConfig struct {
    IncludeTasks       bool `yaml:"include_tasks"`
    UnhealthyThreshold int  `yaml:"unhealthy_threshold"` // 0 = don't affect daemon health
}
```

Add `Health HealthConfig` to the existing Config struct alongside `HealthPort`.

### Health Check Runner

A goroutine launched by the daemon that:
1. Iterates all watched tasks every second
2. For each task with `health_check` set, checks if `health_check_interval` has elapsed since last check
3. Runs the health check command with 30s timeout
4. Updates `taskHealth` map with result
5. Stops when daemon stops (via `d.stop` channel)

### /health Endpoint Changes

Modify `handleHealth` in daemon.go:
- When `detailed=true` and `health.include_tasks` is true, append `task_health` array to response
- When `unhealthy_threshold > 0`, count unhealthy tasks and factor into `healthy` field

### Socket Command for CLI

Add a new socket command handler for `task-health`:
- Returns JSON array of TaskHealthStatus
- Supports `--check` flag to trigger immediate check for a specific task
- CLI formats the response as a table

## Acceptance Criteria

- [ ] Tasks support `health_check` script in frontmatter
- [ ] Script exit code determines health (0=healthy, non-0=unhealthy)
- [ ] `anvil task health` shows health status table
- [ ] Health check runs on configurable interval (default 60s)
- [ ] `/health?detailed=true` can include task health status
- [ ] `anvil task health <name> --check` forces immediate check
- [ ] `unhealthy_threshold` config controls daemon health aggregation

## Files to Modify

1. `internal/project/project.go` - Add HealthCheck and HealthCheckInterval to Todo struct, parse from frontmatter
2. `internal/config/config.go` - Add HealthConfig struct and field to Config
3. `internal/daemon/daemon.go` - Add TaskHealthStatus, health check runner goroutine, extend /health handler
4. `cmd/anvil/main.go` - Add `task health` subcommand with table output

## Edge Cases

- **Task removed while health check running**: Check task still exists before storing result
- **Health check command hangs**: 30s timeout kills the process
- **No tasks have health_check**: `anvil task health` shows empty table, /health omits task_health
- **Daemon restart**: All health statuses reset to unknown
- **health_check_interval not set**: Default to 60s
- **Task disabled**: Skip health checks for disabled tasks
- **Multiple projects**: Health status keyed by "project:taskName" to avoid collisions
