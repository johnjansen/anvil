# Research: Task Execution Timeout Extension

**Feature**: 005-timeout-extension
**Date**: 2026-02-27

## Decision 1: Timeout Extension Mechanism

**Decision**: Replace `context.WithTimeout` with `context.WithCancel` + external `time.AfterFunc` timer stored in `RunningTask`.

**Rationale**: Go's `context.WithTimeout` creates an immutable deadline — once set, the context's deadline cannot be extended. The current code at daemon.go:611 creates `ctx, cancel := context.WithTimeout(context.Background(), timeout)` and passes this to the runner. The external timer approach:
1. Uses `context.WithCancel` instead (no built-in deadline)
2. Creates `time.AfterFunc(timeout, func() { cancel() })` as the timeout enforcement
3. Stores the timer in `RunningTask`
4. To extend: `timer.Stop()` + create new `time.AfterFunc` with remaining+extension time
5. No process restart needed, minimal code change

**Alternatives considered**:
- Context replacement: Not viable — Go's `exec.CommandContext` binds the context at `Start()` time. Replacing the context would require killing and restarting the process.
- Cancel + restart with checkpoint resume: Too complex, loses in-flight work, and requires the task to support clean resume.
- Custom context wrapper: Over-engineered for this use case; the timer approach is simpler and widely used.

## Decision 2: Extension State Storage

**Decision**: Add extension tracking fields directly to the existing `RunningTask` struct and persist in `RunRecord` on completion.

**Rationale**: `RunningTask` (daemon.go:154-165) is the in-memory representation of a running task, protected by `d.tasksMu`. Adding fields here is zero-overhead and follows the existing pattern. Extension data is transient (only relevant while running) so in-memory storage is appropriate. On completion, the data is written to `RunRecord` for historical analysis.

**Alternatives considered**:
- Separate extension tracking map: Adds complexity without benefit. RunningTask already exists.
- File-based persistence during run: Unnecessary — if the daemon restarts, the task restarts with its original timeout anyway.

## Decision 3: CLI-to-Daemon Communication

**Decision**: Add `/extend-timeout` HTTP endpoint on the Unix domain socket, following the existing `/kill` handler pattern.

**Rationale**: The daemon already has a REST-like API over Unix socket (daemon.go:1208-1222) with handlers for `/ps`, `/kill`, `/run`, etc. Each endpoint accepts JSON, finds the task by key, and performs the action. The extend-timeout endpoint follows the same pattern: accept `{task_key, duration, absolute}`, find the running task, stop the old timer, create a new timer.

**Alternatives considered**:
- Signal-based communication: Not flexible enough (can't pass duration).
- File-based IPC: Overly complex for a simple request/response.

## Decision 4: Auto-Extend Trigger

**Decision**: Hook into the existing checkpoint callback in the daemon's runTask function (daemon.go:830-836) to detect recent progress and trigger auto-extension.

**Rationale**: The checkpoint mechanism already exists — tasks emit `##anvil:checkpoint <data>` on stdout, which is detected by `statusWriter.Write()` in runner.go:249. The callback updates `lastCheckpointData`. Adding auto-extend logic here means: when a checkpoint arrives and the task is within the warning window (default 5 minutes) of its deadline and has remaining auto-extensions, trigger an extension. This is the natural integration point.

**Alternatives considered**:
- Periodic polling for checkpoint activity: Adds unnecessary timer complexity when the callback is already in place.
- Output pattern matching beyond checkpoints: Scope creep; checkpoints are the established progress mechanism.

## Decision 5: Warning Hook Execution

**Decision**: Add a goroutine-based timer that fires the `on_timeout_warning` hook when the task enters the warning window (default 5 minutes before deadline). Re-schedule the warning timer after each extension.

**Rationale**: The warning needs to fire at a specific time (deadline minus warning window). A `time.AfterFunc` goroutine scheduled at task start (and rescheduled after extensions) is the simplest approach. The hook execution follows the existing `runHook` pattern (daemon.go:1089-1129).

**Alternatives considered**:
- Check in tick function: Tick runs every 10 seconds which is sufficient granularity, but adding per-task deadline checking to the tick function is less clean than a dedicated timer per task.

## Decision 6: PS Output Enhancement

**Decision**: Add `ExtensionCount`, `OriginalTimeout`, and `TotalExtended` fields to the existing `TaskInfo` struct.

**Rationale**: `TaskInfo` (daemon.go:171-183) already has `Timeout`, `TimeRemaining`, and `PercentUsed` fields. Adding extension metadata is a natural extension. The `handlePs` handler already computes timeout info from `RunningTask` — adding extension fields is straightforward.

**Alternatives considered**:
- Separate endpoint for extension info: Adds unnecessary API complexity.

## Decision 7: Frontmatter Config for Auto-Extend

**Decision**: Add `AutoExtendConfig` struct and `auto_extend` YAML field to frontmatter parsing, following the same pattern as `AllowedWindow` and `SLA`.

**Rationale**: Task configuration consistently uses YAML frontmatter with pointer-based optional structs in `fmData`. Auto-extend follows the same pattern: `AutoExtend *AutoExtendConfig \`yaml:"auto_extend"\`` with `Enabled`, `MaxExtensions`, `ExtensionDuration` fields.

**Alternatives considered**:
- Global config only: Per-task control is essential since different tasks have different timeout needs.
- CLI flags only: Doesn't support unattended operation.
