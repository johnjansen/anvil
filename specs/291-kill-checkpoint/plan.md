# Implementation Plan: Task Kill with Checkpoint

**Branch**: `291-kill-checkpoint` | **Date**: 2026-03-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/291-kill-checkpoint/spec.md`

## Summary

Add a `--checkpoint` flag to `anvil task kill` that sends SIGTERM instead of immediately cancelling context, giving checkpoint-enabled tasks time to save progress before exiting. The existing checkpoint system (capture via `##anvil:checkpoint`, storage in RunRecord, resume via `ANVIL_CHECKPOINT_DATA` env var) is fully leveraged. The daemon coordinates graceful shutdown via a new `GracefulStop` channel on `RunningTask`, and the `runTask` goroutine handles SIGTERM delivery, grace period timeout, and RunRecord status recording.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/daemon` (task execution, kill handling), `internal/project` (Todo, RunRecord, frontmatter parsing), `internal/runner` (checkpoint capture), `cmd/anvil` (CLI commands)
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system)
**Testing**: Go test (`go test ./...`)
**Target Platform**: macOS, Linux (Unix signals: SIGTERM, SIGKILL)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Grace period timeout must be accurate within 1 second
**Constraints**: Must not change behavior of existing `anvil task kill` (without `--checkpoint`)
**Scale/Scope**: Modifies 3 files, adds ~150-200 lines of Go code

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not yet configured for this project (template placeholders only). No gates to enforce. Proceeding with standard engineering practices.

## Project Structure

### Documentation (this feature)

```text
specs/291-kill-checkpoint/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── cli.md           # CLI contract
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/anvil/
└── task_lifecycle.go    # Kill command with --checkpoint flag

internal/daemon/
└── daemon.go            # KillRequest, RunningTask, handleKill, runTask changes

internal/project/
└── project.go           # Todo.CheckpointGracePeriod, frontmatter parsing
```

**Structure Decision**: All changes are in existing files. No new files or directories needed. The feature is a focused extension of the existing kill and checkpoint systems.

## Complexity Tracking

No constitution violations to justify.
