# Implementation Plan: Cross-Project Dependency Status in Task Queue

**Branch**: `265-cross-project-queue` | **Date**: 2026-03-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/265-cross-project-queue/spec.md`

## Summary

Extend the `anvil task queue` command to display cross-project dependency status for tasks that depend on tasks in other watched projects. The daemon's `/queue` endpoint will resolve cross-project dependencies using existing `ResolveDependencyRunRecord` infrastructure from #259, and the CLI will render this information in both table and JSON output formats. A new `--all` flag will include cross-project dependency entries as distinct queue items.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/project` (Todo, Dependency, RunRecord, ParseDependency, ResolveDependencyRunRecord), `internal/daemon` (TaskQueueInfo, handleQueue), `cmd/anvil` (taskQueueCmd)
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system), watched project paths via `~/.anvil/watched/`
**Testing**: `go test ./...`
**Target Platform**: macOS/Linux CLI
**Project Type**: CLI tool
**Performance Goals**: Queue command should complete in under 2 seconds even with cross-project lookups
**Constraints**: Must not require remote project's daemon to be running; resolve from disk-based run records
**Scale/Scope**: Typically <50 tasks per project, <10 watched projects

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not customized (template defaults). No gates to enforce. Proceeding.

## Project Structure

### Documentation (this feature)

```text
specs/265-cross-project-queue/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── cli-output.md    # CLI output format contracts
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/anvil/
└── task_queue.go         # Extend taskQueueCmd with --all flag and cross-project dep display

internal/daemon/
└── daemon.go             # Extend TaskQueueInfo struct and handleQueue to include cross-project deps

internal/project/
└── dependencies.go       # Already has ParseDependency, ResolveDependencyRunRecord (from #259)
```

**Structure Decision**: This feature extends existing files only. No new packages or files needed. The cross-project dependency resolution infrastructure is already in place from #259.

## Complexity Tracking

No violations. Feature is a straightforward extension of existing queue display with existing dependency resolution logic.
