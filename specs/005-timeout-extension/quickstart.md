# Quickstart: Task Execution Timeout Extension

## Manual Timeout Extension

Extend a running task's timeout from the command line:

```bash
# Add 30 minutes to the remaining time
anvil task extend-timeout my-task 30m

# Set deadline to exactly 1 hour from now
anvil task extend-timeout my-task 1h --absolute
```

## Check Timeout Status

See how much time a running task has left:

```bash
# Show timeout for a specific task
anvil task timeout my-task

# Show all running tasks with timeout info
anvil task timeout --all

# Include in ps output
anvil ps
```

Output:
```
TASK              ELAPSED    TIMEOUT    REMAINING   EXTENSIONS   PROGRESS
my-task           15m32s     30m        14m28s      1x +15m      ████░░░░░░ 52%
```

## Auto-Extend Configuration

Configure a task to automatically extend its timeout when making progress:

```yaml
---
schedule: "0 9 * * *"
timeout: 30m
auto_extend:
  enabled: true
  max_extensions: 3
  extension_duration: 15m
---
Run my ETL pipeline
```

The task will auto-extend up to 3 times (adding 15 minutes each time) when it emits a checkpoint within 5 minutes of the deadline. Total possible runtime: 30m + 3×15m = 75 minutes.

## Timeout Warning Hook

Get notified when a task is approaching its deadline:

```yaml
---
schedule: "0 9 * * *"
timeout: 30m
on_timeout_warning: "curl -X POST https://hooks.slack.com/... -d '{\"text\": \"$ANVIL_TASK_NAME has $ANVIL_TIMEOUT_REMAINING remaining\"}'"
---
```

The hook fires 5 minutes before the deadline with environment variables:
- `ANVIL_TASK_NAME` — task name
- `ANVIL_TIMEOUT_REMAINING` — time left
- `ANVIL_TIMEOUT_ORIGINAL` — original timeout
- `ANVIL_EXTENSIONS_USED` — number of extensions applied
- `ANVIL_AUTO_EXTEND_REMAINING` — remaining auto-extensions (if configured)
