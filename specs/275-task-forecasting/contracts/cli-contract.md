# CLI Contract: Task Forecasting

**Feature**: 275-task-forecasting
**Date**: 2026-03-07

## Commands

### `anvil task forecast`

Project future task executions based on cron schedules.

**Usage**:
```
anvil task forecast [flags]
```

**Flags**:

| Flag           | Type   | Default | Description                                      |
| -------------- | ------ | ------- | ------------------------------------------------ |
| `--days`       | int    | 7       | Forecast horizon in days                         |
| `--task`       | string | ""      | Filter to a specific task by name                |
| `--contention` | bool   | false   | Show resource contention windows                 |
| `--cost`       | bool   | false   | Show cost projections                            |
| `--all`        | bool   | false   | Include all watched projects                     |
| `--json`       | bool   | false   | Output in JSON format                            |
| `--verbose`    | bool   | false   | Show individual runs even for high-frequency tasks|

**Human-readable output** (default):
```
FORECAST: 2026-03-07 to 2026-03-14 (7 days)

TIME                 TASK              DURATION   COST
Mon 03/07 09:00      fetch-data        5m         $0.02
Mon 03/07 09:30      process-data      10m        $0.05
Mon 03/07 10:00      generate-report   15m        $0.08
Tue 03/08 09:00      fetch-data        5m         $0.02
...

SUMMARY: 42 runs | 3h 30m estimated runtime | $18.50 estimated cost
```

**Human-readable output with `--contention`**:
```
CONTENTION WINDOWS (next 7 days):

TIME                 CONCURRENT   WORKERS   OVERFLOW   TASKS
Mon 03/07 09:00      5            3         +2         fetch-data, process-data, report, sync, cleanup
Wed 03/09 14:00      4            3         +1         fetch-data, backup, sync, report

2 contention windows found. Consider staggering schedules or increasing workers.
```

**Human-readable output with summarization** (high-frequency tasks):
```
FORECAST: 2026-03-07 to 2026-03-14 (7 days)

TASK             RUNS/DAY   DAILY DURATION   DAILY COST
heartbeat        1440       24h              $0.00
  (every minute, use --task heartbeat --verbose for details)

TIME                 TASK              DURATION   COST
Mon 03/07 09:00      fetch-data        5m         $0.02
...
```

**JSON output** (`--json`):
```json
{
  "start": "2026-03-07T00:00:00Z",
  "end": "2026-03-14T00:00:00Z",
  "total_runs": 42,
  "total_duration_seconds": 12600,
  "total_cost_usd": 18.50,
  "runs": [
    {
      "task_id": "abc-123",
      "task_name": "fetch-data",
      "scheduled_time": "2026-03-07T09:00:00Z",
      "estimated_duration_seconds": 300,
      "estimated_cost_usd": 0.02,
      "input_tokens": 5000,
      "output_tokens": 1200,
      "has_historical_data": true,
      "is_hypothetical": false
    }
  ],
  "contention_windows": [
    {
      "start": "2026-03-07T09:00:00Z",
      "end": "2026-03-07T09:15:00Z",
      "peak_concurrent": 5,
      "worker_count": 3,
      "overflow": 2,
      "tasks": ["fetch-data", "process-data", "report", "sync", "cleanup"]
    }
  ]
}
```

**Exit codes**:
- `0`: Success
- `1`: Error (invalid flags, no project found, cron parse failure)

---

### `anvil add --dry-run`

Show forecast impact of adding a new task without persisting it.

**Usage**:
```
anvil add -s "CRON_EXPR" --dry-run [other add flags] "task name"
```

**Behavior**: Constructs a hypothetical task from the provided arguments, runs the forecast engine with it included, and displays the combined forecast. No file is written. The hypothetical task is marked with `*` in human output and `"is_hypothetical": true` in JSON output.

**Human-readable output**:
```
DRY RUN: Adding "new-task" with schedule "0 9 * * *"

FORECAST: 2026-03-07 to 2026-03-14 (7 days)

TIME                 TASK              DURATION   COST
Mon 03/07 09:00      fetch-data        5m         $0.02
Mon 03/07 09:00    * new-task          -          -
...

IMPACT: 42 → 49 runs (+7) | 3h30m → 3h30m runtime | $18.50 → $18.50 cost
CONTENTION: New overlap at 09:00 Mon-Fri (4 concurrent, 3 workers)
```

**Exit codes**:
- `0`: Success
- `1`: Error (invalid flags, schedule parse failure)

---

## Error Messages

| Condition                        | Message                                              |
| -------------------------------- | ---------------------------------------------------- |
| Invalid `--days` value           | `error: --days must be a positive integer (got: X)`  |
| No tasks found                   | `No scheduled tasks found in project.`               |
| Task not found (with `--task`)   | `error: task "X" not found`                          |
| Invalid cron in task             | `warning: skipping task "X": invalid schedule "Y"`   |
| No historical data (with --cost) | Shows "no data" per task, not a global error         |
