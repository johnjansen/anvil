# Task Execution Snapshots - Quick Start Guide

Task execution snapshots capture the complete runtime context for every task run, making it easy to debug failed executions by providing access to:

- Task configuration (frontmatter)
- Environment variables
- Expanded prompt
- Directory listing
- Run metadata

## Viewing Snapshots

### View Latest Snapshot
```bash
anvil task snapshot my-task
```

### View Specific Run
```bash
anvil task snapshot my-task --run abc123
```

### View Specific File
```bash
anvil task snapshot my-task --file prompt.txt
anvil task snapshot my-task --file env.yaml
```

## Comparing Snapshots

Compare two runs to understand what changed between them:

```bash
anvil task snapshot-diff my-task --run1 abc123 --run2 def456
```

## Snapshot Contents

Each snapshot contains these files:

- `config.yaml` - Task configuration (frontmatter settings)
- `env.yaml` - Resolved environment variables with their values
- `prompt.txt` - Expanded prompt text with variables substituted
- `files.json` - Directory listing of the task's working directory
- `run_record.json` - Execution metadata and results

## Automatic Capture

Snapshots are automatically captured for every task run (success or failure) and stored in:
```
.anvil/runs/<task-id>/<run-id>/snapshot/
```

## Retention

Snapshots are automatically pruned alongside existing run records based on your retention settings.