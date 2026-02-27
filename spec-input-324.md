## Problem

Currently, notifications are limited to basic events (success, failure, timeout). Users need more sophisticated alerting:
- Alert if task runs longer than expected
- Alert if task output contains specific patterns
- Alert if cost exceeds threshold
- Escalate notifications if not acknowledged

## Proposed Solution

Add alerting rules:

### 1. Alert conditions

```yaml
---
schedule: "0 9 * * *"
alerts:
  - name: high_cost
    condition: cost > 10.00
    message: "Task cost exceeded $10"
    severity: warning

  - name: slow_execution
    condition: duration > 30m
    message: "Task running > 30 minutes"
    severity: critical

  - name: output_pattern
    condition: output =~ "ERROR:"
    message: "Task output contains error"
    severity: error
---
```

### 2. Alert actions

```yaml
alerts:
  - name: high_cost
    condition: cost > 10.00
    action:
      webhook: "https://alerts.example.com/pagerduty"
      notify: ["oncall-engineer"]
      retry: 3
```

### 3. Alert commands

```bash
# Show active alerts
anvil alerts

# Acknowledge alert
anvil alerts ack <alert-id>

# Show alert history
anvil alerts history
```

## Acceptance Criteria

- [ ] Tasks support alert rules with conditions
- [ ] Support cost, duration, output pattern conditions
- [ ] Alert actions include webhooks and notifications
- [ ] `anvil alerts` command shows active alerts
- [ ] Alert acknowledgment workflow
