# Data Model: Task Execution Sandbox

## Entities

### SandboxResult

Represents the outcome of a single sandbox execution. This is a transient in-memory struct — never persisted to disk.

| Field | Type | Description |
|-------|------|-------------|
| Label | string | Variation label (filename or "default") |
| Response | string | Raw LLM response text |
| InputTokens | int | Number of input tokens consumed |
| OutputTokens | int | Number of output tokens consumed |
| EstimatedCost | float64 | Estimated cost in USD |
| Duration | time.Duration | Wall-clock execution time |
| Error | string | Error message if execution failed |
| RunnerIndex | int | Which runner in the chain was used |

### No Persistent Entities

This feature creates no new persistent data. It explicitly avoids:
- Run records (`.anvil/runs/`)
- Session files
- Log files (temp logs are cleaned up)
- State files

## Relationships

- SandboxResult is produced by calling `runner.Run()` with the task's content
- `--compare` mode produces multiple SandboxResult instances (one per variation)
- Token counts and cost are derived from runner stderr using `runner.ParseTokenUsage()`
- Cost rates come from `config.Config.InputTokenRate` / `OutputTokenRate`
