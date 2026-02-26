# Implementation Plan: Task Execution Timeout Extension

**Branch**: `005-timeout-extension` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/005-timeout-extension/spec.md`

## Summary

Replace the immutable `context.WithTimeout` mechanism with `context.WithCancel` + external `time.AfterFunc` timer to enable runtime timeout extension. Add CLI command `anvil task extend-timeout`, auto-extend via checkpoint detection, `on_timeout_warning` hook, enhanced timeout visibility in ps/get/timeout commands, and persist extension data in RunRecord.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/daemon` (RunningTask, workItem, handlers), `internal/project` (Todo, RunRecord), `internal/config`, `internal/runner` (checkpoint callback)
**Storage**: In-memory for runtime state (RunningTask), JSON files for persistence (RunRecord)
**Testing**: `go test ./...` with table-driven tests
**Target Platform**: macOS, Linux (CLI + daemon)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Timeout extension takes effect within 2 seconds of command
**Constraints**: Go contexts are immutable once created — must use external timer pattern
**Scale/Scope**: Per-project task management (typically 1-50 tasks per project)

## Constitution Check

No project-specific constitution configured. No gates to evaluate. Standard practices:
- Tests for all new logic
- Backward compatible (no changes to tasks without extension config)
- Follows existing daemon patterns

**Post-Phase 1 re-check**: Design follows established patterns. No violations.

## Project Structure

### Documentation

```text
specs/005-timeout-extension/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Research decisions
├── data-model.md        # Entity definitions
├── quickstart.md        # User quickstart
├── contracts/
│   └── cli-commands.md  # CLI and API contracts
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Task breakdown (created by /speckit.tasks)
```

### Source Code

```text
internal/
├── daemon/
│   ├── daemon.go        # Modify: RunningTask struct, timeout creation, /extend-timeout handler, handlePs, checkpoint callback
│   ├── timeout.go       # NEW: timeout extension helpers, auto-extend logic, warning timer
│   └── timeout_test.go  # NEW: tests for extension logic
├── project/
│   └── project.go       # Add AutoExtendConfig struct, auto_extend/on_timeout_warning frontmatter, RunRecord extension fields
└── config/
    └── config.go        # (no changes needed — timeout is already per-task)

cmd/
└── anvil/
    └── main.go          # Add extend-timeout command, enhance taskTimeoutCmd, enhance taskGetCmd, enhance ps output
```

**Structure Decision**: Existing Go project structure. New timeout extension logic isolated in `internal/daemon/timeout.go` following the pattern of `timewindow.go` and `sla.go`. All other changes are additions to existing files.
