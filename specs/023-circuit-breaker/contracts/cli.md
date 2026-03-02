# CLI Contract: Task Circuit Breaker

## Commands

### `anvil task status <name>`

Shows task status including circuit breaker state.

**Output Format:**
```
Task: my-task
Schedule: */30 * * *
Last Run: 2026-02-27 10:00 (success)
Circuit Breaker: OPEN (will retry at 10:30)
Failures: 5 consecutive
Last Failure: 2026-02-27 10:00 (API timeout)
```

**Circuit State Values:**
- `CLOSED` - Normal operation
- `OPEN` - Circuit is open, showing next retry time
- `HALF_OPEN` - Testing recovery

## Configuration

### Task Frontmatter

```yaml
circuit_breaker:
  failures: 5       # open after N failures (default: 5)
  timeout: 30m     # retry after duration (default: 30m)
  half_open_max: 2 # test requests in half-open (default: 2)
```

### Hook Variables

Available in `on_circuit_open` and `on_circuit_close` hooks:

| Variable | Description |
|----------|-------------|
| `{{ .TaskName }}` | Name of the task |
| `{{ .FailureCount }}` | Number of consecutive failures |
| `{{ .LastError }}` | Error message from last failure |
