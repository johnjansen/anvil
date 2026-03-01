# Quickstart: Task Alerting Rules

## Overview

Task alerting rules let you define conditions that trigger notifications when tasks run. You can alert on cost, duration, or output patterns.

## Configuration

Add alert rules to your task frontmatter:

```yaml
---
name: my-api-task
schedule: "*/5 * * * *"
alerts:
  - name: high_cost
    condition:
      type: cost
      threshold: "10.00"
    message: "Task cost exceeded $10"
    severity: warning
    action:
      webhook: "https://alerts.example.com/hook"
      notify: ["oncall-engineer"]
      retry: 3

  - name: slow_execution
    condition:
      type: duration
      threshold: "30m"
    message: "Task running over 30 minutes"
    severity: critical

  - name: output_error
    condition:
      type: output
      pattern: "ERROR:"
    message: "Task output contains error"
    severity: error
---
```

### Condition Types

- **cost**: Alert when task cost exceeds threshold (in dollars)
- **duration**: Alert when task duration exceeds threshold (e.g., "30m", "1h")
- **output**: Alert when task output matches regex pattern

### Severity Levels

- `warning`: Informational alert
- `error`: Error-level alert
- `critical`: Critical alert requiring immediate attention

### Actions

- **webhook**: URL to send alert payload (POST request)
- **notify**: List of recipients to notify
- **retry**: Number of times to retry failed webhook delivery

## CLI Commands

### View Active Alerts

```bash
anvil alerts
```

Shows all active (unacknowledged) alerts.

### Acknowledge an Alert

```bash
anvil alerts ack <alert-id>
```

Mark an alert as acknowledged.

### View Alert History

```bash
anvil alerts history
```

Shows all past alerts (both active and acknowledged).

## Examples

### High Cost Alert

```yaml
alerts:
  - name: expensive
    condition:
      type: cost
      threshold: "5.00"
    message: "Task cost over $5"
    severity: warning
```

### Duration Alert

```yaml
alerts:
  - name: too_slow
    condition:
      type: duration
      threshold: "10m"
    message: "Task taking over 10 minutes"
    severity: critical
```

### Output Pattern Alert

```yaml
alerts:
  - name: errors
    condition:
      type: output
      pattern: "(?i)error|failed|exception"
    message: "Task output contains error"
    severity: error
```
