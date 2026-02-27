# Tasks: Task Circuit Breaker for Failure Isolation

**Input**: Design documents from `specs/017-task-circuit-breaker/`
**Prerequisites**: SPEC-336.md, PLAN-336.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

---

## Phase 1: Core Data Model

**Purpose**: Add circuit breaker configuration to task definition

- [ ] T001 [P] [US1] Add CircuitBreaker struct to `internal/project/project.go` with Enabled, FailureThreshold, SuccessThreshold, and Timeout fields
- [ ] T002 [P] [US1] Add CircuitBreaker field to Todo struct with yaml tag
- [ ] T003 [US1] Test CircuitBreaker parsing from YAML frontmatter

---

## Phase 2: Daemon Circuit State Management

**Purpose**: Track circuit state in the daemon and integrate with task execution

- [ ] T004 [P] [US1] Add CircuitBreakerState struct to `internal/daemon/daemon.go`
- [ ] T005 [P] [US1] Add circuitBreakers map and circuitBreakerMu mutex to Daemon struct
- [ ] T006 [US1] Implement circuit state check in RunTask - skip if OPEN
- [ ] T007 [US1] Implement state transitions: failure increments and opens circuit
- [ ] T008 [US1] Implement success handling: reset failures, close from half-open
- [ ] T009 [US1] Add timeout handler goroutine to transition OPEN → HALF_OPEN

---

## Phase 3: CLI Commands

**Purpose**: Add visibility and manual control for circuit breakers

- [ ] T010 [P] [US1] Add socket command handler for `task-circuit` in daemon
- [ ] T011 [US1] Add `task circuit` subcommand to `cmd/anvil/main.go`
- [ ] T012 [US1] Implement table output for circuit states
- [ ] T013 [US1] Add --open, --close, --reset flags to CLI

---

## Phase 4: Integration & Testing

**Purpose**: Verify end-to-end functionality

- [ ] T014 [US1] Integration test: task with circuit_breaker enabled skips when open
- [ ] T015 [US1] Test state transitions: CLOSED → OPEN → HALF_OPEN → CLOSED
- [ ] T016 [US1] Test manual --open/--close/--reset commands

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Data Model)**: No dependencies - can start immediately
- **Phase 2 (Daemon)**: Depends on Phase 1 - uses CircuitBreaker from Todo
- **Phase 3 (CLI)**: Depends on Phase 2 - needs socket command handler
- **Phase 4 (Testing)**: Depends on Phases 1-3 complete

### Within Each Phase

- Phase 1: T001, T002 can run in parallel; T003 depends on both
- Phase 2: T004, T005 can run in parallel; T006-T009 depend on them
- Phase 3: T010 can run parallel with T011; T012, T013 depend on T010/T011

---

## Notes

- [P] tasks = different files, no dependencies
- This is a focused feature with single user story (US1: circuit breaker pattern)
- Verify tests fail before implementing
- Commit after each task or logical group
