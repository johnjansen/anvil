## Problem

Currently there's no way to limit how many times a task runs within a time period:
- A task with `*/5 * * * *` runs 288 times per day with no limit
- Users want to cap execution frequency regardless of schedule
- Useful for rate-limiting expensive tasks

## Proposed Solution

Add concurrency limits per time period:

### 1. Rate limiting by execution count

```yaml
---
schedule: "*/5 * * * *"
max_runs_per_hour: 10    # only run 10 times per hour max
# or
max_runs_per_day: 50     # only run 50 times per day max
---
```

### 2. Sliding window enforcement

- Track executions within the time window
- Skip if limit already reached
- Clear visibility in queue why skipped

```bash
$ anvil task queue
TASK           STATUS    REASON
expensive     skipped   hourly limit (9/10 used)
```

### 3. Burst handling

```yaml
max_runs_per_hour: 10
burst: 2    # allow up to 2 extra runs in a single hour
```

## Acceptance Criteria

- [ ] Tasks support `max_runs_per_hour` and `max_runs_per_day`
- [ ] Skip tasks that exceed limit, show clear reason
- [ ] `burst` allows short bursts above limit
- [ ] Visible in queue status why task was skipped
- [ ] Reset counters based on time window
