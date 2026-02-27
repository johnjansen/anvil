# Tasks: Task Health Based on Run History (#335)

## Dependencies

- All tasks depend on: Phase 1 (Data Model)
- Phase 3 (Run Completion) depends on Phase 1 & 2
- Phase 4 (CLI) depends on Phase 2

## Phase 1: Data Model (no dependencies)

### 1.1 Add health check config to Todo struct
- **File**: `internal/project/project.go`
- **Description**: Add HealthCheckConfig struct with FailureThreshold, SuccessThreshold, Timeout fields. Add HealthCheck *HealthCheckConfig field to Todo struct. Parse from frontmatter via yaml tags.
- **Acceptance**: Todo struct has HealthCheck field, yaml parsing works

### 1.2 Create health status persistence directory
- **File**: `internal/daemon/daemon.go`
- **Description**: Ensure `.anvil/health/` directory exists on daemon startup
- **Acceptance**: Directory created if missing

## Phase 2: Health Status Management (no dependencies)

### 2.1 Add TaskHealthStatus type and tracking map
- **File**: `internal/daemon/daemon.go`
- **Description**: Define TaskHealthStatus struct with all required fields. Add taskHealth map and taskHealthMu to Daemon struct. Initialize map in New().
- **Acceptance**: Daemon has health tracking map, mutex initialized

### 2.2 Add health status persistence methods
- **File**: `internal/daemon/daemon.go`
- **Description**: Implement loadHealthStatus, saveHealthStatus, loadAllHealthStatus, deleteHealthStatus methods for reading/writing JSON files in .anvil/health/
- **Acceptance**: Health status persists across daemon restarts

### 2.3 Load health status on daemon startup
- **File**: `internal/daemon/daemon.go`
- **Description**: Call loadAllHealthStatus in Run() after daemon initializes
- **Acceptance**: Previous health status restored on restart

## Phase 3: Run Completion Handler (depends on Phase 1 & 2)

### 3.1 Hook into task completion
- **File**: `internal/daemon/daemon.go`
- **Description**: Find where task runs complete (taskCompleted or similar). After run finishes, get success/failure from run record. Call updateHealthStatus with result.
- **Acceptance**: Health status updated after each task run

### 3.2 Implement health status update logic
- **File**: `internal/daemon/daemon.go`
- **Description**: Implement updateHealthStatus method:
  - On failure: increment ConsecutiveFailures, reset ConsecutiveSuccesses to 0
  - On success: increment ConsecutiveSuccesses, reset ConsecutiveFailures to 0
  - Healthy = ConsecutiveSuccesses >= SuccessThreshold
  - Unhealthy = ConsecutiveFailures >= FailureThreshold
  - Handle timeout: if duration > Timeout, treat as failure
  - Save to disk after update
- **Acceptance**: Correct evaluation of health based on thresholds

### 3.3 Handle task deletion
- **File**: `internal/daemon/daemon.go`
- **Description**: When task is removed, call deleteHealthStatus
- **Acceptance**: No orphaned health files

## Phase 4: CLI Command (depends on Phase 2)

### 4.1 Add socket command handler for task-health
- **File**: `internal/daemon/daemon.go`
- **Description**: Register /task-health handler on socket mux. Return JSON array of TaskHealthStatus. Support task query param for filtering.
- **Acceptance**: Socket returns health data as JSON

### 4.2 Add `task health` subcommand
- **File**: `cmd/anvil/main.go`
- **Description**: New subcommand under `task` that calls daemon socket /task-health. Format as table with columns: TASK, HEALTH, CONSECUTIVE FAILURES, LAST_CHECK
- **Acceptance**: Shows table of all task health statuses

### 4.3 Add `task health <task>` detail view
- **File**: `cmd/anvil/main.go`
- **Description**: When task name provided, show detailed view with all fields including LastError
- **Acceptance**: Shows detailed health for single task

### 4.4 Add `--reset` flag
- **File**: `cmd/anvil/main.go` and daemon handler
- **Description**: Add --reset flag to clear health status (set consecutive counts to 0, healthy to true)
- **Acceptance**: Health status can be reset via CLI
