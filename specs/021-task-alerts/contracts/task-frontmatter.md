# Contract: Task Frontmatter - Alerts

This document defines the YAML schema for alert configuration in task frontmatter.

## Schema

```yaml
alerts:                          # Optional; array of alert rules
  - name: string                 # Required; unique identifier within task
    condition:                  # Required; defines when alert triggers
      type: string              # Required; "cost" | "duration" | "output"
      threshold: string         # Required; threshold value
      pattern: string           # Optional; regex pattern for output type
    message: string             # Required; human-readable message
    severity: string            # Required; "warning" | "error" | "critical"
    action:                     # Optional; what to do when alert fires
      webhook: string           # Optional; URL to POST alert payload
      notify: []string         # Optional; list of recipients
      retry: integer            # Optional; webhook retry count (default: 0)
```

## Examples

### Cost Alert

```yaml
alerts:
  - name: high_cost
    condition:
      type: cost
      threshold: "10.00"
    message: "Task cost exceeded $10"
    severity: warning
    action:
      webhook: "https://alerts.example.com/hook"
```

### Duration Alert

```yaml
alerts:
  - name: slow_execution
    condition:
      type: duration
      threshold: "30m"
    message: "Task running over 30 minutes"
    severity: critical
```

### Output Pattern Alert

```yaml
alerts:
  - name: output_error
    condition:
      type: output
      pattern: "ERROR:"
    message: "Task output contains error"
    severity: error
    action:
      notify: ["oncall-engineer"]
```

### Multiple Alerts

```yaml
alerts:
  - name: high_cost
    condition:
      type: cost
      threshold: "5.00"
    message: "Cost alert"
    severity: warning

  - name: slow
    condition:
      type: duration
      threshold: "10m"
    message: "Duration alert"
    severity: critical
```

## Validation Rules

1. `name` must be unique within the task
2. `condition.type` must be one of: "cost", "duration", "output"
3. `condition.threshold` is required for all types
4. `condition.pattern` is only valid when `type` is "output"
5. `severity` must be one of: "warning", "error", "critical"
6. At least one of `action.webhook` or `action.notify` must be set if `action` is present
7. `action.retry` must be >= 0

## Backward Compatibility

- `alerts` field is optional in task frontmatter
- Tasks without `alerts` field function exactly as before
- Adding alerts to a task does not affect existing runs
