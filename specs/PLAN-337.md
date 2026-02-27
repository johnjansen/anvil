# Implementation Plan: Task Concurrency Limits Per Time Period (#337)

## Overview

Add time-based rate limiting to anvil. Tasks define `rate_limit.max_per_hour` and/or `rate_limit.max_per_day` in frontmatter. The daemon tracks run counts within sliding time windows, skips runs that would exceed limits, and exposes status via CLI (`anvil task rate-limits`).

## Implementation Steps

### Phase 1: Data Model (no dependencies)

**1.1 Add rate limit fields to Todo struct**
- File: `internal/project/project.go`
- Add `RateLimit *RateLimitConfig` to Todo struct
- Define `RateLimitConfig` struct with `MaxPerHour *int` and `MaxPerDay *int`
- Parse from frontmatter via yaml tags

### Phase 2: Daemon Rate Limit Tracking (depends on Phase 1)

**2.1 Add RateLimitCounter type and tracking map**
- File: `internal/daemon/daemon.go`
- Define `RateLimitCounter` struct (TaskID, TaskName, Project, HourCount, HourWindow, DayCount, DayWindow)
- Add `rateLimitCounters map[string]*RateLimitCounter` and `rateLimitMu sync.RWMutex` to Daemon struct
- Initialize map in `New()`

**2.2 Add rate limit checking method**
- File: `internal/daemon/daemon.go`
- New method `checkRateLimit(project, taskName string, config *RateLimitConfig) (allowed bool, reason string)`
- Uses sliding window logic:
  - If current time is past HourWindow+1h, reset hour counter and window
  - If current time is past DayWindow+1d, reset day counter and window
  - Check if HourCount >= MaxPerHour (if set)
  - Check if DayCount >= MaxPerDay (if set)
  - Return false with reason if either limit would be exceeded

**2.3 Add increment method**
- File: `internal/daemon/daemon.go`
- New method `incrementRateLimit(project, taskName string, config *RateLimitConfig)`
- Called after task starts (before adding to queue)
- Resets window if expired, then increments counter

**2.4 Add counter reset method**
- File: `internal/daemon/daemon.go`
- New method `resetRateLimit(project, taskName string)`
- Called when user requests reset via CLI

### Phase 3: Integrate with Queue (depends on Phase 2)

**3.1 Modify queue logic**
- File: `internal/daemon/daemon.go`
- In the scheduling loop where tasks are added to queue:
  - Before adding task, call `checkRateLimit()` with the task's RateLimit config
  - If not allowed, mark task as "skipped" with reason instead of adding to queue
  - If allowed, call `incrementRateLimit()` then add to queue

**3.2 Add skip handling in queue display**
- File: `internal/daemon/daemon.go`
- Ensure skipped tasks (rate limited) show in queue with status "skipped" and reason

### Phase 4: CLI Command (depends on Phase 2)

**4.1 Add socket command handler for task-rate-limits**
- File: `internal/daemon/daemon.go`
- Register `/task-rate-limits` handler on socket mux
- Returns JSON array of RateLimitCounter with percentage calculated
- Supports query param: `task` (filter by name), `reset` (reset counters)

**4.2 Add `task rate-limits` subcommand**
- File: `cmd/anvil/main.go`
- New subcommand under `task` that calls daemon socket `/task-rate-limits`
- Formats response as table: TASK, THIS_HOUR, LIMIT, THIS_DAY, LIMIT
- Calculate percentage and show as "count/limit (XX%)"
- `--reset` flag resets counters for named task

## Dependencies

- Phase 2 depends on Phase 1 (needs Todo.RateLimit field)
- Phase 3 depends on Phase 2 (needs rate limit checking logic)
- Phase 4 depends on Phase 2 (needs socket handler and counter data)

## Testing Strategy

- Unit tests for sliding window logic (reset on hour/day boundary)
- Unit tests for skip logic (should skip when at limit)
- Manual testing:
  - Create task with `rate_limit.max_per_hour: 3`
  - Trigger task 5 times, verify first 3 run, 4th and 5th are skipped
  - Verify `anvil task rate-limits` shows correct counts
  - Verify `--reset` clears counters

## Files to Modify

1. `internal/project/project.go` - RateLimitConfig struct and Todo field
2. `internal/daemon/daemon.go` - RateLimitCounter, checking/increment/reset methods, queue integration, socket handler
3. `cmd/anvil/main.go` - `task rate-limits` CLI subcommand
