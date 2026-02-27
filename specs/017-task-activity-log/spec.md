# Feature Specification: Task Activity Log

**Feature Branch**: `017-task-activity-log`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "Add task activity log for debugging and auditing (#318)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Task Activity History (Priority: P1)

A user wants to see the complete activity history of a specific task to understand what happened over time. They run a command that shows a chronological list of all events: when the task was created, each time it ran (with outcome), and any manual interventions like pausing, editing, or killing.

**Why this priority**: This is the core value proposition — without viewing activity, no other features matter. Users need visibility into task lifecycle for debugging.

**Independent Test**: Can be fully tested by creating a task, running it a few times, pausing it, and then viewing the activity log to verify all events appear in chronological order.

**Acceptance Scenarios**:

1. **Given** a task with several runs and state changes, **When** user requests the task's activity history, **Then** all events are listed in reverse chronological order with timestamps, action types, and relevant details.
2. **Given** a task was edited (e.g., schedule changed), **When** user views activity, **Then** the edit entry shows which fields changed and their old/new values.
3. **Given** a task was killed, **When** user views activity, **Then** the kill entry appears with whether it was graceful or forced.
4. **Given** a newly created task with no runs, **When** user views activity, **Then** only the "created" event appears.

---

### User Story 2 - Filter Activity by Type and Date (Priority: P2)

A user has a task with a long activity history and wants to see only specific types of events (e.g., only runs, only edits) or events within a specific time range. They use filter flags to narrow down the activity view.

**Why this priority**: Filtering is essential for tasks with long histories but builds on the core activity tracking from US1.

**Independent Test**: Can be tested by creating a task with mixed activity types, then using type and date filters to verify only matching events appear.

**Acceptance Scenarios**:

1. **Given** a task with mixed activity types, **When** user filters by type "run", **Then** only run events are shown.
2. **Given** a task with activity spanning multiple days, **When** user filters with a since-date, **Then** only events after that date are shown.
3. **Given** an invalid filter type, **When** user provides it, **Then** a clear error message lists valid types.

---

### User Story 3 - Export Activity for Auditing (Priority: P3)

A user needs to export the activity log for compliance or audit purposes. They export the full or filtered activity to a structured file that can be ingested by audit tools.

**Why this priority**: Export is a convenience feature for compliance workflows, building on the activity data from US1.

**Independent Test**: Can be tested by exporting a task's activity to a file and verifying the file contains structured data with all activity entries.

**Acceptance Scenarios**:

1. **Given** a task with activity history, **When** user exports to a file, **Then** the file contains all activity entries in structured format.
2. **Given** filters are applied (type, since), **When** user exports, **Then** only filtered entries are included in the export.
3. **Given** the export file path already exists, **When** user exports, **Then** the file is overwritten with the new data.

---

### Edge Cases

- What happens when a task has no activity log yet (pre-existing task from before this feature)? The system shows only events that occurred after the feature was enabled. For pre-existing tasks, the first logged event will be whatever action is taken next.
- What happens when the activity log file is corrupted or missing? The system shows an appropriate error message and does not crash.
- What happens when filtering returns no results? The system shows "No matching activity entries" message.
- What happens with very large activity logs (thousands of entries)? Output is paginated or capped at a reasonable limit (default 100 entries, adjustable with --limit flag).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST record an activity entry whenever a task undergoes a state change (created, run started, run completed, paused, resumed, edited, killed, unlocked, force-run).
- **FR-002**: Each activity entry MUST include a timestamp, action type, and action-specific details.
- **FR-003**: For "run" actions, details MUST include the run ID and exit status.
- **FR-004**: For "edited" actions, details MUST include which fields changed and their old/new values.
- **FR-005**: For "killed" actions, details MUST include whether the kill was graceful or forced.
- **FR-006**: System MUST provide a command to display activity history for a specific task in reverse chronological order.
- **FR-007**: System MUST support filtering activity by action type (run, edit, kill, pause, resume, create, unlock).
- **FR-008**: System MUST support filtering activity by date (showing only entries since a given date).
- **FR-009**: System MUST support exporting activity to a structured file (JSON format).
- **FR-010**: System MUST support a --limit flag to control how many entries are displayed (default 100).
- **FR-011**: Activity data MUST persist across daemon restarts.

### Key Entities

- **ActivityEntry**: A single recorded event in a task's lifecycle. Contains timestamp, action type, and action-specific details.
- **ActivityLog**: An ordered collection of ActivityEntry records for a specific task, stored per-task.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view the complete activity history of any task in under 1 second for logs with up to 1000 entries.
- **SC-002**: All 7 activity types (create, run, pause, resume, edit, kill, unlock) are tracked automatically without user intervention.
- **SC-003**: Exported activity logs contain 100% of recorded events matching the applied filters.
- **SC-004**: Users can narrow activity to a specific type or date range, reducing noise by filtering irrelevant entries.
