# Data Model: Dry-Run Impact Analysis

## Entities

### ImpactReport
The result of analyzing a proposed schedule against existing tasks.

| Field | Type | Description |
|-------|------|-------------|
| Schedule | string | The proposed cron expression |
| IsValid | bool | Whether the cron expression is syntactically valid |
| ParseError | string | Error message if schedule is invalid |
| NextRun | time.Time | Next firing time for the proposed schedule |
| Conflicts | []Conflict | List of tasks with overlapping schedules |
| PeakConcurrency | int | Maximum number of tasks (including proposed) running at same time |
| PeakTime | time.Time | The time when peak concurrency occurs |
| Suggestions | []Suggestion | Alternative schedules with fewer conflicts |

### Conflict
A single scheduling conflict between the proposed task and an existing task.

| Field | Type | Description |
|-------|------|-------------|
| TaskName | string | Name of the conflicting existing task |
| Schedule | string | Cron schedule of the conflicting task |
| OverlapTime | time.Time | First detected overlap time |

### Suggestion
An alternative schedule that reduces conflicts.

| Field | Type | Description |
|-------|------|-------------|
| Schedule | string | The suggested cron expression |
| ConflictCount | int | Number of conflicts with this schedule |
| Description | string | Human-readable description of the change |

## State Transitions
None — ImpactReport is computed and displayed, not persisted.

## Relationships
- ImpactReport contains 0..N Conflicts
- ImpactReport contains 0..3 Suggestions
- Conflicts reference existing Todo entities by name
