# Data Model: Task Execution Snapshots

## Entities

### Snapshot

Represents a collection of files capturing the complete execution context for a single task run.

| Field | Type | Description |
|-------|------|-------------|
| TaskID | string | Unique identifier of the task |
| RunID | string | Unique identifier of the run |
| CreatedAt | time | When the snapshot was created |

### SnapshotFile

Individual file within a snapshot directory.

| Field | Type | Description |
|-------|------|-------------|
| Name | string | File name (config.yaml, env.yaml, prompt.txt, files.json, run_record.json) |
| Path | string | Relative path within snapshot directory |
| Size | int | File size in bytes |

### RetentionPolicy

Rules for automatic snapshot cleanup.

| Field | Type | Description |
|-------|------|-------------|
| MaxRuns | int | Maximum snapshots to retain per task (0 = unlimited) |
| MaxAge | duration | Maximum age before deletion |

## Storage Structure

```
.anvil/runs/<task-id>/
└── <run-id>/
    └── snapshot/
        ├── config.yaml      # Task frontmatter as YAML
        ├── env.yaml         # Environment variables
        ├── prompt.txt       # Expanded prompt
        ├── files.json       # Directory listing
        └── run_record.json # Run metadata (reuses RunRecord)
```

## Relationships

- **Task** 1:N **Snapshot** (one task can have many snapshots, one per run)
- **Snapshot** 1:5 **SnapshotFile** (each snapshot contains exactly 5 files)
- **RetentionPolicy** applies to **Task** snapshots
