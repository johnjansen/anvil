# Feature Specification: Task SLA Tracking

**Feature Branch**: `004-task-sla-tracking`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task SLA tracking for missed schedule notifications"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Per-Task SLA Configuration and Violation Detection (Priority: P1)

A user managing time-sensitive tasks wants to be alerted when a task runs significantly later than its scheduled time. They add an `sla` block to their task's frontmatter specifying a `max_delay` duration. When the daemon dispatches the task, it compares the actual dispatch time against the scheduled time. If the delay exceeds `max_delay`, it records an SLA violation and runs the `on_sla_violation` hook command.

**Why this priority**: This is the core value proposition — detecting and reporting schedule delays. Without this, the entire feature has no purpose.

**Independent Test**: Can be tested by creating a task with `sla.max_delay: 1m` and verifying that when the task is dispatched more than 1 minute after its scheduled time, a violation is recorded and the hook fires.

**Acceptance Scenarios**:

1. **Given** a task with `sla: {max_delay: 15m}` scheduled for 09:00, **When** the task is dispatched at 09:10, **Then** no SLA violation is recorded (within threshold).
2. **Given** the same task, **When** the task is dispatched at 09:20, **Then** an SLA violation is recorded with delay of 20 minutes and the `on_sla_violation` hook fires.
3. **Given** a task with `sla: {max_delay: 15m, strict: true}`, **When** the task would be dispatched at 09:20, **Then** the task is skipped entirely instead of running late.
4. **Given** a task with no `sla` block, **When** the task is dispatched late, **Then** no SLA tracking occurs (backward compatible).
5. **Given** a task with `on_sla_violation` set, **When** an SLA violation occurs, **Then** the command is executed with task name and delay information available.

---

### User Story 2 - SLA Status in Task Info (Priority: P2)

A user wants to check the SLA health of a specific task. When they run `anvil task get <name>`, they see the configured SLA threshold and the status of the most recent run — including whether it violated SLA and by how much.

**Why this priority**: Provides visibility into SLA status through existing CLI commands. Builds on the violation detection from US1 but adds user-facing reporting.

**Independent Test**: Can be tested by running `anvil task get` for a task with SLA configured and verifying the output includes SLA information.

**Acceptance Scenarios**:

1. **Given** a task with `sla: {max_delay: 15m}` that ran on time, **When** the user runs `anvil task get <name>`, **Then** the output shows "SLA: 15m max delay" and "Last Run: on time".
2. **Given** a task with SLA that ran 32 minutes late, **When** the user runs `anvil task get <name>`, **Then** the output shows "Last Run: 32m late - SLA VIOLATION".
3. **Given** a task with no SLA configured, **When** the user runs `anvil task get <name>`, **Then** no SLA information is shown.

---

### User Story 3 - SLA Dashboard Command (Priority: P2)

A user wants a quick overview of all tasks with SLA violations across their project. They run `anvil task sla` to see a summary of which tasks have violated their SLA, with `--verbose` for details and `--reset` to clear violation history after intentional downtime.

**Why this priority**: Provides aggregate visibility. Important for operations but depends on violation data from US1.

**Independent Test**: Can be tested by running `anvil task sla` and verifying it lists tasks with SLA violations in the current project.

**Acceptance Scenarios**:

1. **Given** three tasks with SLA configured (one violated, two on time), **When** the user runs `anvil task sla`, **Then** only the violated task is shown with its violation details.
2. **Given** tasks with SLA violations, **When** the user runs `anvil task sla --verbose`, **Then** all SLA-configured tasks are shown with their status (pass/fail) and delay history.
3. **Given** SLA violations exist, **When** the user runs `anvil task sla --reset`, **Then** all violation records are cleared and subsequent `anvil task sla` shows no violations.
4. **Given** no tasks have SLA configured, **When** the user runs `anvil task sla`, **Then** a message indicates no tasks have SLA tracking enabled.

---

### User Story 4 - Global SLA Defaults (Priority: P3)

An operations team wants a default SLA threshold for all tasks without configuring each one individually. They set `sla.default_max_delay` in the global config. Tasks without explicit SLA configuration inherit this default.

**Why this priority**: Convenience feature that reduces per-task configuration overhead. Not required for core functionality.

**Independent Test**: Can be tested by setting a global default and verifying tasks without per-task SLA inherit the global threshold.

**Acceptance Scenarios**:

