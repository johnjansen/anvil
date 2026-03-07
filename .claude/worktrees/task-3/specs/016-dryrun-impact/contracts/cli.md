# CLI Contract: Dry-Run Impact Analysis

## Enhanced `anvil add --dry-run`

### Human-Readable Output

```
$ anvil add -s "0 9 * * *" "New task" --dry-run

Impact Analysis
────────────────────────────────────────
Schedule:   0 9 * * *
Next Run:   Mon Mar 2 09:00:00 (3h from now)

Scheduling Conflicts (3):
  - fetch-data       (0 9 * * *)
  - process-data     (0 9 * * *)
  - daily-report     (0 9 * * 1-5)

Peak Concurrency: 4 tasks at 09:00

Suggested Alternatives:
  - 0 10 * * *  (0 conflicts)
  - 0 8 * * *   (1 conflict)
  - 30 9 * * *  (0 conflicts)
```

### No Conflicts Output

```
$ anvil add -s "0 3 * * *" "Night task" --dry-run

Impact Analysis
────────────────────────────────────────
Schedule:   0 3 * * *
Next Run:   Tue Mar 3 03:00:00 (18h from now)

No scheduling conflicts.
```

### One-Shot Task Output

```
$ anvil add --once "One-time task" --dry-run

No schedule specified (one-shot task).
```

### JSON Output

```
$ anvil add -s "0 9 * * *" "New task" --dry-run --json

{
  "schedule": "0 9 * * *",
  "valid": true,
  "next_run": "2026-03-02T09:00:00Z",
  "conflicts": [
    {"task": "fetch-data", "schedule": "0 9 * * *", "overlap_time": "2026-03-02T09:00:00Z"},
    {"task": "process-data", "schedule": "0 9 * * *", "overlap_time": "2026-03-02T09:00:00Z"}
  ],
  "peak_concurrency": 3,
  "peak_time": "2026-03-02T09:00:00Z",
  "suggestions": [
    {"schedule": "0 10 * * *", "conflicts": 0, "description": "Shift +1 hour"},
    {"schedule": "0 8 * * *", "conflicts": 1, "description": "Shift -1 hour"}
  ]
}
```

## Flag Changes

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| --dry-run | -n | bool | false | Show impact analysis without creating task |
| --json | | bool | false | Output impact analysis in JSON format (only with --dry-run) |
