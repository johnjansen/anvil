# Data Model: Task Alerting

## AlertRule (Task Configuration)

```go
type AlertRule struct {
    Name      string      `yaml:"name"`
    Condition string      `yaml:"condition"`      // e.g., "cost > 10.00", "duration > 30m", "output =~ \"ERROR:\""
    Message   string      `yaml:"message"`       // Alert message template
    Severity  AlertSeverity `yaml:"severity"`   // "warning", "error", "critical"
    Action    *AlertAction `yaml:"action"`       // Optional action configuration
}

type AlertSeverity string

type AlertAction struct {
    Webhook string   `yaml:"webhook"`     // URL to send webhook
    Notify  []string `yaml:"notify"`       // List of recipients
    Retry   int      `yaml:"retry"`        // Number of retry attempts (default 0)
}
```

## Alert (Runtime)

```go
type Alert struct {
    ID          string        `json:"id"`           // Unique alert ID (UUID)
    TaskName    string        `json:"task_name"`    // Associated task name
    RuleName    string        `json:"rule_name"`    // Name of the rule that triggered
    Condition   string        `json:"condition"`    // The condition that matched
    Message     string        `json:"message"`      // Alert message
    Severity    AlertSeverity `json:"severity"`    // warning, error, critical
    Status      AlertStatus   `json:"status"`       // active, acknowledged, resolved
    CreatedAt   time.Time     `json:"created_at"`   // When alert was created
    AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"` // When acknowledged
    ResolvedAt   *time.Time   `json:"resolved_at,omitempty"`      // When resolved
    TaskRunID   string        `json:"task_run_id"` // Associated task run
}

type AlertStatus string
```

## Storage

### Directory Structure

```
.anvil/
└── alerts/
    ├── active/           # Active alerts (one JSON file per alert)
    │   └── <alert-id>.json
    └── history/         # Historical alerts
        └── <alert-id>.json
```

### Alert File Format

```json
{
  "id": "abc123",
  "task_name": "my-task",
  "rule_name": "high_cost",
  "condition": "cost > 10.00",
  "message": "Task cost exceeded $10",
  "severity": "warning",
  "status": "active",
  "created_at": "2026-02-28T10:00:00Z",
  "task_run_id": "run-456"
}
```

## Condition Syntax

### Cost Condition
- Format: `cost > <number>`
- Example: `cost > 10.00`

### Duration Condition
- Format: `duration > <number><unit>`
- Units: `s` (seconds), `m` (minutes), `h` (hours)
- Example: `duration > 30m`

### Output Pattern Condition
- Format: `output =~ "<regex>"`
- Example: `output =~ "ERROR:"`
- Example: `output =~ "(?i)failed"`

## Webhook Payload

```json
{
  "alert": {
    "id": "abc123",
    "task_name": "my-task",
    "rule_name": "high_cost",
    "message": "Task cost exceeded $10",
    "severity": "warning",
    "status": "active",
    "created_at": "2026-02-28T10:00:00Z"
  },
  "task_run": {
    "id": "run-456",
    "cost": 15.50,
    "duration": "45m",
    "output": "..."
  }
}
```