1. **Given** global config with `sla: {default_max_delay: 30m}` and a task with no per-task SLA, **When** the task runs 45 minutes late, **Then** an SLA violation is recorded using the 30m global threshold.
2. **Given** global config with `sla: {default_max_delay: 30m}` and a task with `sla: {max_delay: 15m}`, **When** the task runs 20 minutes late, **Then** an SLA violation is recorded using the per-task 15m threshold (overrides global).
3. **Given** no global SLA config and no per-task SLA, **When** the task runs late, **Then** no SLA tracking occurs.

---

### Edge Cases

- What happens when a task has no schedule (one-shot or persistent)? SLA tracking is skipped — SLA only applies to cron-scheduled tasks.
- What happens when the daemon was down and tasks pile up? The delay is calculated from the most recent scheduled time, not from all missed schedules.
- What happens when `strict: true` and the task is the first run after daemon restart? The delay is calculated; if it exceeds max_delay, the task is skipped.
- What happens if `on_sla_violation` command fails? The violation is still recorded; hook failure is logged but does not affect task execution.
- What happens when `max_delay` is 0? This is treated as "no SLA configured" (disabled).
- How are SLA violations persisted across daemon restarts? Violations are stored as JSON files alongside run records in the task's history directory.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support an `sla` configuration block per task with `max_delay` (duration string like "15m") and optional `strict` (boolean, default false) fields in task frontmatter.
- **FR-002**: System MUST calculate dispatch delay as the difference between actual dispatch time and the most recent scheduled time (using the cron expression's previous match).
- **FR-003**: System MUST record an SLA violation when a task's dispatch delay exceeds its configured `max_delay`.
- **FR-004**: System MUST execute the `on_sla_violation` hook command when an SLA violation is detected, with task name and delay information available to the command.
- **FR-005**: System MUST skip task execution (instead of running late) when `strict: true` and the dispatch delay exceeds `max_delay`.
- **FR-006**: System MUST display SLA configuration and violation status in `anvil task get` output for tasks with SLA configured.
- **FR-007**: System MUST provide an `anvil task sla` command that lists all tasks with SLA violations in the current project.
- **FR-008**: System MUST support `--verbose` flag on `anvil task sla` to show all SLA-configured tasks with their status.
- **FR-009**: System MUST support `--reset` flag on `anvil task sla` to clear all SLA violation records.
- **FR-010**: System MUST support a global `sla.default_max_delay` in the config file that applies to tasks without per-task SLA.
- **FR-011**: System MUST persist SLA violation records across daemon restarts.
- **FR-012**: System MUST NOT apply SLA tracking to one-shot tasks or persistent tasks (only cron-scheduled tasks).
- **FR-013**: System MUST treat tasks with no SLA configuration (and no global default) identically to current behavior (fully backward compatible).

### Key Entities

- **SLA Config**: Per-task configuration with max_delay threshold and optional strict mode. Controls when violations are detected.
- **SLA Violation Record**: A persisted record of a specific SLA violation including task name, scheduled time, actual dispatch time, delay duration, and whether strict mode skipped execution.
- **Global SLA Config**: System-wide default SLA settings that apply when tasks lack per-task configuration.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can configure SLA thresholds per task and receive notification within seconds when a task exceeds its allowed delay.
- **SC-002**: Users can view SLA status for any task with a single command and immediately see whether its last run was on time or violated SLA.
- **SC-003**: Users can see all SLA violations across their project in a single dashboard view.
- **SC-004**: Users can reset violation history after intentional downtime with one command.
- **SC-005**: Existing tasks without SLA configuration continue to operate identically to current behavior (zero breaking changes).
- **SC-006**: SLA violation data survives daemon restarts — users never lose violation history unexpectedly.

## Assumptions

- Dispatch delay is calculated using cron's "previous scheduled time" relative to the actual dispatch moment. This gives the most accurate measure of how late a task actually ran.
- The `on_sla_violation` hook is a shell command executed asynchronously (does not block task dispatch). Template variables `{{ .TaskName }}` and `{{ .Delay }}` are replaced before execution via simple string replacement.
- SLA violation records are stored as JSON files in the existing task history/runs directory structure for consistency with run records.
- The `--reset` command clears all violation records for the current project, not globally.
- Global `sla.default_max_delay` does not imply `strict: true` — strict mode must be explicitly set per task.
- For tasks running every minute (`* * * * *`), the max delay granularity is effectively 1 minute since the daemon evaluates at minute boundaries.
