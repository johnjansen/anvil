# Data Model: Task Execution Timeout Extension

## Entities

### AutoExtendConfig

Per-task configuration for automatic timeout extension.

| Field | Type | Description |
|-------|------|-------------|
| Enabled | boolean | Whether auto-extend is active for this task |
| MaxExtensions | integer | Maximum number of automatic extensions allowed (default: 3) |
| ExtensionDuration | duration | How much time each auto-extension adds (e.g., "15m") |

**Source**: Task frontmatter YAML (`auto_extend` block)
**Relationship**: Belongs to Todo (one-to-one, optional)

### TimeoutState (runtime)

In-memory state tracking timeout and extensions for a running task.

| Field | Type | Description |
|-------|------|-------------|
| OriginalTimeout | duration | The timeout value at task start (before any extensions) |
| CurrentDeadline | timestamp | The current effective deadline (updated on each extension) |
| ExtensionCount | integer | Number of times the timeout has been extended |
| TotalExtended | duration | Total time added through extensions |
| TimeoutTimer | timer | The active timer that enforces the deadline |
| WarningTimer | timer | Timer for on_timeout_warning hook (nil if not configured) |
| WarningFired | boolean | Whether the warning has been fired for the current deadline |
| LastCheckpointTime | timestamp | When the most recent checkpoint was emitted |

**Source**: In-memory on Daemon (part of RunningTask)
**Lifecycle**: Created at task start, updated on extension, discarded on task completion

### RunRecord Extension Fields (persisted)

Additional fields added to the existing RunRecord for post-run analysis.

| Field | Type | Description |
|-------|------|-------------|
| OriginalTimeout | duration | The timeout at task start |
| FinalTimeout | duration | The effective timeout at task completion |
| ExtensionCount | integer | Total extensions applied during the run |
| TotalExtended | duration | Total time added through extensions |
| AutoExtensions | integer | How many extensions were automatic (vs manual) |

**Source**: Written to `.anvil/runs/<task-id>/<run-id>.json` on task completion
**Relationship**: Part of RunRecord (one-to-one)

## State Transitions

### Timeout State Machine

```
[Task Started]
    |
    v
[Running: timer active]
    |
    +-- checkpoint detected within warning window
    |       |
    |       +-- auto-extend available? → [Extended: new timer] → back to Running
    |       +-- auto-extend exhausted? → no action
    |
    +-- warning window reached
    |       |
    |       +-- on_timeout_warning hook fires
    |
    +-- CLI extend-timeout command
    |       |
    |       v
    |   [Extended: new timer] → back to Running
    |
    +-- timer expires
    |       |
    |       v
    |   [Timed Out: cancel() called]
    |
    +-- task completes normally
            |
            v
        [Completed: timer stopped, state persisted to RunRecord]
```

### Extension Modes

| Mode | Trigger | Duration Calculation |
|------|---------|---------------------|
| Manual (default) | `anvil task extend-timeout <name> <dur>` | deadline = now + remaining + duration |
| Manual (absolute) | `anvil task extend-timeout <name> <dur> --absolute` | deadline = now + duration |
| Automatic | Checkpoint detected within warning window | deadline = now + remaining + extension_duration |
