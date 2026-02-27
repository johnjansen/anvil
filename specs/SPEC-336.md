# SPEC-336.md - Task Circuit Breaker for Failure Isolation

## Project Overview
- **Project**: anvil
- **Feature**: Circuit breaker pattern for task failure isolation
- **Issue**: #336
- **Goal**: Automatically pause tasks that depend on failing downstream services, preventing wasted LLM calls and rate limit exhaustion

## Problem Statement

When a downstream service (API, database) is failing, tasks that depend on it should stop trying rather than accumulating failures. Currently:
- Tasks keep retrying even when the service is down
- Each failure counts against rate limits
- No way to automatically pause failing tasks

## Proposed Solution

### 1. Circuit Breaker Config (Task Frontmatter)

Add a `circuit_breaker` field to task frontmatter:

```yaml
---
schedule: "*/30 * * *"
circuit_breaker:
  enabled: true
  failure_threshold: 3      # open after 3 consecutive failures
  success_threshold: 2       # close after 2 successes
  timeout: 5m              # try again after 5 minutes
---
```

- When `enabled: false` or not specified, circuit breaker is bypassed
- `failure_threshold`: Number of consecutive failures to open the circuit (default: 3)
- `success_threshold`: Number of consecutive successes to close the circuit from half-open (default: 2)
- `timeout`: Duration before attempting to close the circuit from open state (default: 5m)

### 2. Circuit States

- **Closed**: Normal operation, requests go through normally
- **Open**: Circuit is open, tasks skip immediately without running (saves LLM calls and rate limits)
- **Half-open**: After timeout expires, allows limited requests through to test if service recovered

### 3. State Transitions

```
CLOSED --(failure >= failure_threshold)--> OPEN
OPEN --(timeout elapsed)--> HALF_OPEN
HALF_OPEN --(success >= success_threshold)--> CLOSED
HALF_OPEN --(failure)--> OPEN
CLOSED --(success)--> CLOSED (reset failure counter)
```

### 4. CLI Visibility

```bash
# Show circuit state for all tasks with circuit_breaker enabled
$ anvil task circuit
TASK           STATE    FAILURES   LAST_FAILURE
api-task      OPEN     5          2026-02-27 10:30
data-task     CLOSED   0          -
report-task   HALF_OPEN 1        2026-02-27 10:35

# Manually control circuit state
$ anvil task circuit my-task --open
$ anvil task circuit my-task --close

# Reset failure count
$ anvil task circuit my-task --reset

# Show detailed info for specific task
$ anvil task circuit my-task --verbose
```

### 5. Circuit State Storage

Circuit state is stored in-memory on the daemon (not persisted to disk):

```go
type CircuitBreakerState struct {
    TaskID    string
    TaskName  string
    Project   string
    State     CircuitState  // closed, open, half-open
    Failures  int
    Successes int
    LastFailure time.Time
    LastStateChange time.Time
}

type CircuitState string
const (
    CircuitClosed    CircuitState = "closed"
    CircuitOpen      CircuitState = "open"
    CircuitHalfOpen  CircuitState = "half-open"
)
```

On daemon restart, all circuits start in CLOSED state.

### 6. Integration with Task Execution

When a task is about to run:
1. Check if circuit_breaker is enabled for the task
2. If circuit is OPEN, skip execution immediately, log as "circuit open"
3. If circuit is CLOSED or HALF_OPEN, proceed with execution
4. After execution completes:
   - On failure: increment failures, reset successes, check if should transition to OPEN
   - On success: increment successes, reset failures, check if should transition to CLOSED (from HALF_OPEN)

## Technical Design

### Data Model

**New fields in Todo struct** (`internal/project/project.go`):
```go
type CircuitBreaker struct {
    Enabled           bool          `yaml:"enabled,omitempty"`
    FailureThreshold  int           `yaml:"failure_threshold,omitempty"`
    SuccessThreshold  int           `yaml:"success_threshold,omitempty"`
    Timeout           time.Duration `yaml:"timeout,omitempty"`
}

type Todo struct {
    // ... existing fields ...
    CircuitBreaker    CircuitBreaker `yaml:"circuit_breaker,omitempty"`
}
```

**New struct for circuit state** (`internal/daemon/daemon.go`):
```go
type CircuitBreakerState struct {
    TaskID    string        `json:"task_id"`
    TaskName  string        `json:"task"`
    Project   string        `json:"project"`
    State     string        `json:"state"`        // closed, open, half-open
    Failures  int           `json:"failures"`
    Successes int           `json:"successes"`
    LastFailure time.Time   `json:"last_failure"`
    LastStateChange time.Time `json:"last_state_change"`
}
```

**New fields in Daemon struct**:
```go
type Daemon struct {
    // ... existing fields ...
    circuitBreakers   map[string]*CircuitBreakerState  // key: "project:taskName"
    circuitBreakerMu  sync.RWMutex
}
```

### Task Execution Integration

Modify `RunTask` in daemon to check circuit state before executing:
1. Lock circuit breaker for task
2. If state is OPEN, return early (skip execution)
3. Execute task normally
4. Update circuit state based on result (success/failure)
5. Handle state transitions (closed -> open -> half-open -> closed)
6. Unlock

### Timeout Handler

A goroutine that:
1. Runs every second
2. Checks all OPEN circuits
3. If `time.Since(lastStateChange) >= timeout`, transitions to HALF_OPEN
4. Uses the daemon's stop channel to terminate

### Socket Command for CLI

Add a new socket command handler for `task-circuit`:
- Returns JSON array of CircuitBreakerState
- Supports `--open`, `--close`, `--reset` flags for manual control
- CLI formats the response as a table

## Acceptance Criteria

- [ ] Tasks support `circuit_breaker` with failure_threshold, success_threshold, timeout in frontmatter
- [ ] Tasks skip immediately when circuit is OPEN (no LLM call made)
- [ ] Circuit transitions to HALF_OPEN after timeout duration
- [ ] After HALF_OPEN, circuit closes after success_threshold consecutive successes
- [ ] `anvil task circuit` shows state for all tasks with circuit_breaker enabled
- [ ] Manual `--open`, `--close`, `--reset` commands work
- [ ] On daemon restart, circuits reset to CLOSED state

## Files to Modify

1. `internal/project/project.go` - Add CircuitBreaker struct and field to Todo
2. `internal/daemon/daemon.go` - Add CircuitBreakerState, circuit breaker logic, timeout handler
3. `cmd/anvil/main.go` - Add `task circuit` subcommand with table output

## Edge Cases

- **Task removed while circuit state exists**: Remove circuit state when task is removed
- **circuit_breaker.enabled not set**: Treat as disabled, no circuit state tracked
- **Daemon restart**: All circuits start in CLOSED state
- **Multiple projects**: Circuit state keyed by "project:taskName" to avoid collisions
- **Timeout of 0**: Treat as immediate transition to half-open (not recommended but valid)
- **Task disabled**: Skip circuit state updates for disabled tasks
