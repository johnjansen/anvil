# CLI Contract: Task Activity Log

## `anvil task activity <name>`

### Default Output (newest first, limit 100)

```
$ anvil task activity my-task
TIMESTAMP              ACTION      DETAILS
2026-02-28 10:00:00    run         run_id=abc123 exit=0 duration=45s
2026-02-28 09:30:00    edited      schedule: "0 9 * * *" -> "0 10 * * *"
2026-02-28 09:00:00    run         run_id=def456 exit=1 error=timeout duration=300s
2026-02-28 08:00:00    paused
2026-02-28 07:00:00    created     priority=1 schedule="0 9 * * *"
```

### Filter by Type

```
$ anvil task activity my-task --type run
TIMESTAMP              ACTION      DETAILS
2026-02-28 10:00:00    run         run_id=abc123 exit=0 duration=45s
2026-02-28 09:00:00    run         run_id=def456 exit=1 error=timeout duration=300s
```

### Filter by Date

```
$ anvil task activity my-task --since 2026-02-28
TIMESTAMP              ACTION      DETAILS
2026-02-28 10:00:00    run         run_id=abc123 exit=0 duration=45s
...
```

### Limit Results

```
$ anvil task activity my-task --limit 5
(shows only 5 most recent entries)
```

### JSON Export

```
$ anvil task activity my-task --export audit.json
Exported 42 activity entries to audit.json
```

### JSON Output to Stdout

```
$ anvil task activity my-task --json
[
  {"timestamp":"2026-02-28T10:00:00Z","action":"run","task_id":"abc123","task_name":"my-task","details":{"run_id":"abc123","exit_code":"0","success":"true","duration":"45s"}},
  ...
]
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| --type | -t | string | "" | Filter by activity type (created, run, paused, resumed, edited, killed, unlocked, force-run) |
| --since | | string | "" | Show only entries since this date (YYYY-MM-DD) |
| --limit | -l | int | 100 | Maximum entries to display |
| --export | -e | string | "" | Export entries to JSON file |
| --json | | bool | false | Output as JSON to stdout |

## Error Cases

| Input | Output |
|-------|--------|
| Unknown task name | "task not found: <name>" (exit 1) |
| Invalid --type value | "invalid activity type: <type>. Valid types: created, run, paused, resumed, edited, killed, unlocked, force-run" (exit 1) |
| Invalid --since format | "invalid date format: <date>. Use YYYY-MM-DD" (exit 1) |
| No activity entries | "No activity entries for task <name>" |
| No matching entries | "No matching activity entries" |
