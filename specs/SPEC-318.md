# SPEC.md - Task Activity Log for Debugging and Auditing

## Project Overview
- **Project**: anvil
- **Feature**: Task activity log tracking all state changes and interventions
- **Issue**: #318
- **Goal**: Provide a complete audit trail of task lifecycle events beyond run records

## Problem Statement

Currently, users can only see run records (success/failure/timing) via `anvil task history`. There's no way to see:
- When a task was created, paused, resumed, edited, killed, or unlocked
- What configuration changed during edits
- Manual interventions like force-run or budget resets
- A unified timeline combining runs with administrative actions

This information is needed for debugging task behavior and compliance auditing.

## Proposed Solution

### 1. Activity Entry Model

Each activity entry captures a single event in a task's lifecycle:

```go
// In internal/project/project.go
type ActivityEntry struct {
    Timestamp time.Time         `json:"timestamp"`
    Action    string            `json:"action"`    // "created", "run", "paused", "resumed", "edited", "killed", "unlocked", "force_run", "budget_reset", "started", "stopped"
    Details   map[string]string `json:"details,omitempty"` // action-specific key-value pairs
}
```

**Actions tracked:**
| Action | Details | Trigger Location |
|--------|---------|-----------------|
| `created` | — | `taskCreateCmd` in main.go |
| `run` | `run_id`, `exit_code` | daemon `executeTask` completion |
| `paused` | — | `taskPauseCmd` in main.go |
| `resumed` | — | `taskResumeCmd` in main.go |
| `edited` | changed field names + old→new values | `taskEditCmd` in main.go |
| `killed` | — | `taskKillCmd` / daemon `handleKill` |
| `unlocked` | — | `taskUnlockCmd` in main.go |
| `force_run` | `run_id` | daemon `handleRun` |
| `started` | — | `taskStartCmd` (clear stopped state) |
| `stopped` | — | `taskStopCmd` (mark stopped) |
| `budget_reset` | `budget_type` | `taskResetBudgetCmd` |

### 2. Storage

Activity entries stored as a JSON array file per task:

```
<project>/.anvil/tasks/<task-id>/activity.json
```

This reuses the existing `.anvil/tasks/<task-id>/` directory (already used by `risk.json`).

Helper functions:
- `AppendActivity(projectPath, taskID string, entry ActivityEntry) error`
- `ReadActivity(projectPath, taskID string) ([]ActivityEntry, error)`

Entries are append-only. The file is read in full and rewritten on each append (acceptable for audit log sizes). No rotation needed initially — activity entries are small.

### 3. CLI Command

```bash
$ anvil task activity my-task
TIMESTAMP              ACTION        DETAILS
2026-02-27 10:00:00    run           run_id=abc123, exit_code=0
2026-02-27 09:30:00    edited        schedule: "0 9 * * *" -> "0 10 * * *"
2026-02-27 09:00:00    run           run_id=def456, exit_code=1
2026-02-27 08:00:00    paused
2026-02-27 07:00:00    created
```

**Flags:**
- `--type <action>` — filter by action type (e.g., `--type run`, `--type edited`)
- `--since <date>` — show entries since date (RFC3339 or `YYYY-MM-DD`)
- `--json` — output as JSON array
- `--export <path>` — write JSON to file

### 4. Implementation in CLI

Add `case "activity":` to `taskCmd` switch in `cmd/anvil/main.go`. The command:
1. Loads project and resolves task name → ID
2. Calls `project.ReadActivity(projectPath, taskID)`
3. Applies filters (type, since)
4. Renders table or JSON output

### 5. Instrumentation Points

Each existing command that modifies task state gets a one-line `project.AppendActivity()` call:

| File | Function | Activity |
|------|----------|----------|
| `cmd/anvil/main.go` | `taskCreateCmd` | `created` |
| `cmd/anvil/main.go` | `taskPauseCmd` | `paused` |
| `cmd/anvil/main.go` | `taskResumeCmd` | `resumed` |
| `cmd/anvil/main.go` | `taskEditCmd` | `edited` (with field diffs) |
| `cmd/anvil/main.go` | `taskKillCmd` | `killed` |
| `cmd/anvil/main.go` | `taskUnlockCmd` | `unlocked` |
| `cmd/anvil/main.go` | `taskStopCmd` | `stopped` |
| `cmd/anvil/main.go` | `taskStartCmd` | `started` |
| `cmd/anvil/main.go` | `taskResetBudgetCmd` | `budget_reset` |
| `internal/daemon/daemon.go` | `executeTask` (post-run) | `run` |
| `internal/daemon/daemon.go` | `handleRun` | `force_run` |

## Acceptance Criteria

1. `anvil task activity <name>` shows complete activity history in reverse chronological order
2. Tracks: create, run, pause, resume, edit, kill, unlock, start, stop, force_run, budget_reset
3. Edit entries show field-level changes (old → new values)
4. `--type` filters by activity type
5. `--since` filters by date
6. `--json` outputs machine-readable JSON
7. `--export <path>` writes JSON to file

## Files to Modify/Create

1. `internal/project/project.go` — Add `ActivityEntry` struct, `AppendActivity`, `ReadActivity` functions
2. `cmd/anvil/main.go` — Add `taskActivityCmd`, add `case "activity"` to `taskCmd`, instrument existing commands
3. `internal/daemon/daemon.go` — Add activity logging after task execution and force-run

## Edge Cases

- **No activity file exists**: Return empty list, don't error
- **Task ID not found**: Error with "task not found" message
- **Concurrent writes**: Use file locking or accept last-writer-wins (activity log is append-only, concurrent appends are rare)
- **Large activity files**: For v1, no rotation. Activity entries are ~200 bytes each; 10,000 entries ≈ 2MB. Revisit if needed.
- **Edit with no changes**: Don't log an activity entry
- **Invalid --since date**: Error with format hint
