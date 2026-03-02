# Feature Specification: Task Circuit Breaker for Failure Isolation

**Feature Branch**: `023-circuit-breaker`
**Created**: 2026-03-01
**Status**: Draft
**Input**: User description: "#330: Add task circuit breaker for failure isolation"

## User Scenarios & Testing

### User Story 1 - Automatic Failure Isolation (Priority: P1)

A user configures a scheduled task that depends on an external API. When the API goes down, the task repeatedly fails. The user wants the system to automatically stop running the task after several consecutive failures to conserve resources and avoid alert fatigue.

**Why this priority**: This is the core value proposition of the circuit breaker pattern - preventing resource waste and alert storms during service outages.

**Independent Test**: Can be tested by configuring a task with circuit breaker enabled, simulating multiple failures, and verifying the circuit opens after the configured threshold.

**Acceptance Scenarios**:

1. **Given** a task with `circuit_breaker.failures: 3`, **When** the task fails 3 times consecutively, **Then** the circuit opens and subsequent task executions are skipped immediately without running the task logic.
2. **Given** a task with `circuit_breaker.failures: 3` that has just opened, **When** the task is triggered again, **Then** the task is skipped with reason "circuit open".
3. **Given** a task with `circuit_breaker.timeout: 30m` that is in OPEN state, **When** 30 minutes pass, **Then** the circuit transitions to HALF_OPEN state and allows the next execution attempt.

---

### User Story 2 - Automatic Recovery (Priority: P1)

A user has a task whose external dependency was down (circuit is open). Once the external service recovers, the user wants the task to automatically resume normal operation without manual intervention.

**Why this priority**: Without automatic recovery, users would need to manually monitor and reset circuits, negating much of the automation benefit.

**Independent Test**: Can be tested by opening a circuit, waiting for the timeout, triggering a task execution, and verifying it runs and the circuit closes on success.

**Acceptance Scenarios**:

1. **Given** a circuit in HALF_OPEN state, **When** a task execution succeeds, **Then** the circuit closes and returns to normal operation.
2. **Given** a circuit in HALF_OPEN state, **When** a task execution fails, **Then** the circuit reopens immediately.

---

### User Story 3 - Circuit State Visibility (Priority: P2)

A user wants to see the current state of circuit breakers across all their tasks to understand which services are experiencing issues.

**Why this priority**: Visibility is essential for operational awareness and debugging.

**Independent Test**: Can be tested by opening circuits on multiple tasks and verifying they all appear in the status command output.

**Acceptance Scenarios**:

1. **Given** tasks with circuit breakers configured, **When** user runs `anvil task status <task-name>`, **Then** the output includes circuit breaker state, failure count, last failure time, and next retry time (if open).
2. **Given** multiple tasks with circuits in various states, **When** user runs `anvil task list`, **Then** each task shows its circuit state in the output.

---

### User Story 4 - Circuit Breaker Hooks (Priority: P3)

A user wants to be notified via hooks when a circuit opens or closes, so they can take immediate action (e.g., page on-call, send Slack alert).

**Why this priority**: Enables automation and alerting around circuit state changes.

**Independent Test**: Can be tested by configuring hooks and triggering circuit state changes, verifying the hook commands execute with correct variables.

**Acceptance Scenarios**:

1. **Given** a task with `on_circuit_open` hook configured, **When** the circuit opens, **Then** the hook command executes with access to task name and failure details.
2. **Given** a task with `on_circuit_close` hook configured, **When** the circuit closes, **Then** the hook command executes with access to task name.

---

### Edge Cases

- What happens when circuit_breaker configuration is added to a task that already has a failure history?
- How does the circuit handle successes between failures (does it reset the failure count)?
- What happens when timeout is very short (e.g., 1m) and task keeps failing in half-open state?
- How does circuit state persist across daemon restarts?
- What happens when multiple task runs are queued and the circuit opens mid-queue?

## Requirements

### Functional Requirements

- **FR-001**: System MUST allow tasks to define `circuit_breaker.failures` (number of consecutive failures before opening)
- **FR-002**: System MUST allow tasks to define `circuit_breaker.timeout` (duration before attempting recovery)
- **FR-003**: System MUST allow tasks to define `circuit_breaker.half_open_max` (maximum test requests in half-open state)
- **FR-004**: Circuit MUST transition from CLOSED to OPEN after consecutive failures reach the configured threshold
- **FR-005**: Circuit MUST transition from OPEN to HALF_OPEN after the timeout duration elapses
- **FR-006**: When in OPEN state, task execution MUST be skipped immediately without running task logic
- **FR-007**: When in HALF_OPEN state, task execution MUST be allowed to test service recovery
- **FR-008**: On success in HALF_OPEN state, circuit MUST transition to CLOSED
- **FR-009**: On failure in HALF_OPEN state, circuit MUST transition back to OPEN
- **FR-010**: Consecutive failures MUST reset to zero on any successful task execution
- **FR-011**: System MUST display circuit breaker state in `anvil task status` output
- **FR-012**: System MUST support `on_circuit_open` hook that fires when circuit opens
- **FR-013**: System MUST support `on_circuit_close` hook that fires when circuit closes
- **FR-014**: Circuit state MUST persist across daemon restarts

### Key Entities

- **CircuitBreakerConfig**: Configuration for circuit breaker behavior (failures threshold, timeout, half_open_max)
- **CircuitState**: Enum representing circuit state (Closed, Open, HalfOpen)
- **CircuitBreakerStatus**: Runtime state including current state, failure count, last failure time, next retry time
- **CircuitBreakerRecord**: Persisted state for circuit breaker including state, failure count, last failure timestamp, opened_at timestamp

## Success Criteria

### Measurable Outcomes

- **SC-001**: Tasks with circuit breakers open automatically after configured consecutive failures, preventing resource waste during service outages
- **SC-002**: Circuits automatically recover after timeout period, resuming normal task execution without manual intervention
- **SC-003**: Users can view circuit breaker state for any task via CLI, enabling operational awareness
- **SC-004**: Circuit state changes trigger configured hooks, enabling automated alerting and response
