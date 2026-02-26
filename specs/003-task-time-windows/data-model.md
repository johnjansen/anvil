# Data Model: Task Execution Time Windows

## Entities

### AllowedWindow

Per-task time window constraint parsed from task frontmatter YAML.

| Field | Type   | Description                                      | Default |
|-------|--------|--------------------------------------------------|---------|
| Start | string | Start time in HH:MM format (24h)                 | ""      |
| End   | string | End time in HH:MM format (24h)                   | ""      |
| Days  | string | Allowed days: range "1-5", list "1,3,5", or both | ""      |

**Validation rules**:
- Start and End must be valid HH:MM (00:00-23:59)
- If Start is set, End must also be set (and vice versa)
- Days uses 0=Sunday through 6=Saturday
- Empty Days means all days allowed
- When End < Start, window spans midnight

**State transitions**: None — this is a static configuration read at dispatch time.

### QuietHoursConfig

Global quiet hours configuration from `~/.anvil/config.yaml`.

| Field           | Type | Description                                            | Default |
|-----------------|------|--------------------------------------------------------|---------|
| Enabled         | bool | Whether quiet hours are active                         | false   |
| Start           | string | Start time in HH:MM format (24h)                    | ""      |
| End             | string | End time in HH:MM format (24h)                      | ""      |
| ExcludePriority | int  | Tasks with priority <= this value bypass quiet hours   | 0       |

**Validation rules**:
- Same time format rules as AllowedWindow
- ExcludePriority range: 0-9
- When End < Start, quiet hours span midnight

### Todo (modified)

Existing entity with new fields added.

| New Field      | Type          | Description                        |
|----------------|---------------|------------------------------------|
| AllowedWindow  | AllowedWindow | Per-task time window (from YAML)   |

### Config (modified)

Existing entity with new field added.

| New Field  | Type             | Description                 |
|------------|------------------|-----------------------------|
| QuietHours | QuietHoursConfig | Global quiet hours settings |

### RunRequest (modified)

Existing entity with new field added.

| New Field | Type | Description                              |
|-----------|------|------------------------------------------|
| Force     | bool | If true, bypass all time window checks   |

## Relationships

```
Config 1---1 QuietHoursConfig : "has global quiet hours"
Todo 1---0..1 AllowedWindow : "may have per-task window"
Daemon *---1 Config : "reads quiet hours from"
Daemon *---* Todo : "evaluates windows for"
RunRequest ----> Todo : "may force-bypass window for"
```

## Evaluation Logic

At dispatch time, for each due task:

1. If task has AllowedWindow → check current time is within window
2. If config has QuietHours enabled → check current time is outside quiet hours (or task priority is exempt)
3. Both constraints must pass (AND logic)
4. Force-run requests bypass both checks
