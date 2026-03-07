# Feature Specification: Task Execution Pause/Resume with Throttling

**Feature Branch**: `342-task-throttle-controls`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add task execution pause/resume with throttling - global pause/resume, throttle rate, label-based pause"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Global Pause and Resume (Priority: P1)

A user is performing maintenance on their system and needs to temporarily halt all task execution without modifying any task files. They run `anvil pause` to immediately stop new tasks from being dispatched. Currently running tasks complete normally. When maintenance is done, they run `anvil resume` to restore normal execution.

**Why this priority**: Global pause is the most fundamental control - it provides an emergency stop for all task execution with a single command. Every other throttling feature builds on this foundation.

**Independent Test**: Can be fully tested by running `anvil pause`, verifying no new tasks dispatch, then running `anvil resume` and confirming tasks resume on schedule.

**Acceptance Scenarios**:

1. **Given** the daemon is running with tasks scheduled, **When** the user runs `anvil pause`, **Then** no new tasks are dispatched on subsequent ticks while already-running tasks complete normally.
2. **Given** the daemon is globally paused, **When** the user runs `anvil resume`, **Then** task dispatching resumes on the next tick cycle.
3. **Given** the daemon is globally paused, **When** the user runs `anvil status`, **Then** the output clearly indicates the system is paused.
4. **Given** the daemon is not paused, **When** the user runs `anvil pause` followed by another `anvil pause`, **Then** the second pause is accepted gracefully (idempotent).
5. **Given** the daemon is globally paused, **When** the daemon restarts, **Then** the pause state persists (not lost on restart).

---

### User Story 2 - Throttle Execution Rate (Priority: P2)

A user wants to slow down task execution to reduce system load during peak hours without stopping tasks entirely. They run `anvil throttle --rate 1/m` to limit execution to at most one task per minute. When the peak period ends, they run `anvil throttle --off` to restore full-speed execution.

**Why this priority**: Rate throttling provides more nuanced control than a binary pause/resume, allowing users to keep tasks running at a reduced pace.

**Independent Test**: Can be tested by setting a throttle rate, observing that tasks are dispatched no faster than the configured rate, and then disabling throttling to confirm full-speed dispatch resumes.

**Acceptance Scenarios**:

1. **Given** the daemon is running normally, **When** the user runs `anvil throttle --rate 1/m`, **Then** at most 1 task is dispatched per minute.
2. **Given** a throttle rate is active, **When** the user runs `anvil throttle --off`, **Then** normal execution speed resumes.
3. **Given** a throttle rate of 1/m is active, **When** the user runs `anvil throttle --rate 5/m`, **Then** the rate is updated to 5 tasks per minute.
4. **Given** a throttle rate is active, **When** the user runs `anvil status`, **Then** the current throttle rate is displayed.
5. **Given** an invalid rate format is provided, **When** the user runs `anvil throttle --rate invalid`, **Then** an error message explains the expected format.

---

### User Story 3 - Pause by Label (Priority: P2)

A user has tasks tagged with labels (e.g., "batch", "monitoring") and wants to pause only batch-processing tasks while keeping monitoring tasks running. They run `anvil pause --label batch` to pause tasks with that label. Later they run `anvil resume --label batch` to resume them.

**Why this priority**: Label-based pause provides targeted control without modifying individual task files, essential for managing task categories in larger setups.

**Independent Test**: Can be tested by pausing tasks with a specific label, verifying only those tasks stop dispatching while other tasks continue, then resuming to confirm they restart.

**Acceptance Scenarios**:

1. **Given** tasks exist with labels "batch" and "monitoring", **When** the user runs `anvil pause --label batch`, **Then** only tasks labeled "batch" stop dispatching while "monitoring" tasks continue normally.
2. **Given** tasks with label "batch" are paused, **When** the user runs `anvil resume --label batch`, **Then** those tasks resume dispatching on the next tick.
3. **Given** no tasks have the label "foo", **When** the user runs `anvil pause --label foo`, **Then** the command succeeds with a warning that no tasks matched.
4. **Given** tasks with label "batch" are paused via label, **When** the user runs `anvil status`, **Then** the output shows which labels are currently paused.
5. **Given** a task has labels "batch" and "important", and "batch" is paused, **When** the tick evaluates, **Then** the task is skipped because it has a paused label.

