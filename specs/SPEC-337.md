# SPEC-337.md - Task Concurrency Limits Per Time Period

## Project Overview
- **Project**: anvil
- **Feature**: Time-based rate limiting for task execution
- **Issue**: #337
- **Goal**: Allow users to limit how many times a task runs within a time period (hour/day)

## Problem Statement

Currently, `max_concurrent` limits how many instances of a task can run at once, but there's no way to limit how many times a task runs within a time period. Users need:
- Run at most N times per hour/day
- Rate limit individual tasks separately from global rate limiting

## Proposed Solution

### 1. Config (Task Frontmatter)

Add `rate_limit` field to task frontmatter:

```yaml
---
schedule: "*/5 * * * *"
rate_limit:
  max_per_hour: 10      # run at most 10 times per hour
  max_per_day: 100      # run at most 100 times per day
---
```

Both fields are optional - you can set just `max_per_hour`, just `max_per_day`, or both.

### 2. Behavior

- Track runs per task within sliding time window
- Before queuing a task, check if limit would be exceeded
- If limit would be exceeded, skip the run with a reason logged
- Run counter resets when the time window expires (sliding window)

### 3. Rate Limit Storage

Rate limit counters stored in-memory on the daemon:

```go
type RateLimitCounter struct {
    TaskID      string
    Project     string
    HourCount   int
    HourWindow  time.Time    // start of current hour window
    DayCount    int
    DayWindow   time.Time    // start of current day window
}
```

On daemon restart, counters reset to zero.

### 4. CLI Visibility

```bash
# Show rate limit status for all tasks
$ anvil task rate-limits
TASK           THIS_HOUR   LIMIT    THIS_DAY   LIMIT
api-poll       8/10       80%      45/100     45%

# Show for specific task
$ anvil task rate-limits my-task
TASK           THIS_HOUR   LIMIT    THIS_DAY   LIMIT
my-task        8/10       80%      45/100     45%

# Reset counters for a task
$ anvil task rate-limits my-task --reset
Rate limit counters reset for my-task
```

### 5. Queue Visibility

When a task is skipped due to rate limiting:

```bash
$ anvil queue
TASK           STATUS      REASON
api-poll       skipped     rate limit: hourly (9/10)
```

## Technical Design

### Data Model

**New fields in Todo struct** (`internal/project/project.go`):
```go
type Todo struct {
    // ... existing fields ...
    RateLimit *RateLimitConfig `yaml:"rate_limit,omitempty"`
}

type RateLimitConfig struct {
    MaxPerHour *int `yaml:"max_per_hour,omitempty"`
    MaxPerDay  *int `yaml:"max_per_day,omitempty"`
}
```

**New struct for rate limit tracking** (`internal/daemon/daemon.go`):
```go
type RateLimitCounter struct {
    TaskID    string    `json:"task_id"`
    TaskName  string    `json:"task"`
    Project   string    `json:"project"`
    HourCount int       `json:"hour_count"`
    HourWindow time.Time `json:"hour_window"`
    DayCount  int       `json:"day_count"`
    DayWindow time.Time `json:"day_window"`
}
```

**New fields in Daemon struct**:
```go
type Daemon struct {
    // ... existing fields ...
    rateLimitCounters   map[string]*RateLimitCounter  // key: "project:taskName"
    rateLimitMu        sync.RWMutex
}
```

### Rate Limit Checker

Before adding a task to the queue:
1. Get the rate limit counter for this task
2. If `max_per_hour` is set and `HourCount >= max_per_hour`, skip the run
3. If `max_per_day` is set and `DayCount >= max_per_day`, skip the run
4. Otherwise, allow the run and increment the appropriate counter(s)

After a task completes successfully:
1. Increment the appropriate counter(s) if within the current window
2. If window has expired, reset the counter(s) and start fresh

### Socket Command for CLI

Add a new socket command handler for `task-rate-limits`:
- Returns JSON array of RateLimitCounter with percentage calculated
- Supports `--reset` flag to reset counters for a specific task
- CLI formats the response as a table

## Acceptance Criteria

- [ ] Tasks support `rate_limit.max_per_hour` in frontmatter
- [ ] Tasks support `rate_limit.max_per_day` in frontmatter
- [ ] Tasks skip when hourly limit would be exceeded
- [ ] Tasks skip when daily limit would be exceeded
- [ ] `anvil task rate-limits` shows status table
- [ ] Counters reset when time window expires (sliding window)
- [ ] Per-task limits work independently
- [ ] `anvil task rate-limits <name> --reset` resets counters

## Files to Modify

1. `internal/project/project.go` - Add RateLimitConfig to Todo struct, parse from frontmatter
2. `internal/daemon/daemon.go` - Add RateLimitCounter, rate limit checking logic, queue skip handling
3. `cmd/anvil/main.go` - Add `task rate-limits` subcommand with table output

## Edge Cases

- **Task removed**: Counter stays but task is gone - no issue, counter will be cleaned up on daemon restart
- **Daemon restart**: Counters reset to zero
- **Neither max_per_hour nor max_per_day set**: No rate limiting applied
- **Both limits set**: Both are checked, run skipped if either would be exceeded
- **Task runs but fails**: Still counts toward rate limit (completed runs, not successful runs)
- **Multiple projects**: Counters keyed by "project:taskName" to avoid collisions
- **Clock changes/DST**: Sliding window uses monotonic time where possible, falls back to wall clock
