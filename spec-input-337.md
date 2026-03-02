## Problem

Currently, max_concurrent limits how many instances of a task can run at once, but there's no way to limit how many times a task runs within a time period. Users need:
- Run at most N times per hour/day
- Rate limit individual tasks separately from global rate limiting

## Proposed Solution

Add time-based concurrency limits:

### 1. Config

```yaml
---
schedule: "*/5 * * * *"
rate_limit:
  max_per_hour: 10      # run at most 10 times per hour
  max_per_day: 100      # run at most 100 times per day
```

### 2. Behavior

- Track runs per task within time window
- Skip runs that would exceed limit
- Show skip reason in queue

### 3. Visibility

```bash
$ anvil task rate-limits
TASK           THIS_HOUR   LIMIT    THIS_DAY   LIMIT
api-poll       8/10       80%      45/100     45%
```

### 4. Commands

```bash
# Show rate limit status
anvil task rate-limits

# Reset counters
anvil task rate-limits my-task --reset
```

## Acceptance Criteria

- [ ] Tasks support rate_limit.max_per_hour and max_per_day
- [ ] Tasks skip when limit would be exceeded
- [ ] `anvil task rate-limits` shows status
- [ ] Counters reset based on configured period
- [ ] Per-task limits work independently
