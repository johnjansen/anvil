# Tasks: Message Queue Subscription for Task Triggers

**Feature**: 025-message-queue-subscription
**Generated**: 2026-03-01
**Spec**: [spec.md](spec.md)
**Plan**: [plan.md](plan.md)

## Summary

- **Total Tasks**: 16
- **User Stories**: 3 (US1-P1: Configure Subscription, US2-P1: Access Payload, US3-P2: Connection Settings)
- **MVP Scope**: US1 + US2 (P1 stories) = Tasks T001-T009
- **Parallel Opportunities**: T004/T005 can run in parallel (different structs), T007/T008 can run in parallel (different files)

## Implementation Strategy

**MVP First**: Complete Phase 3 (US1 - Configure Subscription) and Phase 4 (US2 - Access Payload) for a minimal viable product. This enables basic message queue triggering with payload access.

**Incremental Delivery**:
- Phase 3-4 (MVP): Basic AMQP subscription with payload delivery
- Phase 5: Add connection configuration and error handling
- Phase 6: Polish and edge cases

## Dependencies

```
Phase 2 (Foundational)
  └── T003: Add AMQPConfig to project → Blocks all User Stories

Phase 3 (US1 - Configure Subscription)
  └── Depends on T003

Phase 4 (US2 - Access Payload)
  └── Depends on T003, T006

Phase 5 (US3 - Connection Settings)
  └── Depends on T003

Phase 6 (Polish)
  └── Depends on all phases complete
```

## Phase 1: Setup

- [ ] T001 Add AMQP dependency to go.mod (rabbitmq/amqp091-go)

## Phase 2: Foundational

- [ ] T002 Create amqp package in internal/subscription/amqp/
- [ ] T003 [P] Add AMQPSubscription struct and AMQPConfig field to Todo in internal/project/project.go

## Phase 3: User Story 1 - Configure Message Queue Subscription (P1)

**Goal**: Enable users to define AMQP subscriptions that trigger tasks when messages arrive
**Independent Test**: Create task with subscription.amqp, publish message to queue, verify task triggers

- [ ] T004 [P] [US1] Add MessageEvent, MessageSubscription structs in internal/subscription/amqp/amqp.go
- [ ] T005 [P] [US1] Add MessageEventData struct in internal/subscription/amqp/amqp.go
- [ ] T006 [US1] Implement AMQP connection and consumer in internal/subscription/amqp/consumer.go
- [ ] T007 [US1] Implement message to task trigger conversion in internal/subscription/amqp/amqp.go
- [ ] T008 [US1] Integrate AMQP subscription in daemon startup in internal/daemon/daemon.go

## Phase 4: User Story 2 - Access Message Payload in Task (P1)

**Goal**: Enable triggered tasks to access message payload and metadata
**Independent Test**: Publish JSON message, verify task receives ANVIL_AMQP_PAYLOAD env var

- [ ] T009 [P] [US2] Add environment variable population in internal/subscription/amqp/amqp.go
- [ ] T010 [US2] Implement JSON payload parsing for field access in internal/subscription/amqp/amqp.go
- [ ] T011 [US2] Test payload delivery with sample messages

## Phase 5: User Story 3 - Configure Queue Connection Settings (P2)

**Goal**: Enable users to configure connection URL, authentication, and queue settings
**Independent Test**: Configure different connection URLs, verify correct broker connection

- [ ] T012 [P] [US3] Add exchange and binding configuration in internal/subscription/amqp/amqp.go
- [ ] T013 [US3] Implement auto-ack and manual ack modes in internal/subscription/amqp/consumer.go
- [ ] T014 [US3] Add queue declaration option in internal/subscription/amqp/consumer.go

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T015 Add connection retry and reconnection logic in internal/subscription/amqp/consumer.go
- [ ] T016 Add tests for AMQP subscription in internal/subscription/amqp/amqp_test.go

## Parallel Execution Examples

**Example 1**: T004 + T005 can run in parallel (different structs, no dependencies)

**Example 2**: T007 + T008 can run in parallel (consumer.go and daemon.go, independent)

**Example 3**: T012 + T015 can run in parallel (different features, no dependencies)

## Independent Test Criteria

| Story | Test Criteria |
|-------|---------------|
| US1 | Task with queue "orders" triggers when message published to "orders"; Task with queue "data" does NOT trigger when message published to "other" |
| US2 | Task receives ANVIL_AMQP_PAYLOAD with message body; Task receives ANVIL_AMQP_QUEUE with queue name; JSON payload is accessible as string and parsed fields |
| US3 | Valid connection URL connects successfully; Invalid URL reports error without crash; Authentication credentials work |
