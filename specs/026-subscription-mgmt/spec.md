# Feature Specification: Subscription Management CLI Commands

**Feature Branch**: `026-subscription-mgmt`
**Created**: 2026-03-01
**Status**: Draft
**Input**: User description: "Add subscription management CLI commands

## Problem
Users need to manage task subscriptions (list, pause, resume).

## Proposed Solution
Add subscription management commands:

## Acceptance Criteria
- [ ] anvil subscription ls shows active subscriptions
- [ ] Subscriptions can be paused/resumed
- [ ] Status reflects current subscription state"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - List Active Subscriptions (Priority: P1)

A user wants to see all active task subscriptions and their current status. This is the primary way to understand what subscriptions are running and monitor their health.

**Why this priority**: Users cannot manage what they cannot see. Listing subscriptions is the foundational capability needed for all subscription management tasks.

**Independent Test**: Can be fully tested by running `anvil subscription ls` and verifying it displays all active subscriptions with their status, type, and configuration.

**Acceptance Scenarios**:

1. **Given** the daemon is running with active subscriptions, **When** user runs `anvil subscription ls`, **Then** the command returns a list of all subscriptions showing their ID, task name, subscription type (fs, amqp, etc.), status (active/paused), and configuration summary.

2. **Given** the daemon is running with no subscriptions, **When** user runs `anvil subscription ls`, **Then** the command returns an empty list with a helpful message indicating no subscriptions exist.

3. **Given** the daemon is not running, **When** user runs `anvil subscription ls`, **Then** the command returns an error indicating the daemon is not available.

---

### User Story 2 - Pause a Subscription (Priority: P1)

A user wants to temporarily stop a subscription from triggering tasks without deleting the subscription configuration. This is useful for maintenance windows or troubleshooting.

**Why this priority**: Pausing subscriptions is essential for operational control, allowing users to stop event-driven task triggers without removing configuration.

**Independent Test**: Can be fully tested by pausing a subscription and verifying it no longer triggers tasks, but retains its configuration for resuming.

**Acceptance Scenarios**:

1. **Given** an active subscription exists, **When** user runs `anvil subscription pause <subscription-id>`, **Then** the subscription status changes to "paused" and the subscription stops triggering tasks.

2. **Given** a subscription that is already paused, **When** user runs `anvil subscription pause <subscription-id>`, **Then** the command succeeds with no error and status remains "paused".

3. **Given** a non-existent subscription ID, **When** user runs `anvil subscription pause <subscription-id>`, **Then** the command returns an error indicating the subscription was not found.

---

### User Story 3 - Resume a Paused Subscription (Priority: P1)

A user wants to reactivate a paused subscription to resume triggering tasks based on events.

**Why this priority**: Completes the pause/resume lifecycle, allowing users to temporarily disable and re-enable subscriptions as needed.

**Independent Test**: Can be fully tested by resuming a paused subscription and verifying it starts triggering tasks again.

**Acceptance Scenarios**:

1. **Given** a paused subscription exists, **When** user runs `anvil subscription resume <subscription-id>`, **Then** the subscription status changes to "active" and the subscription resumes triggering tasks.

2. **Given** an active subscription, **When** user runs `anvil subscription resume <subscription-id>`, **Then** the command succeeds with no error and status remains "active".

3. **Given** a non-existent subscription ID, **When** user runs `anvil subscription resume <subscription-id>`, **Then** the command returns an error indicating the subscription was not found.

---

### User Story 4 - View Subscription Details (Priority: P2)

A user wants detailed information about a specific subscription including its configuration, current status, and statistics.

**Why this priority**: Provides deeper visibility into subscription behavior and helps troubleshoot issues.

**Independent Test**: Can be fully tested by viewing a subscription's details and verifying all relevant information is displayed.

**Acceptance Scenarios**:

1. **Given** a subscription exists, **When** user runs `anvil subscription info <subscription-id>`, **Then** the command returns detailed information including subscription ID, task name, type, status, configuration, creation time, and trigger count.

---

### Edge Cases

- What happens when the daemon restarts - do paused subscriptions maintain their paused state?
- How does the system handle subscription configuration changes while paused?
- What happens when trying to pause/resume a subscription for a deleted task?
- How are subscriptions from different types (filesystem, message queue) displayed uniformly?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST provide a `subscription ls` command that lists all active subscriptions with their ID, task name, type, and status.
- **FR-002**: The CLI MUST provide a `subscription pause <id>` command that pauses a specific subscription, changing its status to "paused".
- **FR-003**: The CLI MUST provide a `subscription resume <id>` command that resumes a paused subscription, changing its status to "active".
- **FR-004**: The CLI MUST provide a `subscription info <id>` command that displays detailed information about a specific subscription.
- **FR-005**: The system MUST persist subscription state (active/paused) across daemon restarts.
- **FR-006**: The system MUST validate subscription IDs and return clear error messages for invalid IDs.
- **FR-007**: The CLI MUST support a `--json` flag for machine-readable output on all subscription commands.

### Key Entities

- **Subscription**: Represents a configured event trigger for a task, with attributes: ID, task reference, type (fs, amqp, etc.), status (active/paused), configuration, creation timestamp, trigger count.
- **Subscription Status**: Enum representing the current state: active (triggering tasks), paused (not triggering tasks).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view all active subscriptions within 2 seconds of command execution.
- **SC-002**: Users can successfully pause and resume any active subscription with immediate effect on task triggering.
- **SC-003**: 100% of subscription state changes are persisted and recovered after daemon restart.
- **SC-004**: All subscription commands return appropriate error messages within 1 second for invalid operations.
