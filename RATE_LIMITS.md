# Task Rate Limiting

This feature adds time-based rate limiting to tasks, allowing you to control how frequently tasks can run within specific time periods.

## Configuration

Add rate limiting to tasks using the `rate_limit` field in the task frontmatter:

```yaml
---
schedule: "*/5 * * * *"
rate_limit:
  max_per_hour: 10      # run at most 10 times per hour
  max_per_day: 100      # run at most 100 times per day
---
```

## Commands

### View Rate Limits Status

```bash
anvil task rate-limits
```

Shows the current rate limit status for all tasks with rate limits configured:

```
TASK           THIS_HOUR   LIMIT    THIS_DAY   LIMIT
api-poll       8/10       80%      45/100     45%
```

### Reset Counters

```bash
anvil task rate-limits --reset
```

Resets all rate limit counters to zero.

## Behavior

- Tasks with rate limits that would exceed their configured limits are skipped
- Skip reason is shown in the task queue
- Counters automatically reset at the start of each hour/day
- Per-task limits work independently