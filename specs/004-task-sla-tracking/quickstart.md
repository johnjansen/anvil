# Quickstart: Task SLA Tracking

## Per-Task SLA

Add `sla` to any scheduled task's frontmatter to track delay:

```yaml
---
schedule: "0 9 * * *"
sla:
  max_delay: 15m
---
Run my daily report
```

If this task runs more than 15 minutes late, an SLA violation is recorded.

## Strict Mode

Skip tasks that would run too late:

```yaml
---
schedule: "0 9 * * *"
sla:
  max_delay: 15m
  strict: true
---
Time-sensitive task — don't run if late
```

## SLA Violation Hook

Run a command when an SLA violation is detected:

```yaml
---
schedule: "0 9 * * *"
sla:
  max_delay: 15m
on_sla_violation: "curl -X POST https://hooks.slack.com/... -d '{\"text\": \"SLA violation: $ANVIL_TASK_NAME delayed by $ANVIL_SLA_DELAY\"}'"
---
```

## Check SLA Status

View SLA info for a specific task:

```bash
anvil task get daily-report
```

Output includes:
```
SLA: 15m max delay
Last Run: 2026-02-26 09:32 (32m late - SLA VIOLATION)
```

## SLA Dashboard

See all SLA violations across the project:

```bash
# Show violated tasks
anvil task sla

# Show all SLA-configured tasks
anvil task sla --verbose

# Clear violation history
anvil task sla --reset
```

## Global Default

Set a default SLA for all scheduled tasks in `~/.anvil/config.yaml`:

```yaml
sla:
  default_max_delay: 30m
```

Per-task `sla.max_delay` overrides this default.
