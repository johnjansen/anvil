# Feature Specification: Task Execution Time Windows

**Feature Branch**: `003-task-time-windows`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task execution time windows to restrict when tasks can run"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Per-Task Allowed Window (Priority: P1)

A user managing a data-intensive task wants to restrict it to only run during business hours on weekdays. They add an `allowed_window` to their task's frontmatter specifying start time, end time, and allowed days. When the cron schedule fires outside this window, the task is silently skipped and waits for the next cron match that falls within the window.

**Why this priority**: This is the core value proposition. Without per-task time windows, the entire feature has no purpose. It directly addresses the primary use case of restricting task execution to specific time periods.

**Independent Test**: Can be fully tested by creating a task with an `allowed_window` and verifying it only runs when the current time falls within the specified window. Delivers immediate value for scheduling control.

**Acceptance Scenarios**:

1. **Given** a task with `allowed_window: {start: "09:00", end: "18:00", days: "1-5"}` and `schedule: "*/15 * * * *"`, **When** the cron fires at 08:45 on a Monday, **Then** the task is skipped (not executed).
2. **Given** the same task, **When** the cron fires at 09:00 on a Monday, **Then** the task executes normally.
3. **Given** a task with `allowed_window: {start: "09:00", end: "18:00", days: "1-5"}`, **When** the cron fires at 10:00 on a Saturday, **Then** the task is skipped.
4. **Given** a task with no `allowed_window` defined, **When** the cron fires, **Then** the task executes normally (backward compatible).

---

### User Story 2 - Global Quiet Hours (Priority: P2)

An operations team wants to prevent non-critical tasks from running during nighttime hours across all projects. They configure global `quiet_hours` in the anvil config file. During quiet hours, only high-priority (p0) tasks are allowed to run. Other tasks wait for the next cron match after quiet hours end.

**Why this priority**: Provides system-wide protection without requiring per-task configuration. Builds on the window evaluation logic from P1 but applies it globally, offering a convenient default for all tasks.

**Independent Test**: Can be tested by configuring quiet hours in config and verifying that non-p0 tasks are skipped during the quiet window while p0 tasks still execute.

**Acceptance Scenarios**:

1. **Given** quiet hours configured as `start: "22:00", end: "07:00"` with `exclude_priority: 0`, **When** a p2 task's cron fires at 23:00, **Then** the task is skipped.
2. **Given** the same quiet hours, **When** a p0 task's cron fires at 23:00, **Then** the task executes normally.
3. **Given** quiet hours `enabled: false`, **When** any task's cron fires at 23:00, **Then** the task executes normally.
4. **Given** a task with both a per-task `allowed_window` and global quiet hours active, **When** the cron fires, **Then** the task must satisfy both constraints (both the task window and not be in quiet hours unless exempt by priority).

---

### User Story 3 - Force Run Bypassing Windows (Priority: P2)

A user needs to run a task immediately for debugging or urgent reasons, even though it's currently outside the task's allowed window or during quiet hours. They use the `--force` flag when manually triggering the task, which bypasses all time window checks.

**Why this priority**: Essential for operational flexibility. Without this, time windows become a rigid constraint that blocks urgent work. Same priority as quiet hours because both are needed for a practical deployment.

**Independent Test**: Can be tested by manually running a task with `--force` during a restricted time window and verifying it executes.

**Acceptance Scenarios**:

1. **Given** a task with an active `allowed_window` and the current time is outside that window, **When** the user runs `anvil task run <name> --force`, **Then** the task executes immediately.
2. **Given** quiet hours are active, **When** the user runs `anvil task run <name> --force`, **Then** the task executes immediately regardless of priority.

---

### User Story 4 - View Next Allowed Run Time (Priority: P3)

A user wants to understand when their time-windowed task will next be eligible to run. They run `anvil task next <name> --verbose` and see the next cron match that also falls within the allowed window, including information about any active restrictions.

**Why this priority**: Useful for visibility and debugging but not required for core functionality. The system works without it; this just helps users understand the schedule.

**Independent Test**: Can be tested by running the command for a time-windowed task and verifying the output shows the correct next valid execution time.

