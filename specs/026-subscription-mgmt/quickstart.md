# Quickstart: Subscription Management CLI Commands

## Overview

Manage task subscriptions with the new `anvil subscription` commands. List active subscriptions, pause/resume them, and view details.

## Commands

### List All Subscriptions

```bash
# List all subscriptions
anvil subscription ls

# JSON output for scripting
anvil subscription ls --json
```

### Pause a Subscription

```bash
# Pause a specific subscription by task ID
anvil subscription pause my-task-id
```

### Resume a Subscription

```bash
# Resume a paused subscription
anvil subscription resume my-task-id
```

### View Subscription Details

```bash
# Show detailed information about a subscription
anvil subscription info my-task-id

# JSON output
anvil subscription info my-task-id --json
```

## Examples

### Check All Active Subscriptions

```bash
$ anvil subscription ls
TASK NAME        TYPE    STATUS    TRIGGERS
process-data     fs      active    42
sync-orders      amqp    active    156
backup-files     fs      paused    0
```

### Pause a Subscription

```bash
$ anvil subscription pause process-data
Subscription paused: process-data
```

### Resume a Subscription

```bash
$ anvil subscription resume backup-files
Subscription resumed: backup-files
```

## Requirements

- Anvil daemon must be running
- At least one task with a subscription configured (fs or amqp)
