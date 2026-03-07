# Data Model: Task Forecasting

**Feature**: 275-task-forecasting
**Date**: 2026-03-07

## Entities

### ForecastRun

A single projected task execution within the forecast horizon.

| Field            | Type          | Description                                          |
| ---------------- | ------------- | ---------------------------------------------------- |
| TaskID           | string        | ID of the task (from Todo.ID)                        |
| TaskName         | string        | Human-readable task name (from Todo.Name)            |
| ScheduledTime    | time.Time     | Projected execution time                             |
| EstimatedDuration| time.Duration | Average duration from historical runs, or 0 if none  |
| EstimatedCostUSD | float64       | Average cost per run from history, or 0 if none      |
| InputTokens      | int           | Average input tokens per run from history             |
| OutputTokens     | int           | Average output tokens per run from history            |
| HasHistoricalData| bool          | Whether historical run data exists for estimates      |
| IsHypothetical   | bool          | True if this is from a --dry-run task                 |

### ForecastSummary

Aggregate view of the full forecast.

| Field            | Type          | Description                                          |
| ---------------- | ------------- | ---------------------------------------------------- |
| StartTime        | time.Time     | Forecast window start (now)                          |
| EndTime          | time.Time     | Forecast window end (now + days)                     |
| TotalRuns        | int           | Total projected task executions                      |
| TotalDuration    | time.Duration | Sum of estimated durations                           |
| TotalCostUSD     | float64       | Sum of estimated costs                               |
| TotalInputTokens | int           | Sum of estimated input tokens                        |
| TotalOutputTokens| int           | Sum of estimated output tokens                       |
| Runs             | []ForecastRun | All projected runs, sorted chronologically           |

### ContentionWindow

A period where concurrent tasks exceed worker capacity.

| Field            | Type          | Description                                          |
| ---------------- | ------------- | ---------------------------------------------------- |
| Start            | time.Time     | Window start time                                    |
| End              | time.Time     | Window end time                                      |
| PeakConcurrent   | int           | Maximum simultaneous tasks in this window            |
| WorkerCount      | int           | Available workers (from config)                      |
| Overflow         | int           | PeakConcurrent - WorkerCount                         |
| Tasks            | []string      | Names of tasks running in this window                |

### TaskStats

Historical statistics for a single task, used as input to forecast estimates.

| Field            | Type          | Description                                          |
| ---------------- | ------------- | ---------------------------------------------------- |
| TaskID           | string        | Task identifier                                      |
| RunCount         | int           | Number of historical runs sampled                    |
| AvgDuration      | time.Duration | Mean duration across sampled runs                    |
| AvgCostUSD       | float64       | Mean cost across sampled runs                        |
| AvgInputTokens   | int           | Mean input tokens across sampled runs                |
| AvgOutputTokens  | int           | Mean output tokens across sampled runs               |

## Existing Entities Used (no modifications)

### Todo (from internal/project)

Key fields consumed by forecast:
- `ID`, `Name`, `Schedule` — task identity and cron expression
- `Window`, `ForceWindow` — time window constraints
- `Timeout` — maximum duration (used as fallback if no history)
- `CostBudget` — budget limit (for display context)

### RunRecord (from internal/project)

Key fields consumed by forecast:
- `Started`, `Finished` — for duration calculation
- `InputTokens`, `OutputTokens`, `EstimatedCostUSD` — for cost projection
- `Success` — only successful runs used for averages

### Config (from internal/config)

Key fields consumed by forecast:
- `MaxWorkers` — worker pool size for contention detection
- `InputTokenRate`, `OutputTokenRate` — fallback cost rates
- `QuietHours` — scheduling constraints

## Relationships

```
Todo (1) ──── generates ────> (*) ForecastRun
  │                                │
  │                                ├──── aggregated into ──> ForecastSummary
  │                                │
  └── RunRecord (*) ── averaged ─> TaskStats ── informs ──> ForecastRun estimates

Config.MaxWorkers ── compared against ──> ContentionWindow.PeakConcurrent
```

## State Transitions

No state transitions. Forecast is a read-only projection — it generates ephemeral data structures for display only. Nothing is persisted.
