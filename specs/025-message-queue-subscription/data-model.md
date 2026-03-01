# Data Model: Message Queue Subscription for Task Triggers

**Feature**: Message Queue Subscription for Task Triggers
**Date**: 2026-03-01

## Entities

### MessageEvent

Represents a message received from an AMQP queue.

| Field | Type | Description |
|-------|------|-------------|
| Payload | string | The message body/content |
| QueueName | string | Name of the queue the message was received from |
| Timestamp | int64 | Unix timestamp of when the message was received |
| DeliveryTag | uint64 | AMQP delivery tag for acknowledgment |
| ContentType | string | MIME type of the payload (if specified) |

### MessageSubscription

Configuration for a message queue subscription on a task.

| Field | Type | Description |
|-------|------|-------------|
| TaskID | string | ID of the task this subscription belongs to |
| TaskName | string | Name of the task |
| ConnectionURL | string | AMQP connection URL (amqp://user:pass@host:port/vhost) |
| QueueName | string | Name of the queue to subscribe to |
| Exchange | string | Exchange to bind queue to (optional) |
| BindingKey | string | Routing key for exchange binding (optional) |
| AutoAck | bool | Automatically acknowledge messages (default: true) |
| DeclareQueue | bool | Create queue if it doesn't exist (default: true) |
| Enabled | bool | Whether the subscription is active |

### MessageEventData

Data passed to a triggered task via environment variables.

| Field | Env Variable | Description |
|-------|--------------|-------------|
| Payload | ANVIL_AMQP_PAYLOAD | The full message payload |
| QueueName | ANVIL_AMQP_QUEUE | Name of the queue |
| Timestamp | ANVIL_AMQP_TIMESTAMP | Unix timestamp |
| ContentType | ANVIL_AMQP_CONTENT_TYPE | MIME type if specified |

## Configuration Schema

Following the schema pattern from spec 016-task-subscriptions and similar to filesystem:

```yaml
subscription:
  type: amqp
  url: amqp://guest:guest@localhost:5672/  # Connection URL
  queue: orders                              # Queue name
  exchange: ""                               # Optional: exchange name
  binding_key: ""                            # Optional: routing key
  auto_ack: true                            # Optional: auto-acknowledge
  declare_queue: true                       # Optional: create queue if not exists
```

## State Transitions

```
MessageSubscription States:
  - Disabled → Enabled: When subscription is created or resumed
  - Enabled → Disabled: When subscription is paused
  - Enabled → Connecting: When attempting to connect to broker
  - Connecting → Enabled: When successfully connected
  - Enabled/Connecting → Error: When connection fails
  - Error → Connecting: When attempting to reconnect
```

## Validation Rules

1. ConnectionURL must be a valid AMQP URL format
2. QueueName must be a non-empty string
3. AutoAck must be a boolean
4. TaskID must reference an existing task
5. Connection credentials should be handled securely (not logged)

## Relationships

- MessageSubscription belongs to one Task (via TaskID)
- MessageSubscription receives MessageEvent(s) when messages arrive on the queue
- MessageEvent triggers Task execution with MessageEventData

## Similarities to Filesystem Subscription

This data model follows the same pattern as the filesystem subscription (spec 001):

- Subscription entity with TaskID, enabled state, and configuration
- Event entity representing the trigger data
- EventData passed to task via environment variables
- State machine for enabled/disabled/error states
- JSON file storage in `.anvil/subscriptions/`
