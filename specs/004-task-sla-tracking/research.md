# Research: Task SLA Tracking

**Feature**: 004-task-sla-tracking
**Date**: 2026-02-27

## Decision 1: SLA Delay Calculation Method

**Decision**: Use `cron.Parser.Prev(time.Now())` to get the most recent scheduled time, then calculate delay as `actualDispatchTime - scheduledTime`.

**Rationale**: The cron package already has a `Prev()` function (parser.go:189) that finds the previous matching time. This gives the exact time the task was supposed to run. The delay is simply the difference between when the task is actually dispatched and when it should have been.

**Alternatives considered**:
- Track scheduled time in tick function: Adds state management complexity; `Prev()` already computes this correctly.
- Use `thisMinute` from tick: This is the minute the cron matched, not necessarily when the task was supposed to run (could be delayed by queue backlog).

## Decision 2: SLA Violation Record Storage

**Decision**: Add SLA fields directly to the existing `RunRecord` struct and store violations alongside run records.

**Rationale**: The existing run record system (`.anvil/runs/<task-id>/<run-id>.json`) already persists per-run data with `WriteRunRecord`, `ReadCurrentRunRecord`, and `ReadAllRunRecords`. Adding `ScheduledTime`, `DispatchDelay`, and `SLAViolation` fields to `RunRecord` keeps data co-located and avoids a new storage subsystem. Backward compatible — new fields use `omitempty` so existing records are unaffected.

**Alternatives considered**:
- Separate `.anvil/sla/` directory: Adds a parallel storage system. More files to manage, harder to correlate with runs. No meaningful benefit.
- In-memory only: Doesn't survive daemon restarts (violates FR-011).

## Decision 3: SLA Check Placement in Dispatch Loop

**Decision**: Check SLA after cron match and before task queuing. Record the scheduled time and delay at dispatch time.

**Rationale**: At dispatch time (daemon.go ~line 2069), the cron schedule has already matched. The SLA check compares current time against the scheduled time. If `strict: true` and delay exceeds max, skip the task. If `strict: false`, record the violation and proceed. This follows the same skip-pattern used by time windows (lines 2156-2169) and quiet hours (lines 2170-2181).

**Alternatives considered**:
- Check at worker execution time: The delay between queue and execution is not the user's concern — they care about scheduled vs actual start.
- Check in tick function before queuing: Same place, but the actual dispatch time in the worker gives a more accurate delay measurement. However, checking at dispatch keeps the pattern consistent with window checks.

## Decision 4: Hook Execution Model

**Decision**: Execute `on_sla_violation` as a shell command asynchronously (goroutine), with environment variables providing task and delay information. Follow the existing `runHook` pattern.

**Rationale**: The existing hook system (daemon.go:1089-1129) runs shell commands with `sh -c` and a 60-second timeout, setting environment variables like `ANVIL_TASK_NAME`, `ANVIL_PROJECT`, etc. The SLA hook should follow this exact pattern with additional SLA-specific variables.

**Alternatives considered**:
- Template variable replacement (`{{ .TaskName }}`): The spec suggests this, but environment variables are more consistent with the existing hook system and more flexible for shell scripting.
- Blocking execution: Would delay task dispatch. Async (goroutine) is consistent with existing hooks.

## Decision 5: Duration Parsing for max_delay

**Decision**: Use Go's `time.ParseDuration()` for the `max_delay` field, supporting values like "15m", "1h30m", "30s".

**Rationale**: This is the same approach used for `timeout`, `retry_delay`, `persistent_cooldown`, etc. in the existing frontmatter parsing (project.go:268-283). Consistent with the codebase.

**Alternatives considered**:
- Custom duration parser: Unnecessary complexity when Go's stdlib handles it.
- Minutes-only integer: Less flexible, inconsistent with other duration fields.

## Decision 6: Global SLA Config Structure

**Decision**: Add `SLA SLAGlobalConfig` to the `Config` struct with a single `DefaultMaxDelay` field (duration string).

**Rationale**: Follows the same pattern as `QuietHours QuietHoursConfig` (config.go:33-38). The global config provides a default `max_delay` for tasks without per-task SLA. `strict` mode is intentionally per-task only (per spec assumptions).

**Alternatives considered**:
- Global `on_sla_violation` hook: Could be useful but adds complexity. Per-task hooks provide enough flexibility. Can be added later.

## Decision 7: SLA Dashboard Data Source

**Decision**: `anvil task sla` iterates all projects and todos, reads run records, and filters for SLA violations. No separate violation index needed.

**Rationale**: Run records already contain all the data needed (with the new SLA fields). The number of run records per task is bounded by retention config. Reading all records for a task is already implemented (`ReadAllRunRecords`).

**Alternatives considered**:
- Maintain a separate violation index file: Extra write overhead on every dispatch, extra file to manage. Not worth it for a CLI dashboard command.
