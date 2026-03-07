# Quickstart: File Watcher Trigger for Tasks

**Feature**: 364-file-watch-trigger

## Basic Usage

Create a task that runs when JSON files appear in a directory:

```markdown
---
subscription:
  type: fs
  fs_path: ./data
  fs_glob: "*.json"
  fs_events:
    - create
  fs_debounce: 2s
---
Process new data files as they arrive.

```bash
echo "Processing ${ANVIL_FS_PATH}"
echo "Event: ${ANVIL_FS_EVENT}"
echo "Total files changed: ${ANVIL_FS_EVENT_COUNT}"
```

Save this as `.anvil/todos/process-data.md`, then start the daemon:

```bash
anvil watch
```

Drop a file into `./data/`:

```bash
echo '{"test": true}' > ./data/test.json
```

The task will trigger after the 2-second debounce window.

## Configuration Options

### Watch for all changes (default events)

```yaml
subscription:
  type: fs
  fs_path: ./config
  fs_glob: "*.yaml"
  fs_debounce: 5s
```

When `fs_events` is omitted, all event types are watched (create, modify, delete).

### Recursive directory watching

```yaml
subscription:
  type: fs
  fs_path: ./src
  fs_glob: "*.go"
  fs_events:
    - modify
  fs_recursive: true
  fs_debounce: 3s
```

### Batch processing with event details

The triggered task receives these environment variables:

| Variable              | Description                                   | Example                                    |
| --------------------- | --------------------------------------------- | ------------------------------------------ |
| `ANVIL_FS_EVENT`      | Last event type                               | `create`                                   |
| `ANVIL_FS_PATH`       | Last changed file path                        | `/home/user/project/data/new.json`         |
| `ANVIL_FS_EVENT_COUNT`| Number of files in the debounced batch        | `5`                                        |
| `ANVIL_FS_EVENTS`     | JSON array of all events in the batch         | `[{"path":"...","event":"create"},...]`    |
