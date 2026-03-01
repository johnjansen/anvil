# Research: Task Alerting Rules

## Decisions Made

### Alert Storage Location
**Decision**: Store alerts in `.anvil/alerts/<task-id>/alerts.json`
**Rationale**: Follows existing pattern of `.anvil/runs/<task-id>/` for run records. Allows per-task alert history while keeping it separate from run records.
**Alternatives considered**:
- Storing in run records: Rejected because alerts persist across runs
- SQLite database: Rejected - adding a DB is unnecessary complexity

### Alert Evaluation Timing
**Decision**: Evaluate alerts at task run completion
**Rationale**: Cost and duration are only known after run completes. Output patterns need full output available.
**Alternatives considered**:
- Real-time during execution: Rejected - cost/duration unknown until complete

### Webhook Delivery
**Decision**: Async webhook delivery with retry support
**Rationale**: Webhooks are external calls that should not block task completion. Retry provides reliability.
**Alternatives considered**:
- Sync delivery: Rejected - blocks task completion, no value in failing webhook

## Technology Choices

### JSON Storage
**Choice**: JSON files for alert records
**Rationale**: Follows existing anvil pattern. Simple, no external dependencies.
**Alternatives considered**: SQLite - unnecessary complexity for this use case

### Condition Evaluation
**Choice**: Simple threshold comparison for cost/duration, regex for output
**Rationale**: Matches user-facing config format, simple to implement and test.
**Alternatives considered**: Expression language - overkill for these three condition types
