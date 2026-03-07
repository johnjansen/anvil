# Quickstart: Task Circuit Breaker

## Add Circuit Breaker to a Task

Add `circuit_breaker` to your task's frontmatter:

```yaml
---
schedule: "*/30 * * *"
circuit_breaker:
  failures: 5      # open after 5 consecutive failures
  timeout: 30m    # try again after 30 minutes
  half_open_max: 2 # allow 2 test requests in half-open state
---
```

## View Circuit Status

```bash
# See circuit state for a specific task
anvil task status my-task

# Output includes:
# Circuit Breaker: OPEN (will retry at 10:30)
# Failures: 5 consecutive
# Last Failure: 2026-02-27 10:00 (API timeout)
```

## Add Notification Hooks

```yaml
---
on_circuit_open: "echo 'Circuit opened for {{ .TaskName }}'"
on_circuit_close: "echo 'Circuit closed, service recovered'"
---
```

## How It Works

1. **Normal**: Task runs normally, circuit is CLOSED
2. **Failure**: After N consecutive failures, circuit OPENS
3. **Skip**: While OPEN, task is skipped without running
4. **Timeout**: After timeout, circuit enters HALF_OPEN (test mode)
5. **Recovery**: On success in HALF_OPEN, circuit CLOSES; on failure, reopens
