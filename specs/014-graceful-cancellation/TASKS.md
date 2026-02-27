# TASKS.md - Task Graceful Cancellation with Partial Results

## Task Breakdown

### Phase 1: Data Model Extensions

- [ ] **1.1** Update Todo struct in `internal/project/project.go`
  - Add `OnKill string` field for pre-kill hook
  - Add `GracefulTimeout time.Duration` field (default 30s)

- [ ] **1.2** Update RunRecord struct in `internal/project/project.go`
  - Add `PartialResults map[string]interface{}` field
  - Add `KillReason string` field ("user", "graceful", "force")
  - Add `ExitCode int` field

- [ ] **1.3** Add KillRequest struct in `internal/project/project.go`
  - Fields: TaskID, Graceful, Force, RequestedAt, Timeout

- [ ] **1.4** Add ParsePartialResults function in `internal/project/project.go`
  - Parse `##anvil:partial {json}` from output string
  - Return parsed map or nil

### Phase 2: CLI Updates

- [ ] **2.1** Modify `cmd/anvil/task_kill.go`
  - Add `--graceful, -g` flag (default true if neither flag set)
  - Add `--force, -f` flag
  - Add `--timeout, -t` flag for graceful timeout duration

- [ ] **2.2** Create `cmd/anvil/task_partial.go`
  - Command: `anvil task partial <name>`
  - Read last run record, display PartialResults
  - Show error if no partial results

- [ ] **2.3** Modify `cmd/anvil/task_run.go`
  - Add `--resume` flag
  - If set, read partial results from last run
  - Pass as environment variables to task

### Phase 3: Daemon Integration

- [ ] **3.1** Add kill request handling in `internal/daemon/daemon.go`
  - Accept kill requests with graceful/force flags
  - Track kill state per task

- [ ] **3.2** Add hook execution in `internal/daemon/daemon.go`
  - Execute on_kill hook before sending SIGTERM
  - Pass task ID and run ID to hook

- [ ] **3.3** Add partial capture in `internal/daemon/daemon.go`
  - Receive partial results from runner
  - Store in run record before finalizing

### Phase 4: Runner Integration

- [ ] **4.1** Add SIGTERM handling in `internal/runner/runner.go`
  - Catch SIGTERM signal
  - Set ANVIL_IS_KILLED=true for subprocess
  - Execute on_kill hook

- [ ] **4.2** Add ANVIL_IS_KILLED environment variable
  - Set before executing subprocess when graceful kill requested

- [ ] **4.3** Add partial output parsing in `internal/runner/runner.go`
  - Scan stdout/stderr for ##anvil:partial pattern
  - Send partial results to daemon

## Dependency Order

```
1.1 → 1.2 → 1.3 → 1.4 → 2.1 → 2.2 → 2.3 → 3.1 → 3.2 → 3.3 → 4.1 → 4.2 → 4.3
```

- Phase 1 must complete before Phase 2 (data models needed for CLI)
- Phase 2 must complete before Phase 3 (daemon needs CLI interface)
- Phase 3 must complete before Phase 4 (runner needs daemon coordination)

## Notes

- Tasks can be done in parallel within each phase
- Phase 1 tasks 1.1-1.4 are independent and can be done together
- Phase 2 tasks 2.1-2.3 are independent once Phase 1 is done
