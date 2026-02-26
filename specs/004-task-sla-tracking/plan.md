# Implementation Plan: Task SLA Tracking

**Branch**: `004-task-sla-tracking` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-task-sla-tracking/spec.md`

## Summary

Add SLA (Service Level Agreement) tracking for scheduled tasks. When a task runs later than its configured `max_delay` threshold, the system records an SLA violation and optionally fires a hook command. Supports per-task configuration via frontmatter, global defaults via config, strict mode (skip late tasks), and CLI commands for viewing/resetting violations. Follows the same patterns established by the time windows feature (spec 003).

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/cron` (cron parsing with `Prev()`), `internal/project` (Todo/RunRecord), `internal/config`, `internal/daemon`
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system)
**Testing**: `go test ./...` with table-driven tests
**Target Platform**: macOS, Linux (CLI + daemon)
**Project Type**: CLI tool with background daemon
**Performance Goals**: SLA check adds negligible overhead to dispatch loop (single `Prev()` call + comparison)
**Constraints**: Zero breaking changes to existing tasks; SLA only applies to cron-scheduled tasks
**Scale/Scope**: Per-project task management (typically 1-50 tasks per project)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

No project-specific constitution configured. No gates to evaluate. Proceeding with standard practices:
- Tests included for all new logic
- Backward compatible (no changes to tasks without SLA config)
- Follows existing patterns (time windows, hooks, run records)

**Post-Phase 1 re-check**: Design follows established patterns. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/004-task-sla-tracking/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Research decisions
├── data-model.md        # Entity definitions
├── quickstart.md        # User-facing quickstart
├── contracts/
│   └── task-frontmatter.md  # Frontmatter contract
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Task breakdown (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── project/
│   └── project.go       # Add SLAConfig struct, SLA/OnSLAViolation fields to Todo, SLA fields to RunRecord
├── config/
│   └── config.go        # Add SLAGlobalConfig struct and SLA field to Config
├── daemon/
│   ├── daemon.go        # Add SLA check in dispatch loop, SLA violation hook execution
│   ├── sla.go           # NEW: SLA evaluation helpers (checkSLA, parseSLAConfig, etc.)
│   └── sla_test.go      # NEW: Tests for SLA logic
└── cron/
    └── parser.go        # Existing Prev() function (no changes needed)

cmd/
└── anvil/
    └── main.go          # Add SLA info to task get, add task sla command
```

**Structure Decision**: Existing Go project structure. New SLA logic isolated in `internal/daemon/sla.go` following the pattern of `internal/daemon/timewindow.go`. All other changes are additions to existing files.
