## Problem

Currently, there's no way to define health checks for tasks. Users can't easily determine if a task is "healthy" or "unhealthy" based on its recent behavior.

## Proposed Solution

Add health check configuration:

```yaml
---
schedule: "0 9 * * *"
health_check:
  failure_threshold: 3    # consider unhealthy after 3 consecutive failures
  success_threshold: 2    # consider healthy after 2 consecutive successes
  timeout: 5m            # consider unhealthy if task runs longer than this
---
```

Health status visible in CLI:

```bash
$ anvil task health
TASK           HEALTH    CONSECUTIVE FAILURES
fetch-data    healthy   0
process-data  unhealthy 5
```

## Acceptance Criteria

- [ ] Tasks support health_check configuration
- [ ] Tasks marked unhealthy after consecutive failures
- [ ] Tasks recover to healthy after successes
- [ ] `anvil task health` shows health status
