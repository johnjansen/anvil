# Quickstart: Message Queue Subscription for Task Triggers

**Feature**: Message Queue Subscription for Task Triggers
**Date**: 2026-03-01

## Overview

Message queue subscriptions allow tasks to automatically run when messages are published to a configured AMQP queue. The task receives the message payload and metadata via environment variables.

## Configuration

Add a message queue subscription to your task frontmatter:

```yaml
# tasks/process-order.yaml
name: process-order
schedule: ""  # Empty = not on cron, triggered only by subscription
subscription:
  type: amqp
  url: amqp://guest:guest@localhost:5672/
  queue: orders
  auto_ack: true
run: ./scripts/process-order.sh
```

## Environment Variables

When triggered, your task has access to message data via environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| ANVIL_AMQP_PAYLOAD | The full message payload | {"order_id": "123", "status": "new"} |
| ANVIL_AMQP_QUEUE | Name of the queue | orders |
| ANVIL_AMQP_TIMESTAMP | Unix timestamp | 1709234567 |
| ANVIL_AMQP_CONTENT_TYPE | MIME type if specified | application/json |

## Examples

### Process orders from a queue

```yaml
name: process-orders
subscription:
  type: amqp
  url: amqp://user:pass@rabbitmq:5672/
  queue: orders
  auto_ack: true
run: node process-order.js
```

### Handle notifications with manual acknowledgment

```yaml
name: process-notifications
subscription:
  type: amqp
  url: amqp://localhost:5672/
  queue: notifications
  auto_ack: false
  declare_queue: true
run: ./handle-notification.sh
```

### Subscribe to a specific exchange

```yaml
name: process-events
subscription:
  type: amqp
  url: amqp://localhost:5672/
  queue: events
  exchange: orders
  binding_key: order.created
run: ./handle-event.sh
```

## CLI Commands

```bash
# List all subscriptions including message queue subscriptions
anvil subscription ls

# Pause a message queue subscription
anvil subscription pause process-orders

# Resume a message queue subscription
anvil subscription resume process-orders
```

## How It Works

1. When the daemon starts, it reads task frontmatter for subscription configs
2. For AMQP subscriptions, it connects to the specified message broker
3. It sets up a consumer on the configured queue
4. When a message arrives, it triggers the associated task
5. The task runs with ANVIL_AMQP_* environment variables populated
6. If auto_ack is false, acknowledgment happens after task completion
