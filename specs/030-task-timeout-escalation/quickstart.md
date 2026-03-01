# Quickstart Guide: Task Timeout Escalation

## Overview

This feature adds advanced timeout handling to anvil tasks, including:
- Advance warnings before timeout
- Adaptive timeouts that extend based on task progress
- Custom escalation hooks for timeout events

## Configuration

### Basic Timeout Warning

Add a timeout warning to get notified 5 minutes before a task times out:

```yaml
---
timeout: 30m
timeout_warning: 5m
---
Your task content here
```

### Custom Warning Hook

Execute a custom command when the timeout warning is triggered:

```yaml
---
timeout: 30m
timeout_warning: 5m
on_timeout_warning: "echo 'Task will timeout soon!' | mail -s 'Timeout Warning' admin@example.com"
---
Your task content here
```

### Adaptive Timeout

Enable adaptive timeout that extends when checkpoint files are detected:

```yaml
---
timeout: 30m
timeout_warning: 5m
adaptive_timeout:
  enabled: true
  extend_if: "checkpoint_exists"
  max_extensions: 2
---
Your task content here
```

### Custom Timeout Hooks

Execute commands on both warning and actual timeout:

```yaml
---
timeout: 30m
timeout_warning: 5m
on_timeout_warning: "echo 'Warning: Task approaching timeout'"
on_timeout: "echo 'Task timed out' | mail -s 'Task Timeout' admin@example.com"
---
Your task content here
```

## CLI Usage

### View Timeout Status

Use `anvil ps` to see timeout information:

```bash
anvil ps
```

This will show:
- Time remaining until timeout warning
- Time remaining until actual timeout
- Number of timeout extensions used

### Manual Timeout Extension

Extend a running task's timeout manually:

```bash
anvil task extend-timeout my-task 10m
```

## Example Task

Here's a complete example of a long-running backup task with timeout escalation:

```yaml
---
schedule: "0 2 * * *"
timeout: 4h
timeout_warning: 30m
on_timeout_warning: |
  echo "Backup task approaching timeout" |
  mail -s "Backup Timeout Warning" admin@example.com
on_timeout: |
  echo "Backup task timed out - check logs" |
  mail -s "Backup Failed" admin@example.com
adaptive_timeout:
  enabled: true
  extend_if: "checkpoint_exists"
  max_extensions: 3
---
#!/bin/bash
# Long-running backup script
echo "Starting backup..."
# ... backup logic ...
echo "##anvil:checkpoint"  # Signal progress
# ... more backup logic ...
```

## Testing

To test timeout warning functionality:

1. Create a test task with short timeout and warning:
   ```yaml
   ---
   timeout: 2m
   timeout_warning: 30s
   on_timeout_warning: "echo 'Warning triggered!' >> /tmp/warning.log"
   ---
   sleep 120
   ```

2. Run the task and check that the warning hook executes after 90 seconds

3. Verify the warning appears in `anvil ps` output