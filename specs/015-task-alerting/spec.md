# Feature Specification: Task Alerting Rules

**Feature Branch**: `015-task-alerting`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "Add task alerting rules for custom notifications"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure alert conditions on tasks (Priority: P1)

As a user, I want to define custom alert conditions on my tasks so that I get notified when specific conditions are met.

**Why this priority**: This is the core functionality - without configurable alert conditions, there's no alerting system.

**Independent Test**: Can be tested by creating a task with an alert condition and verifying the alert fires when the condition is met.

**Acceptance Scenarios**:

1. **Given** a task with `cost > 10.00` alert condition, **When** the task completes with cost of $15, **Then** an alert is created with message "Task cost exceeded $10"
2. **Given** a task with `duration > 30m` alert condition, **When** the task runs for 45 minutes, **Then** an alert is created
3. **Given** a task with `output =~ "ERROR:"` alert condition, **When** task output contains "ERROR:", **Then** an alert is created
4. **Given** a task without any alert configuration, **When** the task runs, **Then** no alerts are created

---

### User Story 2 - View and manage active alerts (Priority: P1)

As a user, I want to see active alerts and their status so I can respond to issues quickly.

**Why this priority**: Users need visibility into alerts to take action.

**Independent Test**: Can be tested by creating alerts and verifying `anvil alerts` shows them.

**Acceptance Scenarios**:

1. **Given** active alerts exist, **When** user runs `anvil alerts`, **Then** all active alerts are displayed with task name, severity, message, and timestamp
2. **Given** no active alerts exist, **When** user runs `anvil alerts`, **Then** a "No active alerts" message is shown

---

### User Story 3 - Acknowledge alerts (Priority: P2)

As a user, I want to acknowledge alerts so I can track that someone is handling the issue.

**Why this priority**: Alert acknowledgment is a standard alerting workflow requirement.

**Independent Test**: Can be tested by acknowledging an alert and verifying it shows as acknowledged.

**Acceptance Scenarios**:

1. **Given** an active alert, **When** user runs `anvil alerts ack <alert-id>`, **Then** the alert is marked as acknowledged with timestamp
2. **Given** an acknowledged alert, **When** user runs `anvil alerts`, **Then** the alert shows acknowledged status

---

### User Story 4 - Configure alert actions (Priority: P2)

As a user, I want to configure automatic actions when alerts fire so that the right people are notified.

**Why this priority**: Automated notification is essential for operational alerting.

**Independent Test**: Can be tested by configuring webhook action and verifying it's called when alert fires.

**Acceptance Scenarios**:

1. **Given** a task with webhook action configured, **When** alert fires, **Then** the webhook is called with alert payload
2. **Given** a task with notify list configured, **When** alert fires, **Then** configured recipients are notified
3. **Given** webhook fails, **When** alert fires, **Then** retry logic is applied per configuration

---

### User Story 5 - View alert history (Priority: P3)

As a user, I want to see historical alerts so I can analyze trends and recurring issues.

**Why this priority**: Useful for post-incident analysis and identifying patterns.

**Independent Test**: Can be tested by viewing past alerts with `anvil alerts history`.

**Acceptance Scenarios**:

1. **Given** past alerts exist, **When** user runs `anvil alerts history`, **Then** past alerts are shown with resolved status

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tasks MUST support `alerts` configuration array with name, condition, message, severity, and action
- **FR-002**: Alert conditions MUST support `cost > threshold`, `duration > threshold`, and `output =~ pattern` syntax
- **FR-003**: Alert severity MUST support `warning`, `error`, `critical` levels
- **FR-004**: `anvil alerts` command MUST show all active alerts with task, severity, message, timestamp
- **FR-005**: `anvil alerts ack <alert-id>` command MUST mark alert as acknowledged
- **FR-006**: `anvil alerts history` command MUST show past alerts
- **FR-007**: Alert actions MUST support webhook with JSON payload
- **FR-008**: Alert actions MUST support notify list for recipient notification
- **FR-009**: Alert webhook MUST support retry configuration (number of retries)
- **FR-010**: System MUST evaluate alerts after task completion

### Key Entities

- **Alert**: Represents a triggered alert with task reference, condition that triggered, message, severity, status (active/acknowledged/resolved), timestamps
- **AlertRule**: Configuration on a task defining when to create alerts (condition, severity, actions)
- **AlertHistory**: Record of past alerts for trend analysis

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can create alert rules on tasks that trigger on cost, duration, or output conditions
- **SC-002**: `anvil alerts` displays active alerts within 1 second
- **SC-003**: Alert acknowledgment persists across daemon restarts
- **SC-004**: Webhook notifications are delivered with < 5 second latency
