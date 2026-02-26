# Contract: Config — quiet_hours

## Format

```yaml
# ~/.anvil/config.yaml
quiet_hours:
  enabled: true
  start: "22:00"
  end: "07:00"
  exclude_priority: 0
```

## Fields

| Field            | Type   | Required | Default | Description                                    |
|------------------|--------|----------|---------|------------------------------------------------|
| enabled          | bool   | No       | false   | Whether quiet hours are active                 |
| start            | string | Yes*     | ""      | Start time HH:MM (24h format)                 |
| end              | string | Yes*     | ""      | End time HH:MM (24h format)                   |
| exclude_priority | int    | No       | 0       | Tasks with priority <= this bypass quiet hours |

*Required when `enabled: true`.

## Behavior

- During quiet hours, tasks with `Priority > exclude_priority` are skipped
- Default `exclude_priority: 0` means only p0 tasks run during quiet hours
- Midnight spanning works the same as per-task windows
