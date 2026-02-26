# Data Model: Task SLA Tracking

## Entities

### SLAConfig

Per-task SLA configuration parsed from task frontmatter YAML.

| Field      | Type          | Description                                      | Default |
|------------|---------------|--------------------------------------------------|---------|
| MaxDelay   | time.Duration | Maximum allowed delay before violation            | 0       |
| Strict     | bool          | If true, skip task instead of running late        | false   |

**Validation rules**:
- MaxDelay of 0 means SLA tracking disabled for this task
- MaxDelay parsed via `time.ParseDuration()` (e.g., "15m", "1h30m")
- Strict only meaningful when MaxDelay > 0

**State transitions**: None — static configuration read at dispatch time.

### SLAGlobalConfig

Global SLA configuration from `~/.anvil/config.yaml`.

| Field          | Type   | Description                                          | Default |
|----------------|--------|------------------------------------------------------|---------|
| DefaultMaxDelay| string | Default max_delay for tasks without per-task SLA     | ""      |

**Validation rules**:
- Empty string means no global default (SLA disabled unless per-task configured)
- Parsed via `time.ParseDuration()` at dispatch time

### Todo (modified)

Existing entity with new fields added.

| New Field        | Type      | Description                              |
|------------------|-----------|------------------------------------------|
| SLA              | SLAConfig | Per-task SLA configuration (from YAML)   |
| OnSLAViolation   | string    | Shell command to run on SLA violation     |

### RunRecord (modified)

Existing entity with new fields added.

| New Field      | Type          | Description                                  |
|----------------|---------------|----------------------------------------------|
| ScheduledTime  | time.Time     | When the task was supposed to run (cron prev) |
| DispatchDelay  | time.Duration | Actual delay from scheduled time              |
| SLAViolation   | bool          | Whether this run violated SLA                 |
| SLAMaxDelay    | time.Duration | Configured max_delay at time of dispatch      |
| SLASkipped     | bool          | True if strict mode skipped this run          |

### Config (modified)

Existing entity with new field added.

| New Field | Type            | Description                 |
|-----------|-----------------|------------------------------|
| SLA       | SLAGlobalConfig | Global SLA settings          |

## Relationships

```
Config 1---1 SLAGlobalConfig : "has global SLA defaults"
Todo 1---0..1 SLAConfig : "may have per-task SLA"
Todo 1---0..1 OnSLAViolation : "may have SLA violation hook"
RunRecord *---0..1 SLAViolation : "may record SLA violation data"
Daemon *---1 Config : "reads global SLA defaults from"
Daemon *---* Todo : "evaluates SLA for"
```

## Evaluation Logic

At dispatch time, for each cron-matched task:

1. Determine effective SLA: per-task `sla.max_delay` overrides global `sla.default_max_delay`
2. If no effective SLA → skip SLA tracking entirely (backward compatible)
3. Calculate scheduled time: `cron.Parser.Prev(now)` gives most recent match
4. Calculate delay: `now - scheduledTime`
5. If delay > effective max_delay:
   a. Record SLA violation in RunRecord
   b. If `strict: true` → skip task, record `SLASkipped: true`
   c. If `strict: false` → run task, fire `on_sla_violation` hook
6. If delay <= max_delay → normal dispatch, record scheduled time in RunRecord
