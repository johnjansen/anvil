# Tasks: Task Concurrency Limits Per Time Period (#337)

## Task Breakdown

### Phase 1: Data Model

- [ ] **1.1** Add `RateLimitConfig` struct to `internal/project/project.go`
  - Fields: `MaxPerHour *int` (yaml: "max_per_hour,omitempty"), `MaxPerDay *int` (yaml: "max_per_day,omitempty")
  - Pointer types so we can distinguish "not set" from "set to 0"
- [ ] **1.2** Add `RateLimit *RateLimitConfig` field to Todo struct
  - yaml tag: `yaml:"rate_limit,omitempty"`

### Phase 2: Daemon Rate Limit Tracking

- [ ] **2.1** Define `RateLimitCounter` struct in `internal/daemon/daemon.go`
  - Fields: TaskID, TaskName, Project (strings), HourCount, HourWindow, DayCount, DayWindow (time.Time)
  - JSON tags for API response
- [ ] **2.2** Add `rateLimitCounters map[string]*RateLimitCounter` and `rateLimitMu sync.RWMutex` to Daemon struct
- [ ] **2.3** Initialize `rateLimitCounters` map in `New()` constructor
- [ ] **2.4** Implement `checkRateLimit(project, taskName string, config *RateLimitConfig) (allowed bool, reason string)` method
  - Get or create counter for task
  - If current time >= HourWindow+1h: reset HourCount, set HourWindow to current hour
  - If current time >= DayWindow+1d: reset DayCount, set DayWindow to current day
  - If config.MaxPerHour != nil && HourCount >= *config.MaxPerHour: return false, "rate limit: hourly (X/Y)"
  - If config.MaxPerDay != nil && DayCount >= *config.MaxPerDay: return false, "rate limit: daily (X/Y)"
  - Return true, ""
- [ ] **2.5** Implement `incrementRateLimit(project, taskName string, config *RateLimitConfig)` method
  - Get or create counter for task
  - Reset windows if expired (same logic as check)
  - Increment HourCount if config.MaxPerHour != nil
  - Increment DayCount if config.MaxPerDay != nil
- [ ] **2.6** Implement `resetRateLimit(project, taskName string)` method
  - Delete counter from map if exists
  - Or reset counts to 0

### Phase 3: Integrate with Queue

- [ ] **3.1** Find where tasks are scheduled/queued in daemon.go
  - Likely in the main scheduling loop or `scheduleTask()` method
- [ ] **3.2** Before adding task to queue, call `checkRateLimit()` with task's RateLimit config
- [ ] **3.3** If not allowed, record skip with reason (don't add to queue)
- [ ] **3.4** If allowed, call `incrementRateLimit()`, then add to queue
- [ ] **3.5** Ensure queue display shows skipped tasks with status "skipped" and reason

### Phase 4: CLI Command

- [ ] **4.1** Register `/task-rate-limits` handler on socket mux in `startSocketServer()`
  - Accept query params: `task` (filter), `reset` (reset counters)
  - Return JSON array of counters with percentage calculated
- [ ] **4.2** For `reset` param: call `resetRateLimit()` and return success message
- [ ] **4.3** Add `task rate-limits` subcommand in `cmd/anvil/main.go`
  - Call daemon socket `/task-rate-limits`
  - Format as table: TASK, THIS_HOUR, LIMIT, THIS_DAY, LIMIT
  - THIS_HOUR: "X/Y (XX%)" format
  - THIS_DAY: "X/Y (XX%)" format
  - Show "-" when limit not configured
- [ ] **4.4** Add `--reset` flag to reset counters for a specific task

### Phase 5: Testing

- [ ] **5.1** Unit tests for sliding window logic (reset at hour/day boundary)
- [ ] **5.2** Unit tests for skip logic (should skip when at limit, allow when under)
- [ ] **5.3** Manual testing:
  - Create task with `rate_limit.max_per_hour: 3`
  - Trigger task multiple times
  - Verify first 3 run, subsequent runs are skipped
  - Verify `anvil task rate-limits` shows correct counts
  - Verify `--reset` clears counters

## Implementation Order

1. Phase 1 (Data Model) - no dependencies
2. Phase 2 (Rate Limit Tracking) - depends on Phase 1
3. Phase 3 (Queue Integration) - depends on Phase 2
4. Phase 4 (CLI) - depends on Phase 2 (can do in parallel with Phase 3)
5. Phase 5 (Testing) - after implementation

## Notes

- Rate limit counters are in-memory only; reset on daemon restart (by design)
- Sliding window uses wall clock time
- Count is incremented when task STARTS (queued), not when it completes
- Both max_per_hour and max_per_day can be set; task skipped if either would exceed
- Disabled tasks should still count toward rate limit (they could become enabled later)
