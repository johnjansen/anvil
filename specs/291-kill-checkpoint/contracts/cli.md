# CLI Contract: Task Kill with Checkpoint

## Command: `anvil task kill`

### Extended Usage

```
anvil task kill <task-name> [--checkpoint|-c]
```

### Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| --checkpoint | -c | bool | false | Gracefully stop task and save checkpoint |

### Behavior

#### Without `--checkpoint` (existing behavior, unchanged)

```bash
$ anvil task kill my-task
Killed task: my-task
```

#### With `--checkpoint`

**Success case:**
```bash
$ anvil task kill my-task --checkpoint
Gracefully stopping task: my-task (waiting up to 30s for checkpoint save)...
Task stopped with checkpoint: my-task
```

**Task not checkpoint-enabled:**
```bash
$ anvil task kill my-task --checkpoint
Error: task "my-task" does not have checkpoint enabled
```

**Task not running:**
```bash
$ anvil task kill my-task --checkpoint
Error: task "my-task" is not running
```

**Grace period expired:**
```bash
$ anvil task kill my-task --checkpoint
Gracefully stopping task: my-task (waiting up to 30s for checkpoint save)...
Warning: grace period expired, force-killing task: my-task
```

**Already shutting down:**
```bash
$ anvil task kill my-task --checkpoint
Task "my-task" is already shutting down gracefully
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Task killed/stopped successfully |
| 1 | Error (task not found, not running, checkpoint not enabled) |

## Daemon API: POST /kill

### Request

```json
{
  "id": "task-id",
  "checkpoint": true
}
```

### Response (200 OK)

```json
{
  "status": "ok",
  "task": "my-task",
  "checkpoint": true
}
```

### Response (400 Bad Request - checkpoint not enabled)

```json
{
  "error": "task does not have checkpoint enabled"
}
```

## Command: `anvil task history`

### Extended Output

```
$ anvil task history my-task
STARTED              DURATION  ATTEMPTS  RUNNER  NODE  STATUS
2026-03-07 10:00:00  5m30s     1         sh      local stopped-with-checkpoint
  checkpoint: {"last_item":5000,"total":10000}
2026-03-07 09:00:00  10m15s    1         sh      local ok
2026-03-06 18:00:00  1m02s     1         sh      local error: timeout
```

The "stopped-with-checkpoint" status appears in the STATUS column. Checkpoint data preview shown on next line (existing behavior for checkpoint display).
