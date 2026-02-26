# Quickstart: Task Execution Time Windows

## Per-Task Window

Add `allowed_window` to any task's frontmatter to restrict when it can run:

```yaml
---
schedule: "*/15 * * * *"
allowed_window:
  start: "09:00"
  end: "18:00"
  days: "1-5"
---
Run my business-hours-only task
```

This task fires every 15 minutes but only executes Monday-Friday between 9 AM and 6 PM.

## Global Quiet Hours

Add to `~/.anvil/config.yaml`:

```yaml
quiet_hours:
  enabled: true
  start: "22:00"
  end: "07:00"
  exclude_priority: 0
```

All tasks except p0 are blocked from 10 PM to 7 AM.

## Force Run

Override all window restrictions for immediate execution:

```bash
anvil task run my-task --force
```

## Check Next Run

See when a windowed task will next execute:

```bash
anvil task next my-task --verbose
```
