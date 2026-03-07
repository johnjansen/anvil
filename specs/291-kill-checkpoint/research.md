# Research: Task Kill with Checkpoint

## R1: Current Kill Mechanism

**Decision**: Extend the existing `handleKill` to support graceful shutdown with SIGTERM when `--checkpoint` flag is used.

**Rationale**: Currently `handleKill` only calls `Cancel()` (context cancellation), which doesn't give the task process a chance to save checkpoint data. The `killProcess` function already implements SIGTERM→wait→SIGKILL for timeout-based kills. We need to bring similar behavior to `handleKill` when `--checkpoint` is specified.

**Alternatives considered**:
- Reuse `killProcess` directly: Rejected because `killProcess` uses raw PID and doesn't integrate with the context/RunRecord flow. Better to add graceful shutdown logic directly in `handleKill`.
- Always use SIGTERM for all kills: Rejected because backwards-compatible behavior matters. Users expect `anvil task kill` to be immediate.

## R2: Checkpoint System Already Exists

**Decision**: Leverage the existing checkpoint system entirely. No new checkpoint mechanism needed.

**Rationale**: The codebase already has:
- `Checkpoint bool` field on Todo struct (frontmatter `checkpoint: true`)
- `CheckpointData string` field on RunRecord
- Runner captures `##anvil:checkpoint <data>` lines via `statusWriter`
- Daemon stores latest checkpoint in `lastCheckpointData` variable during execution
- On resume, `LatestCheckpointData()` reads the most recent RunRecord and injects `ANVIL_CHECKPOINT_DATA` env var
- History display shows checkpoint preview (80 chars)

The existing system captures checkpoint data throughout execution. On graceful kill, we just need to ensure the RunRecord is written with the current `lastCheckpointData` and a status indicating it was stopped with checkpoint.

## R3: Graceful Kill Flow Design

**Decision**: Add `Checkpoint bool` to `KillRequest`. When true, send SIGTERM to the child process, wait up to a grace period (default 30s), then SIGKILL if needed. The `runTask` function detects this via context and writes RunRecord with checkpoint data.

**Rationale**: The daemon already tracks `RunningTask` instances with `Cancel` function and `PID`. We need the child process PID (not the daemon PID). The child PID is stored in `runTask` scope but not in `RunningTask`. We need to either:
1. Add child PID to `RunningTask`, or
2. Add a graceful shutdown channel/flag to `RunningTask`

Option 2 is cleaner: add a `GracefulStop chan struct{}` to `RunningTask`. When `--checkpoint` kill is requested, close this channel. The `runTask` goroutine detects the channel close, sends SIGTERM to the child process (which it has access to), waits for graceful exit, then writes the RunRecord with checkpoint data and "stopped-with-checkpoint" status.

**Alternatives considered**:
- Storing child PID in RunningTask and sending SIGTERM from handleKill: Rejected because the child PID changes across retry attempts, and handleKill doesn't have access to the runner's process handle.

## R4: New RunRecord Status

**Decision**: Use `Error` field with value "stopped-with-checkpoint" and `Success: false` to indicate a checkpoint stop. No new field needed.

**Rationale**: The existing RunRecord already distinguishes outcomes via `Success` bool and `Error` string. Adding "stopped-with-checkpoint" as a recognized error string is consistent with how other statuses work (retry failures use Error field). The `CheckpointData` field already exists to store the data.

**Alternatives considered**:
- Adding a `StoppedWithCheckpoint bool` field: Rejected as unnecessary. The combination of `Success: false`, `Error: "stopped-with-checkpoint"`, and `CheckpointData != ""` is sufficient and avoids schema changes.

## R5: Grace Period Configuration

**Decision**: Default 30-second grace period, configurable per-task via `checkpoint_grace_period` frontmatter field.

**Rationale**: 30 seconds is standard for graceful shutdown in most systems. Some tasks may need more or less time. Per-task configuration is natural since checkpoint behavior is already per-task.

**Alternatives considered**:
- Global-only configuration: Rejected because tasks have very different shutdown characteristics.
- CLI flag for grace period: Could be added later, but per-task default covers the common case.
