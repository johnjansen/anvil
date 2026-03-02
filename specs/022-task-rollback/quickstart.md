# Quickstart: Task Rollback

## Overview

Task rollback lets you revert a task to a previous successful run state. List available restore points, preview changes, and restore all or specific files.

## Commands

### List Restore Points

```bash
anvil task rollback <task-name>
```

Shows all successful runs for a task:

```
RUN ID     TIMESTAMP       STATUS    OUTPUT SIZE
abc123     2026-02-27 10:00  success  1.2MB
def456     2026-02-27 09:00  success  1.1MB
ghi789     2026-02-27 08:00  success  1.1MB
```

### Restore to Specific Run

```bash
anvil task rollback <task-name> <run-id>
```

### Preview (Dry-Run)

```bash
anvil task rollback <task-name> <run-id> --dry-run
```

Shows what would happen without making changes.

### Restore Specific Files

```bash
anvil task rollback <task-name> <run-id> --files file1.json,file2.csv
```

## Configuration

Add an `on_rollback` hook to your task config:

```yaml
schedule: "*/30 * * *"
on_rollback: "echo 'Rolling back to {{ .RunID }}'"
```

The hook runs before files are restored. Template variables:
- `{{ .RunID }}` - the run ID being restored to
- `{{ .TaskName }}` - the task name

## Examples

```bash
# See what restore points exist
anvil task rollback api-poller

# Restore to most recent successful run (omit run-id)
anvil task rollback api-poller

# Restore to a specific run
anvil task rollback api-poller abc123def

# Preview without making changes
anvil task rollback api-poller abc123def --dry-run

# Restore only specific files
anvil task rollback api-poller abc123def --files output.json,metrics.csv
```

## Error Handling

- **No restore points**: "No successful runs found for task"
- **Invalid run ID**: "Run ID not found"
- **File not in restore point**: "File 'X' not found in restore point"
- **Hook failure**: Rollback aborted, error returned
