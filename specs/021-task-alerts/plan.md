# Implementation Plan: Task Alerting Rules

**Branch**: `021-task-alerts` | **Date**: 2026-03-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/021-task-alerts/spec.md`

## Summary

Add task alerting rules for custom notifications. Tasks can define alert conditions (cost > threshold, duration > threshold, output matches pattern) that trigger alerts with configurable actions (webhook, notify, retry). Users can view active alerts, acknowledge them, and see alert history via CLI commands.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/cron`, `internal/project` (Todo/RunRecord), `internal/config`, `internal/daemon`
**Storage**: JSON files in `.anvil/alerts/` (new directory for alert records)
**Testing**: `go test ./...` with table-driven tests
**Target Platform**: macOS, Linux (CLI + daemon)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Alert evaluation adds negligible overhead to run completion; webhook calls are async
**Constraints**: Zero breaking changes to existing tasks; alerts only apply to tasks with alert config
**Scale/Scope**: Per-project task management (typically 1-50 tasks per project)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

No project-specific constitution configured. No gates to evaluate. Proceeding with standard practices:
- Tests included for all new logic
- Backward compatible (no changes to tasks without alert config)
- Follows existing patterns (SLA tracking, hooks, run records)

**Post-Phase 1 re-check**: Design follows established patterns. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/021-task-alerts/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Research decisions
├── data-model.md        # Entity definitions
├── quickstart.md        # User-facing quickstart
├── contracts/
│   └── task-frontmatter.md  # Frontmatter contract
└── tasks.md             # Task breakdown (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── project/
│   └── project.go       # Add AlertConfig struct, Alerts field to Todo, Alert-related fields to RunRecord
├── config/
│   └── config.go        # Add AlertGlobalConfig struct and Alerts field to Config
├── daemon/
│   ├── daemon.go        # Add alert evaluation in run completion handler
│   ├── alerts.go        # NEW: Alert evaluation and action execution
│   └── alerts_test.go   # NEW: Tests for alert logic
└── cron/
    └── parser.go        # Existing (no changes needed)

cmd/
└── anvil/
    └── main.go          # Add alerts command with subcommands: list, ack, history

.alerts/                 # NEW: Alert storage directory
└── <task-id>/
    └── alerts.json      # Alert records for each task
```

**Structure Decision**: Existing Go project structure. New alert logic isolated in `internal/daemon/alerts.go` following the pattern of `internal/daemon/sla.go`. Alert storage in `.alerts/` directory per task, similar to `.anvil/runs/` for run records.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
