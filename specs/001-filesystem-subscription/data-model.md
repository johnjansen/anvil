# Data Model: Filesystem Subscription for Task Triggers

**Feature**: Filesystem Subscription for Task Triggers
**Date**: 2026-03-01

## Entities

### FileEvent

Represents a filesystem change event.

| Field | Type | Description |
|-------|------|-------------|
| Path | string | Full path to the file that triggered the event |
| EventType | string | Type of event: "create", "modify", "delete" |
| Timestamp | int64 | Unix timestamp of when the event occurred |
| Pattern | string | The glob pattern that matched this file |

### FileSubscription

Configuration for a filesystem subscription on a task.

| Field | Type | Description |
|-------|------|-------------|
| TaskID | string | ID of the task this subscription belongs to |
| TaskName | string | Name of the task |
| PathPattern | string | Glob pattern to match files against |
| WatchPath | string | Directory to watch (can be relative to project) |
| Events | []string | Event types to watch: "create", "modify", "delete" |
| Enabled | bool | Whether the subscription is active |

### FileEventData

Data passed to a triggered task via environment variables.

| Field | Env Variable | Description |
|-------|--------------|-------------|
| FilePath | ANVIL_FS_PATH | Path to the file that triggered the event |
| EventType | ANVIL_FS_EVENT | Type of event: create, modify, delete |
| Timestamp | ANVIL_FS_TIMESTAMP | Unix timestamp of the event |
| Pattern | ANVIL_FS_PATTERN | Pattern that matched this file |

## Configuration Schema

Following the schema defined in spec 016-task-subscriptions:

```yaml
subscription:
  type: fs
  path: ./data/*.json              # Glob pattern to watch
  events: [create, modify, delete] # Events to watch (default: [create, modify])
```

## State Transitions

```
FileSubscription States:
  - Disabled → Enabled: When subscription is created or resumed
  - Enabled → Disabled: When subscription is paused
  - Enabled → Error: When watch directory becomes inaccessible
  - Error → Enabled: When watch directory becomes accessible again
```

## Validation Rules

1. PathPattern must be a valid glob pattern
2. WatchPath must be a valid directory path (can be created on demand)
3. Events must contain at least one of: create, modify, delete
4. TaskID must reference an existing task

## Relationships

- FileSubscription belongs to one Task (via TaskID)
- FileSubscription generates FileEvent(s) when matching filesystem events occur
- FileEvent triggers Task execution with FileEventData
