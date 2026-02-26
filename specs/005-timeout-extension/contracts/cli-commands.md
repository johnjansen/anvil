# CLI Contracts: Timeout Extension Commands

## `anvil task extend-timeout <name> <duration> [--absolute]`

Extends the timeout for a currently running task.

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| name | yes | Task name (matched against running tasks) |
| duration | yes | Duration to extend by (e.g., "5m", "1h", "30s") |
| --absolute | no | Set deadline to `duration` from now instead of adding to remaining |

### Success Output

```
Timeout extended for <name>: <new_deadline> (<remaining> remaining, extension #<count>)
```

### Error Outputs

```
task not found: <name>                    # no running task with this name
task <name> has no timeout configured     # task is running without timeout
invalid duration: <input>                 # duration parsing failed
duration must be positive                 # zero or negative duration
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Extension successful |
| 1 | Error (task not found, invalid input, etc.) |

---

## `anvil task timeout [name] [--all] [--json]`

Shows timeout progress and extension info for running tasks.

### Enhanced Output Format

```
TASK              ELAPSED    TIMEOUT    REMAINING   EXTENSIONS   PROGRESS
my-task           15m32s     30m        14m28s      1x +15m      ████░░░░░░ 52%
```

### JSON Output (with --json)

```json
{
  "name": "my-task",
  "elapsed": "15m32s",
  "original_timeout": "30m",
  "current_timeout": "45m",
  "remaining": "29m28s",
  "extension_count": 1,
  "total_extended": "15m",
  "percent_used": 34.0
}
```

---

## `anvil ps` (enhanced)

### Enhanced Output Format

```
TASK              STATUS    PID     ELAPSED    TIMEOUT
my-task           running   12345   15m32s     14m28s left (1x extended)
other-task        running   12346   5m12s      24m48s left
no-timeout-task   running   12347   3m00s      —
```

---

## Daemon API: `/extend-timeout`

### Request

```json
{
  "task_key": "path/to/project/task-name",
  "duration": "5m",
  "absolute": false
}
```

### Response (success)

```json
{
  "ok": true,
  "new_deadline": "2026-02-27T10:35:00Z",
  "remaining": "5m",
  "extension_count": 2
}
```

### Response (error)

```json
{
  "ok": false,
  "error": "task not found"
}
```
