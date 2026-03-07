## Problem

Currently, the health endpoint just checks if the daemon is running. Users need custom health checks that verify:
- Task-specific conditions are healthy
- External dependencies are reachable
- Custom business logic is satisfied

## Proposed Solution

Add custom health checks:

### 1. Health check script

```yaml
---
schedule: "*/30 * * *"
health_check: "curl -sf https://api.example.com/health"
---
```

### 2. Health status visibility

```bash
$ anvil task health
TASK           HEALTH    LAST_CHECK
api-task      healthy   10s ago
data-task     unhealthy 30s ago
```

### 3. Health endpoint includes task health

When health_port is configured, the /health endpoint can include task health:

```yaml
health:
  include_tasks: true
  unhealthy_threshold: 2   # mark unhealthy if 2+ tasks unhealthy
```

### 4. Commands

```bash
# Force health check
anvil task health my-task --check

# Show detailed health
anvil task health my-task --verbose
```

## Acceptance Criteria

- [ ] Tasks support health_check script
- [ ] Script exit code determines health (0=healthy, non-0=unhealthy)
- [ ] `anvil task health` shows health status
- [ ] Health check runs on configurable interval
- [ ] /health endpoint can include task health status
