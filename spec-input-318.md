## Problem

Currently, there's no easy way to see the complete activity history of a task beyond run records. Users want to see:
- All state changes (paused, resumed, edited)
- Configuration changes over time
- Manual interventions (kill, unlock, force-run)

This is needed for debugging and compliance auditing.

## Proposed Solution

Add task activity log:

### 1. Activity tracking

Each task maintains an activity log with entries for:
- Task created
- Task run (with run ID)
- Task paused/resumed
- Task edited (with field changes)
- Task killed
- Task unlocked
- Task force-run

### 2. CLI command

```bash
$ anvil task activity my-task
TIMESTAMP            ACTION      DETAILS
2026-02-27 10:00    run         run_id=abc123, exit=0
2026-02-27 09:30    edited      schedule: "0 9 * * *" -> "0 10 * * *"
2026-02-27 09:00    run         run_id=def456, exit=1 (timeout)
2026-02-27 08:00    paused      by user
2026-02-27 07:00    created
```

### 3. Filter options

```bash
# Only show runs
anvil task activity my-task --type run

# Only show config changes
anvil task activity my-task --type edit

# Show since date
anvil task activity my-task --since 2026-01-01
```

### 4. Audit export

```bash
# Export for compliance
anvil task activity my-task --export audit.json
```

## Acceptance Criteria

- [ ] `anvil task activity <name>` shows complete activity history
- [ ] Tracks: create, run, pause, resume, edit, kill, unlock
- [ ] Shows field changes on edit
- [ ] Filter by activity type
- [ ] Export to JSON
