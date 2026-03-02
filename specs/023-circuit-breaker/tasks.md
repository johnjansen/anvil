# Tasks: Task Circuit Breaker

**Feature**: 023-circuit-breaker
**Generated**: 2026-03-01

## Implementation Strategy

**MVP Scope**: User Story 1 (Automatic Failure Isolation) - Core circuit breaker that opens after failures
**Delivery**: Incremental - each user story is independently testable

---

## Phase 1: Setup

- [x] T001 Create circuit breaker storage directory in .anvil/circuits/ for persisting circuit state
- [x] T002 Review similar implementations (internal/daemon/sla.go, internal/daemon/alerts.go) for patterns

---

## Phase 2: Foundational

- [x] T003 Add CircuitBreakerConfig struct to internal/project/project.go for task frontmatter parsing
- [x] T004 Add OnCircuitOpen and OnCircuitClose fields to Todo struct in internal/project/project.go
- [x] T005 Create internal/daemon/circuit.go with CircuitState enum and CircuitBreakerRecord struct
- [x] T006 Implement circuit state persistence (load/save to .anvil/circuits/<task-id>.json)
- [x] T007 Add circuit breaker directory to config.EnsureDir()

---

## Phase 3: User Story 1 - Automatic Failure Isolation (P1)

**Goal**: Tasks automatically stop running after configured consecutive failures

**Independent Test**: Configure task with circuit_breaker.failures: 1, run failing task, verify subsequent runs skip with "circuit open"

- [x] T008 [US1] Implement getEffectiveCircuitBreaker() to merge per-task and global config in internal/daemon/circuit.go
- [x] T009 [US1] Implement checkCircuit() to evaluate if task should run based on circuit state in internal/daemon/circuit.go
- [x] T010 [US1] Implement recordFailure() to increment failure count and open circuit in internal/daemon/circuit.go
- [x] T011 [US1] Implement recordSuccess() to reset failure count on success in internal/daemon/circuit.go
- [x] T012 [US1] Integrate circuit check into daemon tick loop (internal/daemon/daemon.go) - skip task if circuit is OPEN
- [x] T013 [US1] Add circuit state check to task dispatch before executing runner

---

## Phase 4: User Story 2 - Automatic Recovery (P1)

**Goal**: Circuits automatically recover after timeout period

**Independent Test**: Open circuit, wait for timeout, trigger task, verify it runs and circuit closes on success

- [x] T014 [US2] Implement transition from OPEN to HALF_OPEN after timeout in internal/daemon/circuit.go
- [x] T015 [US2] Implement half-open state tracking (allow limited test requests)
- [x] T016 [US2] Implement circuit close on success in HALF_OPEN in internal/daemon/circuit.go
- [x] T017 [US2] Implement circuit reopen on failure in HALF_OPEN in internal/daemon/circuit.go
- [x] T018 [US2] Add timeout check to daemon tick to trigger OPEN→HALF_OPEN transition

---

## Phase 5: User Story 3 - Circuit State Visibility (P2)

**Goal**: Users can see circuit state in CLI

**Independent Test**: Open circuits on multiple tasks, run anvil task status, verify state appears

- [x] T019 [US3] Add circuit status to task status output in cmd/anvil/ (extend existing task status command)
- [x] T020 [US3] Display failure count, last failure time, next retry time in status output
- [x] T021 [US3] Add circuit state column to anvil task list output

---

## Phase 6: User Story 4 - Circuit Breaker Hooks (P3)

**Goal**: Notify users when circuit opens/closes

**Independent Test**: Configure on_circuit_open hook, trigger circuit open, verify hook executes

- [x] T022 [US4] Implement runCircuitOpenHook() in internal/daemon/circuit.go
- [x] T023 [US4] Implement runCircuitCloseHook() in internal/daemon/circuit.go
- [x] T024 [US4] Execute on_circuit_open hook when circuit transitions to OPEN
- [x] T025 [US4] Execute on_circuit_close hook when circuit transitions to CLOSED

---

## Phase 7: Polish & Cross-Cutting

- [x] T026 Add circuit breaker to global config defaults in internal/config/config.go
- [x] T027 Write unit tests for circuit state machine in internal/daemon/circuit_test.go
- [x] T028 Run go build ./... to verify compilation
- [x] T029 Run go test ./... to verify tests pass

---

## Dependencies

```
T001 ──┬─► T003 ──► T008 ──► T012 ──► T014 ──► T019 ──► T022 ──► T026
       │                    │                    │            │
T002 ──┘                    │                    │            │
                           │                    │            │
                    T009 ◄──┴──────► T013 ◄─────┴─► T015 ◄───┴─► T020
                           │                    │
                    T010 ◄──┘              T016 ◄──┘
                           │                    │
                    T011 ◄──┘              T017 ◄──┘
                                              │
T003 ──► T004 ──► T005 ──► T006 ──► T007 ◄───┘
                                              │
T008 ──► T009 ──► T010 ──► T011 ◄────────────┘

T019 ──► T020 ──► T021
T022 ──► T023 ──► T024 ──► T025

T026 ──► T027 ──► T028 ──► T029
```

---

## Parallel Opportunities

- **T003, T004, T005**: Config structs and core types can be created in parallel
- **T019, T022**: CLI visibility and hooks are independent
- **T027**: Unit tests can run after implementation completes

---

## Summary

| Metric | Value |
|--------|-------|
| Total Tasks | 29 |
| User Story 1 (P1) | 6 tasks |
| User Story 2 (P1) | 5 tasks |
| User Story 3 (P2) | 3 tasks |
| User Story 4 (P3) | 4 tasks |
| Setup/Foundational | 7 tasks |
| Polish | 4 tasks |
| Parallelizable | ~6 tasks |

**MVP Scope**: Tasks T001-T013 - Core circuit breaker with automatic failure isolation
