# Research: Task Forecasting

**Feature**: 275-task-forecasting
**Date**: 2026-03-07

## R1: Cron Schedule Projection Strategy

**Decision**: Use iterative `cron.Parse().Next()` calls to project future execution times, combined with `daemon.NextAllowedRun()` to account for time windows and quiet hours.

**Rationale**: The existing cron package already supports `Next(after time.Time)` which searches up to 5 years ahead. By iterating from `now` through the forecast horizon, we get accurate projections that respect all scheduling constraints. `NextAllowedRun()` in `internal/daemon/timewindow.go` already handles time windows, quiet hours, and priority-based scheduling — no need to reimplement.

**Alternatives considered**:
- `CountMissed()` with time range: Only counts, doesn't give individual times. Insufficient for contention analysis.
- Mathematical projection from cron fields: Complex to implement correctly for all cron patterns (ranges, lists, day-of-week). The iterative approach is simpler and already proven.

## R2: Contention Detection Approach

**Decision**: Build a timeline of task execution intervals (start time + estimated duration) and scan for overlapping intervals that exceed worker count.

**Rationale**: Simple interval overlap detection is O(n log n) with sorting and handles partial overlaps correctly. Worker count is available from `config.MaxWorkers`. Duration estimates come from historical RunRecord averages (Finished - Started).

**Alternatives considered**:
- Time-slot bucketing (group by minute/hour): Loses precision for tasks that span multiple slots. Interval overlap is more accurate.
- Event-driven simulation: Overkill for a read-only projection. A sorted interval scan is sufficient.

## R3: Cost Estimation Data Source

**Decision**: Use `RunRecord.EstimatedCostUSD`, `RunRecord.InputTokens`, and `RunRecord.OutputTokens` from historical runs, averaging the last 10 runs per task. Fall back to config-level token rates (`InputTokenRate`, `OutputTokenRate`) for tasks without cost data but with token counts.

**Rationale**: `RunRecord` already tracks all three fields. Using per-task historical averages gives the most accurate projections for each task's actual behavior. The config rates (default $3/1M input, $15/1M output) serve as fallback for tasks that track tokens but not cost.

**Alternatives considered**:
- Use only the most recent run: Too volatile — single outlier skews projection.
- Use median instead of mean: Mean is simpler and sufficient when averaged over 10 runs. Variance indicator addresses outlier concern.

## R4: Output Summarization for High-Frequency Tasks

**Decision**: When a single task has more than 50 projected runs in the forecast period, group its runs by day and show a daily count + aggregate duration/cost instead of individual lines.

**Rationale**: A task running every minute generates 10,080 runs in 7 days. Listing each individually is unusable. The 50-run threshold balances detail vs readability. Individual runs are still available via `--task` filter + `--verbose`.

**Alternatives considered**:
- Always group by day: Loses detail for infrequent tasks where individual times matter.
- Pagination: CLI tools should produce complete output; pagination adds complexity.

## R5: Dry-Run Implementation on `anvil add`

**Decision**: The `--dry-run` flag on `anvil add` creates a temporary in-memory Todo from the provided arguments, injects it into the forecast engine alongside real tasks, and displays the combined forecast. No file is written.

**Rationale**: The forecast engine accepts a `[]project.Todo` slice. Constructing a Todo from CLI args and appending it to the loaded list is trivial. The existing `anvil add` already parses schedule, name, and other flags — `--dry-run` simply skips the file write step and calls the forecast engine instead.

**Alternatives considered**:
- Write a temporary file and delete it: Introduces risk of orphaned files on crash. In-memory is cleaner.
- Separate `anvil forecast --add` command: Splitting the workflow between two commands is less intuitive than extending `anvil add`.

## R6: Worker Pool Size Source

**Decision**: Read `MaxWorkers` from the global config (`~/.anvil/config.yaml`) via `config.Load()`. Default is 1 if not configured.

**Rationale**: This is the same value the daemon uses to size its work queue. No per-project override exists today.

**Alternatives considered**:
- Query running daemon for actual pool size: Requires daemon to be running. Forecast should work offline.
- Add per-project worker config: Out of scope — would require config format changes.