**Acceptance Scenarios**:

1. **Given** a task with `allowed_window` and a cron schedule, **When** the user runs `anvil task next <name> --verbose`, **Then** the output shows the next cron match that satisfies the window constraint.
2. **Given** a task with no window constraints, **When** the user runs `anvil task next <name>`, **Then** the output shows the next cron match (same as current behavior).

---

### Edge Cases

- What happens when `allowed_window` start and end times cross midnight (e.g., start: "22:00", end: "06:00")? The window should be interpreted as spanning overnight.
- What happens when `days` is empty or omitted in `allowed_window`? All days should be allowed (same as "0-6").
- What happens when quiet hours span midnight (e.g., start: "22:00", end: "07:00")? This is the common case and must be handled correctly.
- How does the system handle timezone changes (e.g., DST transitions)? Use local system time consistently; tasks near DST boundaries may shift by one hour.
- What happens when a task has a per-task window that conflicts with quiet hours (e.g., task window allows 23:00-01:00 but quiet hours block 22:00-07:00)? Both constraints apply; the task is effectively blocked during the overlap.
- What if `exclude_priority` in quiet hours is set to a value like 1, meaning both p0 and p1 tasks can run? The system should allow any task with priority less than or equal to the `exclude_priority` value.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support an `allowed_window` configuration per task with `start` (HH:MM), `end` (HH:MM), and `days` (range like "1-5" using 0=Sunday convention) fields in task frontmatter.
- **FR-002**: System MUST evaluate time window constraints at dispatch time, after the cron schedule matches but before task execution begins.
- **FR-003**: System MUST skip task execution when the current time falls outside the task's `allowed_window`, without raising an error.
- **FR-004**: System MUST support a global `quiet_hours` configuration in the anvil config file with `enabled`, `start`, `end`, and `exclude_priority` fields.
- **FR-005**: System MUST exempt tasks from quiet hours when their priority level is less than or equal to the `exclude_priority` value (e.g., `exclude_priority: 0` means only p0 tasks bypass quiet hours).
- **FR-006**: System MUST support a `--force` flag on manual task execution that bypasses all time window and quiet hour constraints.
- **FR-007**: System MUST support an `anvil task next <name>` command that shows the next valid execution time considering both cron schedule and time window constraints.
- **FR-008**: System MUST handle time windows that span midnight correctly (e.g., start: "22:00", end: "06:00").
- **FR-009**: System MUST treat tasks with no `allowed_window` as having no time restrictions (fully backward compatible).
- **FR-010**: System MUST apply both per-task window and global quiet hours when both are configured; a task must satisfy both constraints to execute.
- **FR-011**: System MUST use local system time for all time window evaluations.

### Key Entities

- **Allowed Window**: A per-task time constraint with start time, end time, and allowed days of the week. Controls when a specific task is eligible to run.
- **Quiet Hours**: A global configuration that defines a system-wide restricted period during which only high-priority tasks may execute. Has an enabled flag, time range, and priority exemption threshold.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can configure per-task execution windows and observe that tasks only run during the specified time periods.
- **SC-002**: Users can configure global quiet hours and observe that non-exempt tasks are blocked during the restricted period.
- **SC-003**: Users can force-run any task at any time, bypassing all window restrictions, within 1 command.
- **SC-004**: Users can view the next valid execution time for any task, accounting for both schedule and window constraints.
- **SC-005**: Existing tasks without time window configuration continue to operate identically to current behavior (zero breaking changes).

## Assumptions

- Time zones: All time evaluations use the local system timezone. No per-task timezone configuration is needed for the initial implementation.
- Day numbering follows the standard cron convention: 0 = Sunday, 1 = Monday, ..., 6 = Saturday.
- The `days` field supports range notation (e.g., "1-5") and comma-separated values (e.g., "1,3,5").
- Priority values follow the existing anvil convention where p0 is highest priority.
- Skipped tasks due to time windows are silent (no error, no special log entry beyond normal scheduling behavior). The skip is visible in the daemon's scheduling decisions.
- The `--verbose` flag on `anvil task next` shows additional detail about which constraints are active. Without `--verbose`, just the next valid time is shown.
