# Data Model: Task Alerting Rules

## New Fields on Existing Entities

### Todo (internal/project/project.go)

Add field: `Alerts []AlertRule`

## New Entities

### AlertRule

- `Name string` - Alert rule name
- `Condition string` - Condition expression
- `Severity string` - info/warning/error/critical
- `Message string` - Custom message (optional)
- `Webhook string` - Webhook URL (optional)

### AlertRecord

- `ID string` - Short unique ID
- `Timestamp time.Time`
- `TaskID string`
- `TaskName string`
- `RunID string`
- `AlertName string`
- `Severity string`
- `Message string`
- `Condition string`
- `Acknowledged bool`
- `AckedAt time.Time`

## Storage

`.anvil/alerts/<task-id>.jsonl` (append-only JSONL)
