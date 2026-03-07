# Task Frontmatter Contract: Circuit Breaker

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `circuit_breaker.failures` | int | No | 5 | Number of consecutive failures before opening |
| `circuit_breaker.timeout` | duration | No | 30m | Time to wait before attempting recovery |
| `circuit_breaker.half_open_max` | int | No | 2 | Max test requests in half-open state |
| `on_circuit_open` | string | No | - | Shell command to run when circuit opens |
| `on_circuit_close` | string | No | - | Shell command to run when circuit closes |

## Example

```yaml
---
schedule: "*/5 * * *"
circuit_breaker:
  failures: 3
  timeout: 5m
  half_open_max: 1
on_circuit_open: "echo 'Circuit opened for {{ .TaskName }}'"
on_circuit_close: "echo 'Circuit closed, service recovered'"
---
```
