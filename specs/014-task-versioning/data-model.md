# Data Model: Task Diff and Versioning

**Date**: 2026-02-27
**Feature**: 014-task-versioning

## Entities

### TaskVersion

A snapshot of a task file at a specific point in time.

| Field | Type | Description |
|-------|------|-------------|
| VersionNumber | int | Sequential version number (1, 2, 3...) |
| TaskName | string | Name of the task file (without extension) |
| Content | string | Full task file content at this version |
| ContentHash | string | SHA256 hash of the content |
| Timestamp | time.Time | When the version was created |
| Author | string | Who created or modified the task |
| Summary | string | Brief description of changes (e.g., "initial version", "restored from v1") |

### Storage Layout

```text
.anvil/
  versions/
    <task-name>/
      v1.json          # First version snapshot
      v2.json          # Second version snapshot
      ...
```

### JSON Schema (v1.json example)

```json
{
  "version_number": 1,
  "task_name": "my-task",
  "content": "---\nschedule: \"*/5 * * * *\"\n---\necho hello",
  "content_hash": "a1b2c3d4...",
  "timestamp": "2026-02-27T10:30:00Z",
  "author": "johnjansen",
  "summary": "initial version"
}
```

### Relationships

- **TaskVersion** belongs to a **Todo** (linked by task name)
- Multiple TaskVersions exist per task, ordered by VersionNumber
- Daemon tracks file hashes in-memory (`map[string]string`) to detect changes between ticks

### State Transitions

```text
Task Created  -> v1 snapshot (summary: "initial version")
Task Modified -> v(N+1) snapshot (summary: auto-generated from diff)
Task Restored -> v(N+1) snapshot (summary: "restored from vX")
Task Deleted  -> No action (versions preserved for reference)
```
