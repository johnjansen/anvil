# Implementation Plan: Task Circuit Breaker for Failure Isolation

**Branch**: `[017-task-circuit-breaker]` | **Date**: 2026-02-28 | **Spec**: [SPEC-336.md](./SPEC-336.md)

## Summary

Add circuit breaker pattern to tasks that automatically pauses task execution when downstream services are failing. Prevents wasted LLM calls and rate limit exhaustion by tracking consecutive failures and transitioning through CLOSED → OPEN → HALF_OPEN → CLOSED states. Includes CLI visibility and manual control commands.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: No new dependencies (existing sync utilities)
**Storage**: In-memory state on daemon (not persisted to disk)
**Testing**: Go testing (`go test`)
**Target Platform**: CLI tool (macOS/Linux)
**Project Type**: CLI daemon/task runner
**Performance Goals**: Minimal overhead, O(1) circuit state lookup
**Constraints**: None identified
**Scale/Scope**: Per-task configuration, single-user CLI

## Constitution Check

*No constitutional issues identified for this feature.*

## Project Structure

### Documentation (this feature)

```text
specs/017-task-circuit-breaker/
├── SPEC.md          # Feature specification
└── plan.md          # This file
```

### Source Code (repository root)

```text
internal/
├── project/
│   └── project.go     # Add CircuitBreaker struct to Todo
└── daemon/
    └── daemon.go      # Add CircuitBreakerState, circuit logic, timeout handler
cmd/anvil/
└── main.go            # Add "task circuit" subcommand
```

**Structure Decision**: Minimal changes to existing architecture. Circuit breaker logic integrated into daemon since it directly controls task execution flow.

## Implementation Approach

### 1. Add CircuitBreaker to Todo Struct

In `internal/project/project.go`, add `CircuitBreaker` struct with Enabled, FailureThreshold, SuccessThreshold, and Timeout fields. Add to `Todo` struct with YAML tag.

### 2. Add CircuitBreakerState to Daemon

In `internal/daemon/daemon.go`:
- Add `CircuitBreakerState` struct with TaskID, TaskName, Project, State, Failures, Successes, LastFailure, LastStateChange
- Add `circuitBreakers` map and `circuitBreakerMu` mutex to Daemon struct

### 3. Integrate with Task Execution

Modify `RunTask` function to:
1. Check if circuit_breaker is enabled for task
2. If circuit is OPEN, skip execution and return early
3. After execution completes, update circuit state based on success/failure
4. Handle state transitions

### 4. Add Timeout Handler

Add goroutine that:
1. Runs every second
2. Checks all OPEN circuits
3. Transitions to HALF_OPEN when timeout expires
4. Stops when daemon stops

### 5. Add CLI Command

In `cmd/anvil/main.go`, add `task circuit` subcommand:
- `anvil task circuit` - show all circuit states
- `anvil task circuit <task>` - show specific task
- `anvil task circuit <task> --open` - manually open
- `anvil task circuit <task> --close` - manually close
- `anvil task circuit <task> --reset` - reset failure count

## Files to Modify

1. `internal/project/project.go` - Add CircuitBreaker struct and field to Todo
2. `internal/daemon/daemon.go` - Add CircuitBreakerState, circuit logic, timeout handler
3. `cmd/anvil/main.go` - Add `task circuit` subcommand

## Testing Approach

1. Unit tests for CircuitBreaker struct parsing
2. Unit tests for state transition logic (closed → open → half-open → closed)
3. Integration test: task with circuit_breaker skips when open
4. Manual testing for CLI commands

## Acceptance Criteria Validation

- [ ] `circuit_breaker` config parses correctly from frontmatter
- [ ] Task skips when circuit is OPEN (no LLM call)
- [ ] Circuit transitions to HALF_OPEN after timeout
- [ ] Circuit closes after success_threshold successes (from HALF_OPEN)
- [ ] `anvil task circuit` shows table of all circuit states
- [ ] Manual --open/--close/--reset work
- [ ] Daemon restart resets all circuits to CLOSED
