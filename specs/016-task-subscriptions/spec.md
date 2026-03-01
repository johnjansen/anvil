# Feature Specification: Task Subscriptions for External Event Triggers

**Feature Branch**: `[016-task-subscriptions]`
**Created**: 2026-03-02
**Status**: Draft
**Input**: Issue #349: "Add message queue subscription for task triggers"

## Problem Statement

Currently, tasks only run on cron schedules. Users need to trigger tasks based on external events like:
- HTTP requests from external systems
- Message queue messages
- File system events
- Custom webhook endpoints

This spec addresses the message queue subscription functionality specifically.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Message Queue (AMQP) Subscription (Priority: P1)

User wants to trigger tasks based on messages from a message queue (RabbitMQ).

**Why this priority**: Core feature for async processing and integration with message queues.

**Independent Test**: Can be tested by configuring AMQP subscription and publishing messages to the queue.

**Acceptance Scenarios**:

1. **Given** a task with `subscription.type: amqp` and queue `task-queue`, **When** a message is published to that queue, **Then** the task is triggered and runs
2. **Given** an AMQP subscription with invalid connection URL, **When** daemon starts, **Then** error is logged and subscription is marked as failed
3. **Given** a message queue subscription, **When** the queue connection is lost, **Then** daemon attempts to reconnect automatically
4. **Given** a message with payload, **When** task runs, **Then** message payload is available to the task

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tasks MUST support `subscription` field in frontmatter with `type`, `queue`, and `url` properties
- **FR-002**: AMQP subscriptions MUST connect to message queue and trigger on message receipt
- **FR-003**: Message payload MUST be available to triggered tasks via environment variables
- **FR-004**: AMQP connections MUST handle reconnection automatically on connection loss
- **FR-005**: Invalid connection URLs MUST be handled gracefully with proper error logging

### Configuration Schema

```yaml
subscription:
  type: amqp

  # AMQP options
  queue: task-queue                 # Queue name to consume from
  url: amqp://localhost:5672       # Connection URL
  prefetch: 1                      # Number of messages to prefetch
```

### Key Entities

- **Subscription**: Event source configuration linked to a task
- **AMQPConsumer**: Message queue consumer component
- **SubscriptionManager**: Component that manages all subscriptions and lifecycle

## Success Criteria *(mandurable)*

### Measurable Outcomes

- **SC-001**: Tasks can be triggered via AMQP message queue
- **SC-002**: Message payload is available to triggered tasks
- **SC-003**: Connection recovery works on connection loss
- **SC-004**: Invalid configurations are handled gracefully

## Implementation Plan

### Phase 1: Core AMQP Support
1. Add subscription configuration parsing to task config
2. Create AMQP consumer component
3. Integrate with daemon startup/shutdown
4. Handle message payload delivery to tasks

### Phase 2: Robustness Features
1. Implement automatic reconnection
2. Add connection error handling
3. Add graceful shutdown support

### Phase 3: CLI Commands
1. Add subscription management commands (covered in issue #352)