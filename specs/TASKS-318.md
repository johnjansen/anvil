# Tasks: Task Activity Log (#318)

## Phase 1: Data Model & Storage

### Task 1.1: Add ActivityEntry struct and storage functions
- **File:** `internal/project/project.go`
- **Description:** Add `ActivityEntry` struct, `AppendActivity()`, and `ReadActivity()` functions. Storage at `.anvil/tasks/<task-id>/activity.json`.
- **Dependencies:** None
- **Estimate:** 1h

## Phase 2: CLI Command

### Task 2.1: Add `anvil task activity` command
- **File:** `cmd/anvil/main.go`
- **Description:** Add `case "activity"` to `taskCmd` switch. Implement `taskActivityCmd` with `--type`, `--since`, `--json`, `--export` flags. Table and JSON output formats.
- **Dependencies:** 1.1
- **Estimate:** 2h

### Task 2.2: Add activity to usage/help text
- **File:** `cmd/anvil/main.go`
- **Description:** Add `activity` to the task subcommands section in `printUsage()`
- **Dependencies:** 2.1
- **Estimate:** 0.1h

## Phase 3: Instrument Existing Commands

### Task 3.1: Instrument taskCreateCmd
- **File:** `cmd/anvil/main.go`
- **Description:** Append `created` activity after successful task creation
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 3.2: Instrument taskPauseCmd and taskResumeCmd
- **File:** `cmd/anvil/main.go`
- **Description:** Append `paused` and `resumed` activities after state changes
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 3.3: Instrument taskKillCmd
- **File:** `cmd/anvil/main.go`
- **Description:** Append `killed` activity after successful kill request
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 3.4: Instrument taskUnlockCmd
- **File:** `cmd/anvil/main.go`
- **Description:** Append `unlocked` activity after lock removal
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 3.5: Instrument taskStopCmd and taskStartCmd
- **File:** `cmd/anvil/main.go`
- **Description:** Append `stopped` and `started` activities after daemon confirms
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 3.6: Instrument taskResetBudgetCmd
- **File:** `cmd/anvil/main.go`
- **Description:** Append `budget_reset` activity with budget type details
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 3.7: Instrument taskEditCmd with field diffs
- **File:** `cmd/anvil/main.go`
- **Description:** Snapshot Todo before edit, compare after, append `edited` activity with changed field names and old→new values. Skip if no changes.
- **Dependencies:** 1.1
- **Estimate:** 1h

### Task 3.8: Instrument daemon executeTask
- **File:** `internal/daemon/daemon.go`
- **Description:** After `WriteRunRecord` call in task execution path, append `run` activity with `run_id` and `exit_code`
- **Dependencies:** 1.1
- **Estimate:** 0.5h

### Task 3.9: Instrument daemon handleRun (force-run)
- **File:** `internal/daemon/daemon.go`
- **Description:** Append `force_run` activity when a manual run is triggered via `/run` endpoint
- **Dependencies:** 1.1
- **Estimate:** 0.25h

## Summary

| Phase | Tasks | Estimated Time |
|-------|-------|---------------|
| 1. Data Model & Storage | 1 | 1h |
| 2. CLI Command | 2 | 2.1h |
| 3. Instrument Commands | 9 | 3h |
| **Total** | **12** | **~6h** |
