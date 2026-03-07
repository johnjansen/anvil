## Problem

When a downstream service (API, database) is failing, tasks that depend on it should stop trying rather than accumulating failures. Currently:
- Tasks keep retrying even when the service is down
- Each failure counts against rate limits
- No way to automatically pause failing tasks

## Proposed Solution

Add circuit breaker pattern:

### 1. Circuit breaker config

```yaml
---
schedule: "*/30 * * *"
circuit_breaker:
  enabled: true
  failure_threshold: 3      # open after 3 consecutive failures
  success_threshold: 2       # close after 2 successes
  timeout: 5m              # try again after 5 minutes
```

### 2. States

- **Closed**: Normal operation, requests go through
- **Open**: Circuit is open, tasks skip immediately without running
- **Half-open**: Testing if service recovered

### 3. Visibility

```bash
$ anvil task circuit
TASK           STATE    FAILURES   LAST_FAILURE
api-task      OPEN     5          2026-02-27 10:30
data-task     CLOSED   0          -
```

### 4. Commands

```bash
# Manually open/close circuit
anvil task circuit my-task --open
anvil task circuit my-task --close

# Reset failure count
anvil task circuit my-task --reset
```

## Acceptance Criteria

- [ ] Tasks support circuit_breaker with failure_threshold, success_threshold, timeout
- [ ] Tasks skip immediately when circuit is OPEN
- [ ] Circuit transitions to HALF_OPEN after timeout
- [ ] `anvil task circuit` shows state for all tasks
- [ ] Manual open/close/reset commands work
