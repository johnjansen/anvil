# Feature Specification: Task Alerting Rules

**Feature Branch**: `021-task-alerts`
**Created**: 2026-03-01
**Status**: Draft
**Input**: User description: "Add task alerting rules for custom notifications"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Alert Conditions (Priority: P1)

As a task owner, I want to define conditions that trigger alerts so that I am notified when something needs my attention.

**Why this priority**: This is the core value of the feature - without alert conditions, there can be no alerting.

**Independent Test**: Can be tested by creating a task with alert conditions and verifying alerts fire when conditions are met.

**Acceptance Scenarios**:

1. **Given** a task with a cost alert condition (cost > $10), **When** the task completes with cost $15, **Then** an alert is triggered and notification sent.
2. **Given** a task with a duration alert condition (duration > 30 minutes), **When** the task runs for 45 minutes, **Then** an alert is triggered.
3. **Given** a task with an output pattern alert condition (output contains "ERROR:"), **When** the task outputs "ERROR: connection failed", **Then** an alert is triggered.

---

### User Story 2 - Configure Alert Actions (Priority: P1)

As a task owner, I want to specify what happens when an alert triggers so that the right people are notified.

**Why this priority**: Alerts are only useful if they reach the right people through the right channels.

**Independent Test**: Can be tested by configuring alert actions and verifying they execute when alerts fire.

**Acceptance Scenarios**:

1. **Given** an alert with a webhook action, **When** the alert fires, **Then** the webhook receives a POST request with alert details.
2. **Given** an alert with a notify action listing "oncall-engineer", **When** the alert fires, **Then** the oncall-engineer receives notification.
3. **Given** an alert with retry: 3, **When** the webhook fails, **Then** the system retries up to 3 times.

---

### User Story 3 - View and Acknowledge Alerts (Priority: P2)

As a task owner, I want to see active alerts and acknowledge them so that I can track and manage alerting state.

**Why this priority**: Users need visibility into what's alerting and the ability to silence acknowledged alerts.

**Independent Test**: Can be tested by viewing active alerts and acknowledging one, verifying it no longer appears as active.

**Acceptance Scenarios**:

1. **Given** active alerts exist, **When** running `anvil alerts`, **Then** all active alerts are displayed with task name, condition, and timestamp.
2. **Given** an active alert, **When** running `anvil alerts ack <alert-id>`, **Then** the alert is marked as acknowledged.
3. **Given** alerts have been triggered, **When** running `anvil alerts history`, **Then** past alerts (both active and acknowledged) are shown.

---

### User Story 4 - Define Multiple Alert Rules (Priority: P3)

As a task owner, I want to define multiple alert rules for a single task so that I can monitor different conditions.

**Why this priority**: Users may want to be alerted on multiple metrics with different severities.

**Independent Test**: Can be tested by defining multiple alerts on a task and verifying each fires independently.

**Acceptance Scenarios**:

1. **Given** a task with multiple alert rules (cost, duration, output), **When** each condition is met at different times, **Then** separate alerts are triggered for each.
2. **Given** a task with alerts of different severity levels (warning, error, critical), **When** alerts fire, **Then** each alert carries its defined severity.

---

### Edge Cases

- What happens when the alert webhook URL is unreachable?
- How does the system handle alerts when multiple conditions are met simultaneously?
- What happens to alerts when the task is deleted?
- How are alerts handled during daemon downtime - do they get missed or replayed?
- What happens when the task runs multiple times - does each run generate new alerts?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tasks MUST support alert rules in configuration
- **FR-002**: System MUST support cost-based alert conditions (cost > threshold)
- **FR-003**: System MUST support duration-based alert conditions (duration > threshold)
- **FR-004**: System MUST support output pattern matching conditions (output =~ "pattern")
- **FR-005**: System MUST support alert actions including webhook, notify, and retry
- **FR-006**: System MUST provide `anvil alerts` command to show active alerts
- **FR-007**: System MUST provide `anvil alerts ack` command to acknowledge alerts
- **FR-008**: System MUST provide `anvil alerts history` command to show past alerts
- **FR-009**: Alerts MUST include severity level (warning, error, critical)
- **FR-010**: Alert configuration MUST support custom message templates

### Key Entities *(include if feature involves data)*

- **AlertRule**: Defines a condition and action for alerting
  - name: identifier for the alert
  - condition: the expression that triggers the alert
  - message: human-readable message when alert fires
  - severity: warning, error, or critical
  - action: what happens when alert fires (webhook, notify, retry)

- **Alert**: An instance of an alert rule that has fired
  - alert_id: unique identifier
  - task_id: reference to the task
  - rule_name: which rule triggered
  - fired_at: timestamp
  - acknowledged: boolean
  - acknowledged_at: timestamp (if acknowledged)

- **AlertHistory**: Record of all alerts (active and past)
  - Same as Alert plus final state

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can configure at least 3 types of alert conditions (cost, duration, output pattern)
- **SC-002**: Alert webhook delivery succeeds within 5 seconds of alert trigger
- **SC-003**: `anvil alerts` command returns active alerts within 1 second
- **SC-004**: Alert acknowledgment updates within 500ms of user command
- **SC-005**: System supports at least 100 active alerts without performance degradation
