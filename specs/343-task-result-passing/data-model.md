# Data Model: Task Result Passing

## Entities

### Todo (extended)

Existing entity in `internal/project/project.go`. New field:

| Field          | Type   | Description                                    |
|----------------|--------|------------------------------------------------|
| CaptureOutput  | bool   | When true, scan stdout for `##anvil:result` lines |

Parsed from frontmatter `capture_output: true`.

### RunRecord (extended)

Existing entity in `internal/project/project.go`. New field:

| Field       | Type   | Description                                           |
|-------------|--------|-------------------------------------------------------|
| ResultData  | string | Raw data from last `##anvil:result` line (max 1MB)    |

Stored as `result_data` in the JSON run record file. Empty string when no result captured.

### DependencyResults (new, runtime only)

Not persisted — constructed at task dispatch time from dependency RunRecords.

| Field                          | Type                  | Description                                    |
|--------------------------------|-----------------------|------------------------------------------------|
| Map key: dependency task name  | string                | Task name without `.md` extension              |
| Map value: result data         | `json.RawMessage`     | Parsed JSON from dependency's `ResultData`     |

Serialized to JSON and set as `ANVIL_DEPENDENCY_RESULTS` environment variable.

## Relationships

```
Todo (capture_output: true)
  └── produces → RunRecord.ResultData (on successful completion)

RunRecord.ResultData
  └── consumed by → DependencyResults map (at dependent task dispatch)

DependencyResults
  ├── injected as → ANVIL_DEPENDENCY_RESULTS env var
  └── available as → .DependencyResults.<task> template variable
```

## State Transitions

```
Task with capture_output: true
  ├── Running → scans stdout for ##anvil:result lines (last one wins)
  ├── Success → ResultData stored in RunRecord
  └── Failure → ResultData NOT stored (or stored as empty)

Dependent task dispatch
  ├── All deps met → collect ResultData from each dep's latest RunRecord
  ├── Build DependencyResults map → serialize to JSON
  └── Inject into task env and template context
```

## Validation Rules

- `capture_output` must be a boolean (default: `false`)
- `ResultData` is capped at 1MB; data exceeding this is truncated with a warning logged
- `##anvil:result` line data is stored as-is (no JSON validation required — raw string preserved)
- Missing results for a dependency produce a `null` value in the DependencyResults map
