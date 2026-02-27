# Feature Specification: Task Subscriptions for External Event Triggers

**Feature Branch**: `[016-task-subscriptions]`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "Add task subscription for external event triggers"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - HTTP Webhook Subscription (Priority: P1)

User wants to trigger tasks via HTTP webhooks from external systems.

**Why this priority**: Core feature - enables integration with external services and CI/CD pipelines.

**Independent Test**: Can be tested by configuring a webhook subscription and sending HTTP requests to trigger the task.

**Acceptance Scenarios**:

1. **Given** a task with `subscription.type: webhook` and path configured, **When** an HTTP POST is sent to the webhook endpoint, **Then** the task is triggered and executed
2. **Given** a task with webhook secret configured, **When** a request with incorrect secret is sent, **Then** the request is rejected with 401
3. **Given** webhook payload is sent, **When** task runs, **Then** the payload is available via `ANVIL_WEBHOOK_PAYLOAD` environment variable

---

### User Story 2 - AMQP Message Queue Subscription (Priority: P1)

User wants to trigger tasks when messages arrive on a message queue.

**Why this priority**: Core feature - enables event-driven architectures with message brokers.

**Independent Test**: Can be tested by configuring an AMQP subscription and publishing messages to the queue.

**Acceptance Scenarios**:

1. **Given** a task with `subscription.type: amqp` and queue configured, **When** a message is published to the queue, **Then** the task is triggered and the message is available via environment
2. **Given** AMQP connection fails, **When** daemon tries to subscribe, **Then** error is logged and subscription retries
3. **Given** message is consumed, **When** task runs, **Then** message body is available via environment variable

---

### User Story 3 - File System Event Subscription (Priority: P1)

User wants to trigger tasks when files matching a pattern are created or modified.

**Why this priority**: Common use case - process new data files as they arrive.

**Independent Test**: Can be tested by configuring an fs subscription and creating/modifying files.

**Acceptance Scenarios**:

1. **Given** a task with `subscription.type: fs` and path pattern configured, **When** a matching file is created, **Then** the task is triggered
2. **Given** a task with `events: [create, modify]`, **When** file is modified, **Then** the task is triggered
3. **Given** file event triggers task, **When** task runs, **Then** file path is available via environment variable

---

### User Story 4 - Subscription Management CLI (Priority: P1)

User wants to manage subscriptions via CLI commands.

**Why this priority**: Provides visibility and control over subscriptions.

**Independent Test**: Can be tested by running subscription commands.

**Acceptance Scenarios**:

1. **Given** subscriptions are active, **When** `anvil subscription ls` is run, **Then** all active subscriptions are listed with type, task name, and status
2. **Given** a subscription exists, **When** `anvil subscription pause <task>` is run, **Then** the subscription stops receiving events
3. **Given** a subscription is paused, **When** `anvil subscription resume <task>` is run, **Then** the subscription resumes receiving events

---

### User Story 5 - Pause/Resume on Daemon Restart (Priority: P2)

User wants subscriptions to persist their paused state across daemon restarts.

**Why this priority**: Convenience - users shouldn't need to re-pause after restart.

**Independent Test**: Can be tested by pausing a subscription, restarting daemon, and verifying it remains paused.

**Acceptance Scenarios**:

1. **Given** a subscription is paused, **When** daemon is restarted, **Then** the subscription remains paused
2. **Given** a subscription is active, **When** daemon is restarted, **Then** the subscription resumes automatically

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tasks MUST support `subscription.type: webhook` for HTTP triggers
- **FR-002**: Tasks MUST support `subscription.type: amqp` for message queue triggers
- **FR-003**: Tasks MUST support `subscription.type: fs` for file system event triggers
- **FR-004**: Webhook subscriptions MUST support path, method, and secret configuration
- **FR-005**: AMQP subscriptions MUST support queue and url configuration
- **FR-006**: FS subscriptions MUST support path pattern and events configuration
- **FR-007**: `anvil subscription ls` MUST show all active subscriptions
- **FR-008**: `anvil subscription pause <task>` MUST pause a subscription
- **FR-009**: `anvil subscription resume <task>` MUST resume a paused subscription
- **FR-010**: Webhook payload MUST be available via `ANVIL_WEBHOOK_PAYLOAD` env var
- **FR-011**: AMQP message body MUST be available via environment variable
- **FR-012**: FS file path MUST be available via environment variable
- **FR-013**: Subscriptions MUST persist paused state across daemon restarts
- **FR-014**: Webhook endpoints MUST be validated with secret if configured

### Key Entities

- **Subscription**: External event source configuration (webhook/amqp/fs)
- **TaskConfig**: Existing config struct needs `Subscription` field added
- **SubscriptionManager**: New component to manage all subscription types

## Success Criteria *(mandurable)*

### Measurable Outcomes

- **SC-001**: Tasks can be triggered via HTTP webhooks
- **SC-002**: Tasks can be triggered via AMQP messages
- **SC-003**: Tasks can be triggered via file system events
- **SC-004**: CLI provides subscription management commands
- **SC-005**: Paused state persists across restarts
