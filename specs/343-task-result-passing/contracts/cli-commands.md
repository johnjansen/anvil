# CLI Contract: Task Result Passing

## New Command: `anvil task results`

### `anvil task results <task-name>`

Display the most recent captured result for a task.

**Arguments**:
- `task-name` (required): Name of the task (with or without `.md` extension)

**Flags**:
- `--preview`: Show the dependency results that would be injected into this task on its next run
- `--run <run-id>`: Show results from a specific run (default: most recent)
- `--json`: Output as raw JSON (default: pretty-printed)

**Output (default)**:
```
Task: fetch-data
Run:  abc123
Time: 2026-03-07 09:00:00

Result:
  {"records": 42, "status": "success"}
```

**Output (--preview)**:
```
Task: process-data
Dependencies: fetch-data, validate-data

Dependency Results (would be injected as ANVIL_DEPENDENCY_RESULTS):
{
  "fetch-data": {"records": 42, "status": "success"},
  "validate-data": {"valid": true}
}
```

**Output (no results)**:
```
No captured results for task "fetch-data".
Ensure the task has capture_output: true in its frontmatter.
```

**Exit codes**:
- 0: Success
- 1: Task not found or no results available

## Task Output Protocol

### `##anvil:result` prefix

Tasks emit result data by printing a line to stdout:
```
##anvil:result {"key": "value"}
```

- Only the **last** `##anvil:result` line is captured (consistent with `##anvil:checkpoint` behavior)
- The line is **stripped** from downstream output (consistent with `##anvil:status` and `##anvil:checkpoint`)
- Data after the prefix is stored as-is in `RunRecord.ResultData`
- Maximum size: 1MB (truncated with warning if exceeded)

## Environment Variable

### `ANVIL_DEPENDENCY_RESULTS`

Set on dependent tasks when dependencies have captured results.

**Format**: JSON object keyed by dependency task name (without `.md` extension)

```json
{
  "fetch-data": {"records": 42, "status": "success"},
  "validate-data": null
}
```

- Keys are dependency task names as specified in `depends_on`
- Values are the parsed JSON from `ResultData`, or `null` if no result captured
- Not set if the task has no dependencies

## Frontmatter

### `capture_output`

```yaml
---
capture_output: true
---
```

- Type: boolean
- Default: `false`
- When `true`, the runner scans stdout for `##anvil:result` lines and stores the last one in the run record
