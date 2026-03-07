## Problem

When a task repeatedly fails (e.g., due to external API outage), it continues to consume resources and trigger alerts. Users need a way to automatically stop retrying after too many failures, then automatically recover when the external service comes back.

## Proposed Solution

Add circuit breaker:

### 1. Circuit breaker configuration

```yaml
---
schedule: "*/30 * * *"
circuit_breaker:
  failures: 5       # open after 5 consecutive failures
  timeout: 30m      # try again after 30 minutes
  half_open_max: 2  # allow 2 test requests in half-open state
---
```

### 2. States

- **Closed**: Normal operation, requests go through
- **Open**: Too many failures, requests fail immediately
- **Half-Open**: Testing if service recovered

### 3. Visibility

```bash
$ anvil task status my-task
Circuit Breaker: OPEN (will retry at 10:30)
Failures: 5 consecutive
Last Failure: 2026-02-27 10:00 (API timeout)
```

### 4. Hooks

```yaml
on_circuit_open: "echo 'Circuit opened for {{ .TaskName }}'"
on_circuit_close: "echo 'Circuit closed, service recovered'"
```

## Acceptance Criteria

- [ ] Tasks support `circuit_breaker.failures` and `circuit_breaker.timeout`
- [ ] Circuit opens after configured failures
- [ ] Circuit auto-recovers after timeout
- [ ] Status visible in `anvil task get`
- [ ] `on_circuit_open` and `on_circuit_close` hooks fire
