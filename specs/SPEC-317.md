# SPEC.md - Task Execution Cancellation with Partial Result Capture

## Project Overview
- **Project**: anvil
- **Feature**: Graceful task cancellation with partial result capture
- **Issue**: #317
- **Goal**: Allow users to gracefully cancel long-running tasks while capturing partial results

## Problem Statement

When cancelling a running task (`anvil task kill`), all progress is lost. For long-running tasks that have done significant work, users want to:
- Capture partial results before killing
- Save checkpoint state for manual resume
- Get summary of what was accomplished

## Proposed Solution

### 1. Graceful kill with capture

```bash
# Request graceful shutdown, capture state before killing
anvil task kill my-task --graceful
# or
anvil task kill my-task -g

# Force kill if graceful takes too long
anvil task kill my-task --force
```

### 2. Pre-kill hook

```yaml
---
on_kill: "echo 'Saving state...' && cp /tmp/work.json /tmp/work-partial.json"
---
```

The `on_kill` hook runs before the task is terminated, giving it a chance to save state.

### 3. Partial result capture

Tasks can emit partial results:

```
##anvil:partial {"records_processed": 500, "last_id": 1234}
```

On kill, the daemon captures partial output and stores it in the run record.

### 4. Resume from partial

```bash
# See partial results from last run
anvil task partial my-task

# Resume with partial data
anvil task run my-task --resume
```

### 5. Environment variables on kill

```
ANVIL_IS_KILLED=true
ANVIL_PARTIAL_RESULTS={"records_processed": 500}
```

Task can check `ANVIL_IS_KILLED` and save state accordingly.

## Technical Design

### Data Model

**Todo struct updates** (`internal/project/project.go`):
```go
type Todo struct {
    // ... existing fields ...
    OnKill          string        // hook: shell command to run before kill
    GracefulTimeout time.Duration // timeout for graceful shutdown (default 30s)
}
```

**RunRecord updates** (`internal/project/run_record.go`):
```go
type RunRecord struct {
    // ... existing fields ...
    WasKilled       bool
    IsGraceful      bool
    PartialResults  json.RawMessage // captured partial results
}
```

**TaskState updates** (`internal/project/project.go`):
```go
type TaskState struct {
    // ... existing fields ...
    IsGracefulKill  bool      // graceful kill in progress
    GracefulDeadline time.Time // when graceful timeout expires
}
```

### CLI Commands

**`anvil task kill <name> [flags]`**:
- Location: `cmd/anvil/task_kill.go` (modify existing)
- New flags:
  - `-g, --graceful` - Request graceful shutdown
  - `-f, --force` - Force kill after graceful timeout
  - `--timeout duration` - Graceful timeout (default 30s)
- Behavior:
  1. Send SIGTERM to task process
  2. Set `ANVIL_IS_KILLED=true` environment variable
  3. Run `on_kill` hook if defined
  4. Wait for graceful timeout
  5. If `--force` or timeout expires, send SIGKILL

**`anvil task partial <name>`**:
- Location: `cmd/anvil/task_partial.go` (new file)
- Shows partial results from last run if available
- Output format:
  ```
  Records processed: 500
  Last ID: 1234

  Run: abc123 (2026-02-27 10:00)
  Status: killed
  ```

**`anvil task run <name> --resume`**:
- Modify existing `cmd/anvil/task_run.go`
- Pass partial results as environment variable `ANVIL_PARTIAL_RESULTS`
- Task can read and use to resume

### Daemon Changes

**Signal handling** (`internal/daemon/runner.go`):
- On SIGTERM, check if graceful kill requested
- Set `ANVIL_IS_KILLED=true` in process environment
- Execute `on_kill` hook before termination
- Parse `##anvil:partial` from stdout and capture to run record

**Kill command handler** (`internal/daemon/daemon.go`):
- Add `/kill` endpoint with graceful flag
- Track graceful kill state per task
- Send signals through process group

### Partial Result Parsing

- Daemon monitors task stdout in real-time
- Detect lines matching `##anvil:partial {json}`
- Store latest partial result in memory
- On kill, save to run record as `partial_results.json`

## Acceptance Criteria

1. `anvil task kill --graceful` allows task to save state before termination
2. `on_kill` hook runs before termination
3. Partial results captured in run record
4. `anvil task partial` shows partial results from last run
5. `ANVIL_IS_KILLED` env var signals imminent termination
6. `--force` can override graceful timeout

## Files to Modify/Create

1. `internal/project/project.go` - Add OnKill, GracefulTimeout to Todo
2. `internal/project/run_record.go` - Add WasKilled, IsGraceful, PartialResults
3. `cmd/anvil/task_kill.go` - Add graceful/force flags
4. `cmd/anvil/task_partial.go` - New CLI command
5. `internal/daemon/daemon.go` - Add graceful kill endpoint
6. `internal/daemon/runner.go` - Add signal handling, partial parsing
7. `internal/daemon/api.go` - Add partial results to run record response

## Edge Cases

- **Task already terminating**: Handle gracefully, don't double-kill
- **No on_kill hook**: Skip hook execution, proceed with kill
- **No partial results**: Show "No partial results available"
- **Task completes before graceful timeout**: Normal exit, partial captured if any
- **Force kill during graceful**: Immediate SIGKILL, lose partial
- **Partial result too large**: Truncate at 64KB, warn user
