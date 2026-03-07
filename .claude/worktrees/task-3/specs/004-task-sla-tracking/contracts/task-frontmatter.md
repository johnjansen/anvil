# Contract: Task Frontmatter — SLA Configuration

## Format

```yaml
---
schedule: "0 9 * * *"
sla:
  max_delay: 15m
  strict: false
on_sla_violation: "echo 'Task {{ .TaskName }} missed SLA by {{ .Delay }}'"
---
```

## Fields

### sla block

| Field     | Type     | Required | Description                                    |
|-----------|----------|----------|------------------------------------------------|
| max_delay | duration | Yes*     | Maximum allowed delay (e.g., "15m", "1h30m")   |
| strict    | bool     | No       | If true, skip task instead of running late      |

*If `sla` block is present, `max_delay` is required.

### on_sla_violation

| Field            | Type   | Required | Description                                |
|------------------|--------|----------|--------------------------------------------|
| on_sla_violation | string | No       | Shell command to run when SLA is violated   |

## Duration Format

Uses Go's `time.ParseDuration` format:
- `"15m"` — 15 minutes
- `"1h30m"` — 1 hour 30 minutes
- `"30s"` — 30 seconds
- `"2h"` — 2 hours

## Hook Environment Variables

When `on_sla_violation` fires, these environment variables are set:

| Variable                  | Description                                   |
|---------------------------|-----------------------------------------------|
| ANVIL_TASK_NAME           | Name of the task                              |
| ANVIL_PROJECT             | Project path                                  |
| ANVIL_SLA_SCHEDULED_TIME  | When the task was supposed to run (RFC3339)    |
| ANVIL_SLA_ACTUAL_TIME     | When the task was actually dispatched (RFC3339)|
| ANVIL_SLA_DELAY           | Delay duration (e.g., "20m0s")                |
| ANVIL_SLA_MAX_DELAY       | Configured max_delay (e.g., "15m0s")          |

## Interaction with Global Config

```yaml
# ~/.anvil/config.yaml
sla:
  default_max_delay: 30m
```

- Per-task `sla.max_delay` overrides global `sla.default_max_delay`
- Global default does not imply `strict: true`
- Tasks with no per-task SLA and no global default have no SLA tracking
