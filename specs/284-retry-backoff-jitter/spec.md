# Feature Specification: Advanced Task Retry with Backoff Strategies and Jitter

**Feature Branch**: `284-retry-backoff-jitter`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add task retry with exponential backoff and jitter"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Backoff Strategy (Priority: P1)

A user with a task that retries on failure wants to choose how retry delays increase over time. They configure their task with a retry strategy (exponential, linear, or constant) so that retries are spaced appropriately for their use case. For example, an API-calling task uses exponential backoff to avoid overwhelming a recovering service, while a simple file-check task uses constant delay.

**Why this priority**: Backoff strategy is the core enhancement. Without it, users are stuck with the existing hardcoded exponential behavior and cannot choose the delay pattern that fits their workload.

**Independent Test**: Can be fully tested by creating tasks with each strategy and verifying the delay between retry attempts matches the expected pattern.

**Acceptance Scenarios**:

1. **Given** a task configured with `strategy: exponential`, `retry: 3`, and `delay: 1m`, **When** the task fails repeatedly, **Then** retries occur after approximately 1m, 2m, 4m delays.
2. **Given** a task configured with `strategy: linear`, `retry: 3`, and `delay: 1m`, **When** the task fails repeatedly, **Then** retries occur after approximately 1m, 2m, 3m delays.
3. **Given** a task configured with `strategy: constant`, `retry: 3`, and `delay: 1m`, **When** the task fails repeatedly, **Then** retries occur after approximately 1m, 1m, 1m delays.
4. **Given** a task with no strategy specified but `retry` and `retry_delay` set (legacy syntax), **When** the task fails, **Then** behavior is identical to the existing exponential backoff (backward compatible).

---

### User Story 2 - Add Jitter to Prevent Thundering Herd (Priority: P2)

A user running many identical tasks (e.g., across watched projects) wants to add randomization to retry delays so that failed tasks don't all retry at the same instant, overwhelming a shared resource.

**Why this priority**: Jitter is a critical reliability enhancement that prevents synchronized retries. It builds on the backoff strategy from P1.

**Independent Test**: Can be tested by configuring jitter on a task and verifying that actual retry delays vary within the expected range across multiple runs.

**Acceptance Scenarios**:

1. **Given** a task with `delay: 1m` and `jitter: 0.5`, **When** the task fails, **Then** the actual delay is randomized within the range 30s to 90s (1m +/- 50%).
2. **Given** a task with `jitter: 0` or no jitter configured, **When** the task fails, **Then** the delay is deterministic (no randomization applied).

---

### User Story 3 - Limit Total Retry Duration (Priority: P2)

A user wants to set a maximum wall-clock time for retries so that a task doesn't keep retrying indefinitely. After the time limit is reached, the task should fail regardless of remaining retry attempts.

**Why this priority**: Prevents runaway retry loops that consume resources long after the issue window has passed.

**Independent Test**: Can be tested by setting a short `max_total_time` with many retries and verifying the task stops retrying once the time limit is exceeded.

**Acceptance Scenarios**:

1. **Given** a task with `retry: 10`, `delay: 5m`, and `max_total_time: 15m`, **When** the task fails repeatedly, **Then** retries stop after 15 minutes even if retry attempts remain.
2. **Given** a task with `max_total_time` not configured, **When** the task fails repeatedly, **Then** all configured retry attempts are used regardless of elapsed time (existing behavior).

---

### User Story 4 - Show Retry Strategy in Task History (Priority: P3)

A user wants to see retry details (strategy, delays used, jitter applied) in the task history output so they can diagnose retry behavior and tune configuration.

**Why this priority**: Observability enhancement. Useful for debugging but not required for the retry mechanism to function.

**Independent Test**: Can be tested by running a task that retries and checking that `anvil task history` displays the strategy, delays, and attempt information.

**Acceptance Scenarios**:

1. **Given** a task that retried with exponential backoff and jitter, **When** the user runs `anvil task history <task>`, **Then** the output shows the retry strategy, number of attempts, and delays used.
2. **Given** a task that succeeded on the first attempt, **When** the user views its history, **Then** no retry information is shown.

---

### Edge Cases

- What happens when `max_total_time` is shorter than the first retry delay? The task should fail immediately without retrying (no retry attempt can fit within the time budget).
- What happens when jitter value is greater than 1.0 (e.g., 1.5)? The system should clamp jitter to the range 0.0-1.0 and log a warning.
- What happens when both legacy syntax (`retry: N`, `retry_delay: Xm`) and new syntax (`retry.max`, `retry.delay`) are used? Legacy syntax should be treated as shorthand for the new format with `strategy: exponential` (preserving current behavior).
- What happens when the retry delay with backoff exceeds a reasonable cap? Apply a maximum delay cap (e.g., 1 hour) to prevent unreasonably long waits.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support three backoff strategies: `exponential` (delay doubles each attempt), `linear` (delay increases by base amount each attempt), and `constant` (same delay every attempt).
- **FR-002**: System MUST support a `jitter` configuration (0.0 to 1.0) that randomizes each retry delay by +/- the specified percentage.
- **FR-003**: System MUST support a `max_total_time` configuration that limits the total wall-clock time spent retrying; once exceeded, the task fails with remaining retries unused.
- **FR-004**: System MUST remain backward compatible with existing `retry: N` and `retry_delay: Xm` syntax, treating them as equivalent to the new structured format with `strategy: exponential`.
- **FR-005**: System MUST default to `strategy: exponential` when no strategy is specified (matching current behavior).
- **FR-006**: System MUST record retry strategy details (strategy name, delays used, jitter applied) in the run record for observability.
- **FR-007**: System MUST display retry strategy information in task history output when retries occurred.
- **FR-008**: System MUST cap computed retry delays at a maximum of 1 hour to prevent unreasonably long waits.
- **FR-009**: System MUST clamp jitter values to the valid range (0.0-1.0) and warn the user if an out-of-range value is provided.

### Key Entities

- **RetryConfig**: Represents the retry configuration for a task, including max attempts, base delay, strategy, jitter percentage, and max total time.
- **RunRecord**: Extended with retry strategy metadata (strategy name, actual delays used per attempt, jitter applied).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can configure any of the three backoff strategies and observe correct delay patterns in task execution logs.
- **SC-002**: Tasks with jitter configured show measurably different retry delays across repeated failures (delays vary within the configured jitter range).
- **SC-003**: Tasks with `max_total_time` configured stop retrying within the specified time limit, even when retry attempts remain.
- **SC-004**: Existing tasks using `retry` and `retry_delay` continue to work identically without configuration changes.
- **SC-005**: Task history output clearly shows retry strategy, attempt count, and delay progression for tasks that retried.

## Assumptions

- The `on_error` filtering (retry only on specific error types) described in the issue is deferred. Error-based filtering requires a mechanism for tasks to report structured error types, which is a separate concern. This spec focuses on delay strategy, jitter, time limits, and observability.
- The existing retry loop structure in the daemon can be extended in-place without architectural changes.
- Jitter uses uniform random distribution (not full jitter or decorrelated jitter variants).
- The maximum delay cap of 1 hour is a sensible default; it is not user-configurable in this iteration.
