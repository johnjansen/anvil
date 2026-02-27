# Data Model: Task Activity Log

## Entities

### ActivityEntry
A single recorded event in a task's lifecycle.

| Field | Type | Description |
|-------|------|-------------|
| Timestamp | time.Time | When the event occurred (UTC) |
| Action | string | Activity type: "created", "run", "paused", "resumed", "edited", "killed", "unlocked", "force-run" |
| TaskID | string | UUID of the task |
| TaskName | string | Human-readable task name |
| Details | map[string]string | Action-specific key-value pairs |

### Action-Specific Details

| Action | Details Keys | Description |
|--------|-------------|-------------|
| created | priority, schedule | Task creation metadata |
| run | run_id, exit_code, success, error, duration | Run outcome |
| paused | — | Task was disabled |
| resumed | — | Task was re-enabled |
| edited | changed_fields, old_<field>, new_<field> | Field changes with old/new values |
| killed | graceful | Whether kill was graceful (true) or forced (false) |
| unlocked | — | Stale lock removed |
| force-run | — | Manual forced execution |

### ActivityLog (Virtual)
An ordered collection of ActivityEntry records for a specific task, stored as JSONL file at .anvil/activities/<task-id>.jsonl. One JSON object per line, newest entries appended at end. Reading reverses order for display (newest first).

## Validation Rules
- Timestamp must be non-zero
- Action must be one of the defined types
- TaskID must be non-empty
- TaskName must be non-empty

## State Transitions
None — ActivityEntry is append-only, never modified or deleted.
