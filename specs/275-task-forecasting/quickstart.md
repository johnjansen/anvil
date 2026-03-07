# Quickstart: Task Forecasting

## View your upcoming task schedule

```bash
# See what runs in the next 7 days
anvil task forecast

# See the next 14 days
anvil task forecast --days 14

# Focus on a specific task
anvil task forecast --task fetch-data
```

## Check for resource contention

```bash
# Find time windows where tasks will compete for workers
anvil task forecast --contention
```

If contention is detected, consider:
- Staggering task schedules to spread the load
- Increasing `max_workers` in `~/.anvil/config.yaml`

## Estimate costs

```bash
# See projected token usage and costs
anvil task forecast --cost
```

Cost estimates are based on your historical run data. Tasks that haven't run yet will show "no data".

## Test before adding a new task

```bash
# See what happens if you add a task — without actually adding it
anvil add -s "0 9 * * *" --dry-run "daily-report"
```

This shows the forecast with the new task included, highlighting any new contention it would cause.

## Get machine-readable output

```bash
# JSON output for scripting
anvil task forecast --json
anvil task forecast --cost --contention --json
```
