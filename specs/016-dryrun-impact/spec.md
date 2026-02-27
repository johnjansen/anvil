# Feature Specification: Dry-Run Impact Analysis

**Feature Branch**: `016-dryrun-impact`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "Add task dry-run impact analysis before adding new tasks (#319)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Schedule Conflict Detection (Priority: P1)

A user is adding a new scheduled task and wants to know if it will conflict with existing tasks that run at the same time. Before committing the task, they see which existing tasks share the same scheduled time slots.

**Why this priority**: Schedule conflicts are the most common source of worker bottlenecks and the most actionable piece of information when deciding whether to add a task.

**Independent Test**: Can be fully tested by running `anvil add -s "0 9 * * *" "Test task" --dry-run` against a project with existing tasks scheduled at 09:00, and verifying the conflict list appears.

**Acceptance Scenarios**:

1. **Given** a project with tasks scheduled at "0 9 * * *", **When** user runs `anvil add -s "0 9 * * *" "New task" --dry-run`, **Then** the output lists all conflicting tasks and their schedules.
2. **Given** a project with no scheduling conflicts, **When** user runs `anvil add -s "0 3 * * *" "Night task" --dry-run`, **Then** the output shows "No scheduling conflicts".
3. **Given** a task with a complex schedule like "*/15 * * * *", **When** user runs `anvil add -s "0 * * * *" "Hourly task" --dry-run`, **Then** the output shows the 15-minute task as a conflict (since it fires at the top of each hour too).

---

### User Story 2 - Worker Load Estimate (Priority: P2)

A user wants to understand how adding a new task will affect overall worker utilization at the scheduled times. The dry-run shows how many tasks are already scheduled in the same time windows and the percentage load increase.

**Why this priority**: Worker load gives users a broader view of system health beyond just direct conflicts, helping them make informed scheduling decisions.

**Independent Test**: Can be tested by running `anvil add --dry-run` and verifying the worker load statistics appear alongside the conflict data.

**Acceptance Scenarios**:

1. **Given** 5 tasks scheduled at 09:00, **When** user runs `anvil add -s "0 9 * * *" "New task" --dry-run`, **Then** the output shows the number of concurrent tasks at that time slot (e.g., "6 tasks at 09:00").
2. **Given** a one-shot task (no schedule), **When** user runs `anvil add --once "One-time task" --dry-run`, **Then** the output shows schedule validation only (no conflict or load analysis).

---

### User Story 3 - Alternative Schedule Suggestions (Priority: P3)

When conflicts are detected, the system suggests alternative schedules that avoid the congested time slots, helping users spread their workload more evenly.

**Why this priority**: This is a convenience enhancement that builds on conflict detection -- useful but not essential for the core dry-run experience.

**Independent Test**: Can be tested by running `anvil add --dry-run` with a conflicting schedule and verifying that alternative times are suggested.

**Acceptance Scenarios**:

1. **Given** conflicts at "0 9 * * *", **When** dry-run detects conflicts, **Then** it suggests 2-3 nearby time slots with fewer conflicts (e.g., "0 10 * * *", "30 8 * * *").
2. **Given** no conflicts detected, **When** dry-run completes, **Then** no alternative suggestions are shown.

---

### Edge Cases

- What happens when the schedule is invalid (bad cron syntax)? The existing validation error is shown and no impact analysis is performed.
- What happens when there are no existing tasks in the project? The output shows "No existing tasks to compare against" and skips conflict analysis.
- What happens with persistent tasks (schedule = "persistent")? They are excluded from time-based conflict analysis since they don't follow cron schedules.
- What happens with disabled tasks? Disabled tasks are excluded from conflict analysis since they won't actually run.
- What happens with one-shot tasks (--once)? One-shot tasks have no schedule, so the output shows schedule validation only without conflict or load analysis.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST extend the existing `anvil add --dry-run` to show impact analysis including scheduling conflicts when a schedule is provided.
- **FR-002**: System MUST compare the new task's schedule against all active (non-disabled) scheduled tasks in the project to detect time overlaps.
- **FR-003**: System MUST display a list of conflicting tasks with their names and schedules when overlaps are detected.
- **FR-004**: System MUST show the total number of tasks that would run concurrently at the proposed schedule times.
- **FR-005**: System MUST suggest 2-3 alternative time slots with fewer conflicts when conflicts are detected.
- **FR-006**: System MUST preserve the existing schedule validation behavior (cron syntax checking, next run time display) as part of the enhanced dry-run.
- **FR-007**: System MUST skip conflict analysis for one-shot tasks (--once) and only show schedule validation.
- **FR-008**: System MUST exclude disabled tasks and persistent tasks from conflict analysis.
- **FR-009**: System MUST support `--json` flag on `anvil add --dry-run` to output impact analysis in machine-readable JSON format.

### Key Entities

- **Schedule Conflict**: A pair of tasks whose cron schedules produce overlapping execution times within a comparison window.
- **Time Slot**: A specific minute-resolution time point where one or more tasks are scheduled to fire within the next 24 hours.
- **Impact Report**: The collection of conflicts, worker load data, and suggested alternatives produced by the dry-run analysis.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can see all scheduling conflicts before adding a task, in under 2 seconds for projects with up to 100 tasks.
- **SC-002**: Conflict detection correctly identifies overlapping schedules by comparing next-24-hour firing times.
- **SC-003**: Alternative schedule suggestions reduce conflicts to zero or fewer conflicts than the original schedule.
- **SC-004**: The dry-run output is clear enough that users can make an informed add/skip decision without running additional commands.
