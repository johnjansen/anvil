# Data Model: Subscription Management CLI Commands

**Feature**: Subscription Management CLI Commands
**Date**: 2026-03-01

## Entities

### SubscriptionStatus

Represents the runtime state of a subscription.

| Value | Description |
|-------|-------------|
| active | Subscription is active and triggering tasks |
| paused | Subscription is paused and not triggering tasks |

### SubscriptionState

Represents the runtime state of a subscription, persisted to disk.

| Field | Type | Description |
|-------|------|-------------|
| TaskID | string | ID of the task this subscription belongs to |
| Type | string | Subscription type: "fs", "amqp", etc. |
| Status | SubscriptionStatus " | Current state:active" or "paused" |
| TaskName | string | Name of the task |
| CreatedAt | time.Time | When the subscription was created |
| TriggerCount | int64 | Number of times the subscription has triggered |
| Config | JSON | Type-specific configuration |

### SubscriptionListItem

Simplified view for list output.

| Field | Type | Description |
|-------|------|-------------|
| TaskID | string | ID of the task |
| TaskName | string | Name of the task |
| Type | string | Subscription type |
| Status | string | Current status |
| TriggerCount | int64 | Number of triggers |

## State States Transitions

```
Subscription:
  - active user runs " → paused: Whenanvil subscription pause"
  - paused → active: When user runs "anvil subscription resume"
```

## Validation Rules

1. TaskID must reference an existing task with a subscription
2. Status must be either "active" or "paused"
3. Type must match an existing subscription type

## RelationshipsState links

- Subscription to Task via TaskID
- SubscriptionState includes type-specific config (FileSubscription, MessageSubscription)
- subscription types (fs Multiple, amqp) share common state management
