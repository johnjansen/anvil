# Feature Specification: Task Alerting Rules

**Feature Branch**: `019-alert-rules`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "Add task alerting rules for custom notifications"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Define Alert Rules in Task Frontmatter (Priority: P1)

A user wants to define custom alert conditions for a task so they get notified when specific thresholds are crossed. They add an `alerts` block to their task frontmatter with conditions based on cost, duration, or output patterns.

**Why this priority**: This is the core capability. Without defining alert rules, no other alerting features work.

**Independent Test**: Can be tested by creating a task with alert rules in frontmatter, running it, and verifying alerts fire when conditions are met.

**Acceptance Scenarios**:

1. **Given** a task with `alerts: [{name: high_cost, condition: "cost > 10.00", severity: warning}]`, **When** the task completes with estimated cost > $10.00, **Then** the system generates an alert with the specified name and severity.
2. **Given** a task with `alerts: [{name: slow, condition: "duration > 30m", severity: critical}]`, **When** the task runs for more than 30 minutes, **Then** the system generates an alert.
3. **Given** a task with `alerts: [{name: errors, condition: "output contains ERROR:", severity: error}]`, **When** the task output summary contains "ERROR:", **Then** the system generates an alert.
4. **Given** a task with alert rules, **When** conditions are NOT met, **Then** no alert is generated.
5. **Given** a task with an invalid alert condition, **When** the task file is parsed, **Then** a warning is displayed and the task is still loaded.

---

### User Story 2 - Alert Actions (Priority: P2)

A user wants alerts to trigger specific actions beyond the basic notification. They configure webhook URLs on alert rules so external systems (PagerDuty, Slack, etc.) are notified when conditions are met.

**Why this priority**: Actions make alerts useful beyond console output. Most users want integration with external monitoring.

**Independent Test**: Can be tested by configuring an alert with a webhook action and verifying the webhook is called when the condition triggers.

**Acceptance Scenarios**:

1. **Given** an alert rule with `webhook: "https://example.com/hook"`, **When** the alert fires, **Then** the system sends a POST request to the webhook URL with alert details.
2. **Given** an alert with a webhook that fails, **When** the alert fires, **Then** the system retries up to 3 times before logging the failure.

---

### User Story 3 - View and Manage Alerts (Priority: P3)

A user wants to see which alerts have fired and acknowledge them. They use `anvil alerts` to list active alerts and `anvil alerts ack` to acknowledge them.

**Why this priority**: Management and acknowledgment are important for operational workflows but secondary to alerting itself.

**Independent Test**: Can be tested by triggering alerts and using the CLI to list and acknowledge them.

**Acceptance Scenarios**:

1. **Given** alerts have fired, **When** the user runs `anvil alerts`, **Then** the system displays active (unacknowledged) alerts with task name, alert name, severity, timestamp, and message.
2. **Given** an active alert, **When** the user runs `anvil alerts ack <alert-id>`, **Then** the alert is marked as acknowledged and no longer shown in the default list.
3. **Given** alerts have been fired and acknowledged, **When** the user runs `anvil alerts history`, **Then** the system shows all alerts including acknowledged ones.

---

### Edge Cases

- What happens when an alert condition references a metric not available (e.g., cost on a non-LLM task)? The condition evaluates to false (no alert).
- What happens when multiple alert rules fire simultaneously? Each alert is recorded independently.
- What happens when the alerts storage directory is missing? It is created automatically.
- What happens when alert names conflict (duplicate names on same task)? Last definition wins, with a parse warning.
- What happens when an acknowledged alert fires again on a new run? A new alert instance is created (unacknowledged).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support an `alerts` block in task frontmatter YAML containing a list of alert rules
- **FR-002**: Each alert rule MUST have a `name`, `condition`, and `severity` (info, warning, error, critical)
- **FR-003**: Alert conditions MUST support cost threshold comparisons (e.g., `cost > 10.00`)
- **FR-004**: Alert conditions MUST support duration threshold comparisons (e.g., `duration > 30m`)
- **FR-005**: Alert conditions MUST support output pattern matching (e.g., `output contains ERROR:`)
- **FR-006**: System MUST evaluate alert conditions after each task run completes
- **FR-007**: System MUST persist fired alerts with timestamp, task name, alert name, severity, message, and run ID
- **FR-008**: Alert rules MUST support an optional `message` field for custom alert messages
- **FR-009**: Alert rules MUST support an optional `webhook` field to POST alert details to an external URL
- **FR-010**: Webhook delivery MUST retry up to 3 times on failure
- **FR-011**: System MUST provide an `anvil alerts` command to list active (unacknowledged) alerts
- **FR-012**: System MUST provide an `anvil alerts ack <id>` command to acknowledge an alert
- **FR-013**: System MUST provide an `anvil alerts history` command to show all alerts including acknowledged
- **FR-014**: System MUST display appropriate error messages for invalid alert conditions at parse time

### Key Entities

- **AlertRule**: Defines a condition, severity, message, and optional action (webhook). Embedded in task frontmatter.
- **AlertRecord**: A fired alert instance with timestamp, task reference, run reference, severity, message, and acknowledgment status.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can define alert rules and receive notifications within 5 seconds of task completion when conditions are met
- **SC-002**: Alert conditions evaluate correctly for cost, duration, and output pattern conditions in 100% of cases
- **SC-003**: Users can view and manage alerts via CLI commands within normal command response time
- **SC-004**: Webhook alerts are delivered with at most 3 retry attempts within 30 seconds

## Assumptions

- Alert conditions use a simple expression syntax (not a full expression language)
- Supported condition types: `cost > N`, `duration > Nd/Nh/Nm/Ns`, `output contains PATTERN`
- Alerts are stored as JSONL files at `.anvil/alerts/<task-id>.jsonl` (append-only, similar to activity log)
- Alert IDs are auto-generated short identifiers for acknowledgment
- Escalation (re-alerting if not acknowledged) is deferred to a future iteration
- The `notify` field for team routing is deferred; webhook is the primary action mechanism
