# SPEC.md - Task Graceful Cancellation with Partial Results

## Project Overview
- **Project**: anvil
- **Feature**: Task graceful cancellation with partial result capture
- **Issue**: #317
- **Goal**: Allow users to capture partial results when cancelling long-running tasks

## Problem Statement

When cancelling a running task (`anvil task kill`), all progress is lost. For long-running tasks that have done significant work, users want to:
- Capture partial results before killing
- Save checkpoint state for manual resume
- Get summary of what was accomplished

## Proposed Solution

### 1. Graceful Kill with Capture

```bash
# Request graceful shutdown, capture state before killing
anvil task kill my-task --graceful
# or
anvil task kill my-task -g

# Force kill if graceful takes too long
anvil task kill my-task --force
```

### 2. Pre-kill Hook

```yaml
---
on_kill: "echo 'Saving state...' && cp /tmp/work.json /tmp/work-partial.json"
---
```

The `on_kill` hook runs before the task is terminated, giving it a chance to save state.

### 3. Partial Result Capture

Tasks can emit partial results:

```
##anvil:partial {"records_processed": 500, "last_id": 1234}
```

On kill, the daemon captures partial output and stores it in the run record.

### 4. Resume from Partial

```bash
# See partial results from last run
anvil task partial my-task

# Resume with partial data
anvil task run my-task --resume
```

### 5. Environment Variables on Kill

```
ANVIL_IS_KILLED=true
ANVIL_PARTIAL_RESULTS={"records_processed": 500}
```

Task can check `ANVIL_IS_KILLED` and save state accordingly.

## Technical Design

### Data Model

**Updated Todo struct** (in `internal/project/project.go`):
```go
type Todo struct {
    // ... existing fields ...
    OnKill           string        // hook: shell command to run before task is killed
    GracefulTimeout  time.Duration // timeout for graceful shutdown (default 30s)
}
```

**RunRecord partial results** (in `internal/project/project.go`):
```go
type RunRecord struct {
    // ... existing fields ...
    PartialResults   map[string]interface{} // captured partial results
    KillReason       string                  // "user", "graceful", "force"
    ExitCode         int                     // 0 for graceful, -1 for force/killed
}
```

**KillRequest struct** (in `internal/project/project.go`):
```go
type KillRequest struct {
    TaskID        string
    Graceful      bool
    Force         bool
    RequestedAt   time.Time
}
```

### Signal Handling

1. **Graceful kill flow**:
   - User runs `anvil task kill --graceful`
   - CLI sends SIGTERM to runner process
   - Runner sets `ANVIL_IS_KILLED=true` env var for subprocess
   - Runner executes `on_kill` hook if defined
   - Runner waits for graceful timeout (default 30s)
   - If task still running after timeout, runs `anvil task kill --force`

2. **Force kill flow**:
   - User runs `anvil task kill --force` or timeout expires
   - CLI sends SIGKILL to runner process
   - Runner kills subprocess immediately
   - RunRecord saved with KillReason="force"

### Partial Results Parser

In `internal/project/project.go` - new function:
```go
// ParsePartialResults extracts ##anvil:partial JSON from output
func ParsePartialResults(output string) map[string]interface{}
```

- Searches for `##anvil:partial {json}` pattern in stdout/stderr
- Parses JSON and stores in RunRecord.PartialResults
- Handles multiline JSON

### CLI Commands

**`anvil task kill <name> [flags]`**:
- Location: Modify existing `cmd/anvil/task_kill.go`
- New flags:
  - `-g, --graceful` - graceful shutdown (default true if neither specified)
  - `-f, --force` - force kill immediately
  - `-t, --timeout duration` - graceful timeout (default 30s)

**`anvil task partial <name>`**:
- Location: New `cmd/anvil/task_partial.go`
- Shows partial results from most recent run
- Output format:
  ```
  Partial results from run abc123 (2026-02-27 10:00):
  {
    "records_processed": 500,
    "last_id": 1234
  }
  ```

**`anvil task run <name> --resume`**:
- Location: Modify existing `cmd/anvil/task_run.go`
- Passes partial results as environment variables:
  - `ANVIL_PARTIAL_RESULTS` - JSON string of partial data
  - `ANVIL_RESUMED_FROM` - run ID of partial run

### Daemon Integration

In `internal/daemon/daemon.go`:
- Add kill request handling
- Track active kill requests per task
- Execute on_kill hook before sending SIGTERM
- Capture partial results from runner output

### Runner Integration

In `internal/runner/runner.go`:
- Add signal handling for SIGTERM
- Set `ANVIL_IS_KILLED=true` for subprocess
- Execute on_kill hook on SIGTERM
- Parse and emit partial results to daemon
- Handle graceful timeout

## Acceptance Criteria

1. [ ] `anvil task kill --graceful` allows task to save state before termination
2. [ ] `on_kill` hook runs before termination
3. [ ] Partial results captured in run record
4. [ ] `anvil task partial` shows partial results from last run
5. [ ] `ANVIL_IS_KILLED` env var signals imminent termination

## Files to Modify/Create

1. `internal/project/project.go` - Add OnKill, GracefulTimeout to Todo; add PartialResults to RunRecord; add KillRequest; add ParsePartialResults
2. `cmd/anvil/task_kill.go` - Add --graceful, --force, --timeout flags
3. `cmd/anvil/task_partial.go` - New CLI command
4. `cmd/anvil/task_run.go` - Add --resume flag
5. `internal/daemon/daemon.go` - Add kill request handling, hook execution
6. `internal/runner/runner.go` - Add signal handling, ANVIL_IS_KILLED env var, partial output parsing

## Edge Cases

- **No on_kill hook defined**: Skip hook execution, proceed with kill
- **Task already stopped**: Return error "task not running"
- **Graceful timeout expires**: Auto-force kill after timeout
- **No partial results**: Show "No partial results available"
- **Invalid partial JSON**: Log warning, continue without partial capture
- **Hook fails**: Log error but proceed with kill
