# Quickstart: Task Alerting Rules

## Define alerts in frontmatter

```yaml
---
alerts:
  - name: high_cost
    condition: "cost > 10.00"
    severity: warning
  - name: slow
    condition: "duration > 30m"
    severity: critical
  - name: errors
    condition: "output contains ERROR:"
    severity: error
    webhook: "https://example.com/hook"
---
```

## View alerts

```bash
anvil alerts
```

## Acknowledge

```bash
anvil alerts ack abcd1234
```
