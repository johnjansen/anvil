# Contract: Task Frontmatter — allowed_window

## Format

```yaml
---
schedule: "*/15 * * * *"
allowed_window:
  start: "09:00"
  end: "18:00"
  days: "1-5"
---
```

## Fields

| Field | Type   | Required | Description                         |
|-------|--------|----------|-------------------------------------|
| start | string | Yes*     | Start time HH:MM (24h format)      |
| end   | string | Yes*     | End time HH:MM (24h format)        |
| days  | string | No       | Allowed days (0=Sun, 6=Sat)        |

*If `allowed_window` is present, both `start` and `end` are required.

## Days Format

- Range: `"1-5"` (Monday through Friday)
- List: `"1,3,5"` (Monday, Wednesday, Friday)
- Combined: `"1-5,0"` (Weekdays plus Sunday)
- Omitted: All days allowed

## Midnight Spanning

When `end` < `start`, the window spans midnight:
- `start: "22:00", end: "06:00"` means 10 PM to 6 AM next day
