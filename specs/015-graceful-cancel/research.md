# Research: Graceful Cancel with Partial Result Capture

**Date**: 2026-02-27
**Feature**: 015-graceful-cancel

## Decision 1: Graceful Kill Mechanism

**Decision**: Send SIGTERM first, then SIGKILL after grace period. Use `cmd.Process.Signal(syscall.SIGTERM)` instead of context cancellation.

**Rationale**: The current kill uses `Cancel()` (context cancellation) which propagates through the runner but doesn't give the task process itself a chance to handle shutdown. Sending SIGTERM to the child process allows it to trap the signal and save state. The grace period timer then sends SIGKILL if the process hasn't exited.

**Alternatives considered**:
- Context cancellation with timeout: Rejected -- doesn't signal the child process directly
- Custom IPC channel: Rejected -- over-engineered, SIGTERM is the Unix standard

## Decision 2: Partial Result Protocol

**Decision**: Add `##anvil:partial` as a new magic prefix in `statusWriter` (runner.go), following the existing `##anvil:status` and `##anvil:checkpoint` pattern.

**Rationale**: The `statusWriter` (runner.go:310-360) already scans task stdout line-by-line and handles `##anvil:status` and `##anvil:checkpoint` prefixes with callbacks. Adding `##anvil:partial` follows the exact same pattern with minimal code change.

**Alternatives considered**:
- Separate file-based protocol: Rejected -- adds filesystem coupling, stdout protocol is simpler
- Reuse checkpoint prefix: Rejected -- checkpoints serve a different purpose (session resume), partial results are user-facing progress data

## Decision 3: Partial Result Storage

**Decision**: Add `PartialResults` string field to RunRecord struct, alongside existing `CheckpointData`.

**Rationale**: RunRecord (project.go:140-166) already has `CheckpointData` for session checkpoints. Partial results serve a different purpose (user-facing progress) and need a separate field. Only the most recent partial result is stored (same as checkpoint behavior).

**Alternatives considered**:
- Separate file storage: Rejected -- RunRecord JSON is the natural home for per-run metadata
- Array of all partials: Rejected -- only latest matters for resume, avoids unbounded growth

## Decision 4: On-Kill Hook

**Decision**: Add `on_kill` frontmatter field, parsed alongside existing `on_success`/`on_failure`. Execute via existing `runHook()` function.

**Rationale**: `runHook()` (daemon.go:1115) already handles hook execution with environment variables. Adding `on_kill` follows the exact same pattern as `on_success` and `on_failure`.

**Alternatives considered**:
- Inline hook in kill handler: Rejected -- runHook already provides env vars, timeout handling
- Pre-kill signal protocol: Rejected -- on_kill hook is simpler and follows existing patterns

## Decision 5: Kill Request Enhancement

**Decision**: Extend `KillRequest` struct with `Graceful bool` field. The `handleKill` handler checks this flag.

**Rationale**: The existing `KillRequest{ID string}` is simple. Adding a boolean field is backward-compatible -- old clients that don't send `graceful` get the default `false` (force kill), preserving current behavior.

**Alternatives considered**:
- Separate endpoint (/graceful-kill): Rejected -- bloats API, single endpoint with flag is cleaner

## Decision 6: Resume Mechanism

**Decision**: Add `--resume` flag to `taskRunCmd()`. The daemon reads the latest RunRecord's `PartialResults` and passes it as `ANVIL_PARTIAL_RESULTS` environment variable.

**Rationale**: `taskRunCmd()` already sends a run request to the daemon with the task ID. The daemon can look up the latest run record and inject the partial results into the task's environment.

**Alternatives considered**:
- Client-side resume: Rejected -- daemon owns run records, should be source of truth
- File-based resume: Rejected -- env var is simpler and follows existing patterns

## Decision 7: Termination Method Tracking

**Decision**: Add `TerminationMethod` string field to RunRecord (values: `"normal"`, `"graceful"`, `"force"`, `"timeout"`).

**Rationale**: Operators need to distinguish how tasks ended. This is a simple string field that integrates with existing RunRecord serialization.

**Alternatives considered**:
- Boolean flags: Rejected -- multiple booleans are harder to reason about than an enum-like string
