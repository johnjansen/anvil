## Problem

Currently, tasks only run on cron schedules. Users need to trigger tasks based on external events like:
- HTTP requests from external systems
- Message queue messages
- File system events
- Custom webhook endpoints

## Proposed Solution

Add task subscriptions:

### 1. HTTP webhook subscriptions

```yaml
---
subscription:
  type: webhook
  path: /webhooks/trigger-task
  method: POST
  secret: env:WEBHOOK_SECRET
---
Triggered by external HTTP calls...
```

Access payload via `ANVIL_WEBHOOK_PAYLOAD` environment variable.

### 2. Message queue subscriptions

```yaml
---
subscription:
  type: amqp
  queue: task-queue
  url: amqp://localhost:5672
---
Process messages from queue...
```

### 3. File system subscriptions

```yaml
---
subscription:
  type: fs
  path: ./data/*.json
  events: [create, modify]
---
Process new data files...
```

### 4. Subscription management

```bash
# List active subscriptions
anvil subscription ls

# Pause a subscription
anvil subscription pause my-task

# Resume
anvil subscription resume my-task
```

## Acceptance Criteria

- [ ] Tasks support `subscription.webhook` for HTTP triggers
- [ ] Tasks support `subscription.amqp` for message queue
- [ ] Tasks support `subscription.fs` for file system events
- [ ] `anvil subscription ls` shows active subscriptions
- [ ] Subscriptions can be paused/resumed