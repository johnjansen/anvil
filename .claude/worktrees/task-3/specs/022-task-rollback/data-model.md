# Data Model: Task Rollback

## Entities

### RollbackEvent

Records when a rollback occurred, who initiated it, and which files were restored.

| Field | Type | Description |
|-------|------|-------------|
| RunID | string | ID of the run that was rolled back FROM |
| TaskID | string | Name of the task |
| RolledBackTo | string | RunID of the restore point |
| Timestamp | time.Time | When the rollback occurred |
| UserInitiated | bool | Whether user triggered manually |
| FilesRestored | []string | List of files that were restored |

**Storage**: `.anvil/runs/<task-id>/rollbacks.json` (append-only log)

---

### RunRecord (existing)

Used as-is. Contains all data needed for rollback.

| Field | Type | Description |
|-------|------|-------------|
| RunID | string | Unique identifier |
| TaskID | string | Task name |
| Started | time.Time | When run started |
| Success | bool | Whether run succeeded |
| OutputSummary | string | First/last N lines of output |

**Storage**: `.anvil/runs/<task-id>/<run-id>.json`

---

### Todo (existing, extended)

Extended with new field.

| Field | Type | Description |
|-------|------|-------------|
| OnRollback | string | Shell command to run before rollback (NEW) |

---

## Relationships

```
Task (Todo)
  └── OnRollback hook → shell command
  └── RunRecord[] → stored in .anvil/runs/<task-id>/
        └── each run is a restore point
  └── RollbackEvent[] → stored in .anvil/runs/<task-id>/rollbacks.json
```

## State Transitions

N/A - this is a command-based feature, not a state machine.

## Validation Rules

- Rollback only allowed to successful runs (RunRecord.Success == true)
- Target run must exist
- Files to restore must exist in target run's output
- OnRollback hook must be valid shell command (can be empty)
- Dry-run mode must not modify any files
