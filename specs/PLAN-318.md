# Implementation Plan: Task Activity Log (#318)

## Overview

Add a task activity log that records lifecycle events (create, run, pause, resume, edit, kill, unlock, etc.) and exposes them via `anvil task activity <name>` with filtering and export options.

## Implementation Steps

### Phase 1: Data Model & Storage

**1.1 Add ActivityEntry struct and storage functions**
- File: `internal/project/project.go`
- Add `ActivityEntry` struct with `Timestamp`, `Action`, `Details` fields
- Add `AppendActivity(projectPath, taskID string, entry ActivityEntry) error`
  - Creates `.anvil/tasks/<task-id>/` directory if needed (same as risk.go pattern)
  - Reads existing entries, appends new one, writes back
- Add `ReadActivity(projectPath, taskID string) ([]ActivityEntry, error)`
  - Returns entries sorted newest-first
  - Returns empty slice (not error) if file doesn't exist

### Phase 2: CLI Command

**2.1 Add `taskActivityCmd` function**
- File: `cmd/anvil/main.go`
- Add `case "activity":` to `taskCmd` switch
- Implement `taskActivityCmd(args []string)`:
  - Parse flags: `--type`, `--since`, `--json`, `--export`
  - Load project, find todo by name, get task ID
  - Call `project.ReadActivity()`
  - Apply filters
  - Render as table (default) or JSON
- Add to `printUsage()` task subcommands section

### Phase 3: Instrument Existing Commands

**3.1 Instrument CLI commands in main.go**
- `taskCreateCmd`: append `created` activity after successful creation
- `taskPauseCmd`: append `paused` activity after writing disable flag
- `taskResumeCmd`: append `resumed` activity after clearing disable flag
- `taskEditCmd`: append `edited` activity with field-level diffs (compare before/after Todo)
- `taskKillCmd`: append `killed` activity after successful kill request
- `taskUnlockCmd`: append `unlocked` activity after lock removal
- `taskStopCmd`: append `stopped` activity after daemon confirms stop
- `taskStartCmd`: append `started` activity after daemon confirms start
- `taskResetBudgetCmd`: append `budget_reset` activity with budget type

**3.2 Instrument daemon in daemon.go**
- After `executeTask` completes (where `WriteRunRecord` is called): append `run` activity with `run_id` and `exit_code`
- In `handleRun`: append `force_run` activity with `run_id`

### Phase 4: Edit Diffing

**4.1 Add edit diff helper**
- File: `cmd/anvil/main.go` (within `taskEditCmd`)
- Before applying edits, snapshot the Todo's relevant fields
- After applying edits, compare and build `Details` map with `field: "old -> new"` entries
- Only log activity if at least one field changed

## Dependencies

- Phase 2 depends on Phase 1 (needs storage functions)
- Phase 3 depends on Phase 1 (needs `AppendActivity`)
- Phase 4 depends on Phase 3 (modifies edit instrumentation)
- Phase 3.1 and 3.2 are independent of each other

## Testing Strategy

- Manual testing: create task, perform operations, verify `anvil task activity` output
- Verify filter flags work correctly
- Verify JSON and export output
- Verify edit diffs capture field changes