---

### User Story 4 - Status Visibility (Priority: P3)

A user wants to understand the current throttle state of their system. Running `anvil status` shows whether the system is globally paused, the current throttle rate (if any), and which labels are paused.

**Why this priority**: Visibility into throttle state is important for operational awareness but depends on the other features being implemented first.

**Independent Test**: Can be tested by setting various throttle states and verifying `anvil status` output reflects each state accurately.

**Acceptance Scenarios**:

1. **Given** no throttle controls are active, **When** the user runs `anvil status`, **Then** no throttle information is shown (or shows "normal").
2. **Given** the system is globally paused with labels "batch" paused and throttle rate 1/m, **When** the user runs `anvil status`, **Then** all three states are clearly displayed.

---

### Edge Cases

- What happens when global pause and label pause are both active, then global resume is issued? Label-specific pauses should remain in effect.
- What happens when a task has multiple labels and one is paused but not the other? The task should be paused (any paused label blocks execution).
- What happens when throttle rate is set to 0? This should be treated as invalid input with an error message.
- What happens when the daemon is paused and a user runs `anvil task run <name> --force`? Force-run should bypass the global pause.
- How does throttle interact with existing per-task rate limits? Both apply independently - the more restrictive limit wins.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a global pause command (`anvil pause`) that prevents all new task dispatching while allowing running tasks to complete.
- **FR-002**: System MUST support a global resume command (`anvil resume`) that restores normal task dispatching.
- **FR-003**: System MUST support a throttle rate command (`anvil throttle --rate N/m`) that limits task dispatching to N tasks per minute.
- **FR-004**: System MUST support disabling throttle (`anvil throttle --off`) to restore unlimited dispatch rate.
- **FR-005**: System MUST support label-based pause (`anvil pause --label <label>`) that pauses only tasks matching the specified label.
- **FR-006**: System MUST support label-based resume (`anvil resume --label <label>`) that resumes only tasks matching the specified label.
- **FR-007**: System MUST display current throttle state (global pause, throttle rate, paused labels) in `anvil status` output.
- **FR-008**: Global pause state MUST persist across daemon restarts (stored in a state file, not just in memory).
- **FR-009**: Throttle rate and paused labels MUST persist across daemon restarts.
- **FR-010**: Force-run (`anvil task run <name> --force`) MUST bypass global pause and label pause.
- **FR-011**: Global resume MUST NOT affect label-specific pauses (they are independent).
- **FR-012**: All pause/resume/throttle commands MUST be idempotent (running the same command twice produces no error).
- **FR-013**: Rate format MUST support N/m (per minute) notation with validation and clear error messages for invalid formats.
- **FR-014**: When a task has any paused label, the task MUST be skipped during dispatch regardless of its other labels.

### Key Entities

- **Throttle State**: Represents the current throttle configuration including global pause flag, throttle rate, and set of paused labels. Persisted to disk for daemon restart survival.
- **Throttle Rate**: A rate limit expressed as N tasks per time unit (e.g., "1/m" = 1 per minute). Controls maximum dispatch frequency across all tasks.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can pause all task execution with a single command and resume with a single command, each completing in under 1 second.
- **SC-002**: When a throttle rate is set, observed task dispatch frequency does not exceed the configured rate over any 1-minute window.
- **SC-003**: Label-based pause correctly filters tasks - only tasks with the paused label are blocked, all other tasks continue unaffected.
- **SC-004**: Throttle state survives daemon restart with 100% fidelity - pause state, throttle rate, and paused labels are all preserved.
- **SC-005**: `anvil status` accurately reflects current throttle state within one tick interval of any change.

## Assumptions

- Rate format uses `N/m` (tasks per minute) only. Other time units (per hour, per second) are not needed for the initial implementation.
- Throttle state is stored in a file under `~/.anvil/` (e.g., `~/.anvil/throttle.json`) rather than modifying the config file, since throttle state is operational (transient user intent) rather than permanent configuration.
- The existing `disabled` field on tasks (per-task pause) remains independent of global/label pause. They are separate mechanisms.
- Currently running tasks are never killed by pause commands - only new dispatching is affected.
