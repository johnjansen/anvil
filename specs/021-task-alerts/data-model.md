# Data Model: Task Alerting Rules

## Entities

### AlertCondition

Defines when an alert should trigger. Three types supported.

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| Type | string | "cost", "duration", or "output" | required |
| Threshold | string | Threshold value (e.g., "10.00" for cost, "30m" for duration) | required |
| Pattern | string | Regex pattern for output type | "" |

**Validation rules**:
- Type must be one of: "cost", "duration", "output"
- For cost: Threshold parsed as float (dollars)
- For duration: Threshold parsed via `time.ParseDuration()` (e.g., "30m", "1h")
- For output: Pattern is a regex matched against task output

### AlertAction

Defines what happens when an alert fires.

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| Webhook | string | URL to POST alert payload | "" |
| Notify | []string | List of recipients to notify | [] |
| Retry | int | Number of retries for webhook on failure | 0 |

**Validation rules**:
- At least one of Webhook or Notify must be set
- Retry must be >= 0

### AlertRule

A complete alert definition combining condition and action.

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Unique identifier for the alert rule |
| Condition | AlertCondition | When to trigger |
| Message | string | Human-readable message template |
| Severity | string | "warning", "error", or "critical" |
| Action | AlertAction | What to do when triggered |

### AlertConfig

Per-task alert configuration parsed from task frontmatter YAML.

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| Rules | []AlertRule | List of alert rules for this task | [] |

**Validation rules**:
- Each rule must have unique Name within the task
- Condition and Action are both required per rule

### AlertGlobalConfig

Global alert configuration from `~/.anvil/config.yaml`.

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| Enabled | bool | Whether alerts are globally enabled | true |
| DefaultWebhook | string | Default webhook URL for all alerts | "" |

### AlertRecord

An instance of an alert that has fired.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique alert ID (UUID) |
| TaskID | string | Task that triggered the alert |
| RuleName | string | Name of the rule that triggered |
| Condition | AlertCondition | The condition that was met |
| Message | string | The message template filled in |
| Severity | string | warning, error, or critical |
| FiredAt | time.Time | When the alert fired |
| Acknowledged | bool | Whether alert has been acknowledged |
| AcknowledgedAt | *time.Time | When alert was acknowledged (if any) |

### Todo (modified)

Existing entity with new fields added.

| New Field | Type | Description |
|-----------|------|-------------|
| Alerts | AlertConfig | Per-task alert configuration (from YAML) |

### RunRecord (modified)

Existing entity with new fields added.

| New Field | Type | Description |
|-----------|------|-------------|
| AlertsFired | []AlertRecord | Alerts that fired during this run |

### Config (modified)

Existing entity with new field added.

| New Field | Type | Description |
|-----------|------|-------------|
| Alerts | AlertGlobalConfig | Global alert settings |

## Relationships

```
Config 1---1 AlertGlobalConfig : "has global alert defaults"
Todo 1---0..1 AlertConfig : "may have alert rules"
RunRecord 1---* AlertRecord : "records alerts fired during run"
Daemon *---1 Config : "reads global alert config"
Daemon *---* Todo : "evaluates alerts for"
Daemon *---* AlertRecord : "creates when conditions met"
```

## Evaluation Logic

When a task run completes:

1. Load AlertConfig for the task (if any)
2. If no alerts configured → skip alert evaluation
3. For each AlertRule in the config:
   - **Cost condition**: Compare RunRecord.Cost > threshold
   - **Duration condition**: Compare RunRecord.Duration > threshold
   - **Output condition**: Regex match RunRecord.Output against Pattern
4. If condition met:
   a. Create AlertRecord with filled message template
   b. Execute AlertAction (async for webhook)
   c. Store AlertRecord in `.alerts/<task-id>/alerts.json`
5. Return AlertRecords to include in RunRecord

## Storage

Alert records stored in `.anvil/alerts/<task-id>/alerts.json`:

```json
{
  "task_id": "my-task",
  "alerts": [
    {
      "id": "uuid",
      "rule_name": "high_cost",
      "condition": {"type": "cost", "threshold": "10.00"},
      "message": "Task cost exceeded $10",
      "severity": "warning",
      "fired_at": "2026-03-01T10:00:00Z",
      "acknowledged": false,
      "acknowledged_at": null
    }
  ]
}
```
