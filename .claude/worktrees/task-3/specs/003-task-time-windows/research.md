# Research: Task Execution Time Windows

## R1: Time Window Evaluation Point in Daemon Dispatch

**Decision**: Insert time window checks in the daemon `tick()` function after dependency checks (~line 2018 in daemon.go) and before the stopped-task check.

**Rationale**: This is the natural position in the dispatch filter chain. The cron schedule has already matched (confirming the task is "due"), and we filter by window before entering persistent-task-specific logic. This keeps the window logic independent and follows the existing pattern of sequential dispatch checks.

**Alternatives considered**:
- Modify cron matching itself: Rejected — would conflate scheduling with windowing, harder to maintain
- Check at enqueue time (after all other checks): Rejected — would inflate inFlight counters for skipped tasks
- Check in the worker goroutine: Rejected — too late, would waste a worker slot

## R2: AllowedWindow Struct Design

**Decision**: Add an `AllowedWindow` struct to the Todo with string fields for start, end, and days. Parse HH:MM times at dispatch time using `time.Parse("15:04", ...)`.

**Rationale**: Keeping raw strings in the struct matches the existing pattern (e.g., `Schedule string`, `Timeout time.Duration` parsed from string). Parsing at evaluation time is cheap and avoids adding complexity to the frontmatter parser.

**Alternatives considered**:
- Pre-parse into `time.Time` during frontmatter loading: Rejected — `time.Time` carries date info we don't need; HH:MM comparison is simpler with hour/minute integers
- Use a custom type with parsed fields: Rejected — over-engineering for 3 string fields

## R3: Midnight-Spanning Windows

**Decision**: When `end < start` (e.g., start="22:00", end="06:00"), interpret as spanning midnight. Check: `now >= start OR now < end`.

**Rationale**: Standard approach for time range checks that cross midnight. Users expect "22:00-06:00" to mean "from 10 PM to 6 AM the next day."

**Alternatives considered**:
- Require two separate windows for overnight: Rejected — poor UX, error-prone
- Use duration instead of end time: Rejected — less intuitive than start/end format

## R4: Day-of-Week Parsing

**Decision**: Support range notation ("1-5"), comma-separated ("1,3,5"), and combined ("1-5,0") using 0=Sunday convention matching Go's `time.Weekday()`.

**Rationale**: Matches cron day-of-week conventions that anvil users already understand. Go's `time.Weekday()` uses 0=Sunday natively.

**Alternatives considered**:
- Named days ("mon-fri"): Rejected — adds parsing complexity for marginal UX gain; can be added later
- 1=Monday convention: Rejected — inconsistent with cron and Go stdlib

## R5: Force-Run Bypass Mechanism

**Decision**: Add `Force bool` field to the existing `RunRequest` struct. In `handleRun()`, set a flag on the todo copy that skips window evaluation in the dispatch path.

**Rationale**: Follows the existing pattern where `handleRun` already modifies the todo copy (clears `PreCheck`). Adding a `ForceWindow` bool to the work item keeps the change minimal.

**Alternatives considered**:
- Skip window check entirely in handleRun path: More invasive — handleRun enqueues to the same workQueue, so the tick function would need to differentiate
- Add a separate "force dispatch" channel: Over-engineering for this use case

## R6: Quiet Hours Priority Exemption

**Decision**: `exclude_priority` is an integer threshold. Tasks with `Priority <= exclude_priority` bypass quiet hours. Default: 0 (only p0 exempt).

**Rationale**: Simple numeric comparison. Priority 0 is highest in anvil (stored in p0/ directory). Setting `exclude_priority: 1` means both p0 and p1 tasks bypass quiet hours.

**Alternatives considered**:
- List of exempt priorities: Rejected — over-complex for a threshold use case
- Boolean "exempt_p0_only": Rejected — less flexible than a numeric threshold

## R7: `anvil task next` Implementation

**Decision**: Add a `taskNextCmd` function that uses `cron.Parser.Next()` in a loop, testing each candidate time against the window constraints until finding one that passes. Cap iterations to prevent infinite loops (max 366 days forward).

**Rationale**: Simple brute-force approach. Cron fires at most once per minute, so iterating forward through candidates is fast. The 366-day cap handles degenerate cases (e.g., a window that can never be satisfied).

**Alternatives considered**:
- Analytical calculation combining cron and window: Rejected — complex to implement correctly, especially with midnight-spanning windows
- Only show next cron match without window info: Rejected — defeats the purpose of the command
