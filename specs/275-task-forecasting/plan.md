# Implementation Plan: Task Forecasting

**Branch**: `275-task-forecasting` | **Date**: 2026-03-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/275-task-forecasting/spec.md`

## Summary

Add an `anvil task forecast` command that projects future task executions based on cron schedules, identifies resource contention windows where concurrent tasks exceed worker pool capacity, estimates costs from historical run data, and supports what-if analysis via `anvil add --dry-run`. The implementation builds on existing `internal/cron` (Next/Prev), `internal/project` (Todo/RunRecord with cost fields), and `internal/daemon` (NextAllowedRun with time window support).

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/cron, internal/project, internal/config, internal/daemon
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system)
**Testing**: Go testing package with table-driven tests
**Target Platform**: Cross-platform CLI (Linux, macOS)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Forecast calculation completes in under 2 seconds for 100 tasks over 7 days
**Constraints**: Backward compatible; no changes to existing RunRecord or Todo formats beyond what already exists
**Scale/Scope**: Handle projects with up to 100 tasks, forecast horizons up to 30 days

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not yet configured for this project (template placeholders only). No gates to evaluate. Proceeding with standard project conventions observed from existing features:
- CLI commands follow flag.NewFlagSet pattern in `cmd/anvil/`
- Core logic lives in `internal/` packages
- Table-driven tests in `*_test.go` files
- JSON and human-readable output formats supported

## Project Structure

### Documentation (this feature)

```text
specs/275-task-forecasting/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── cli-contract.md  # CLI interface contract
└── checklists/
    └── requirements.md  # Specification quality checklist
```

### Source Code (repository root)

```text
cmd/anvil/
├── task_router.go       # MODIFY: add "forecast" case
├── task_forecast.go     # NEW: CLI command for `anvil task forecast`
└── add.go               # MODIFY: add --dry-run flag

internal/
├── forecast/
│   ├── forecast.go      # NEW: Core forecast engine
│   ├── contention.go    # NEW: Contention detection logic
│   ├── cost.go          # NEW: Cost projection logic
│   ├── forecast_test.go # NEW: Tests for forecast engine
│   ├── contention_test.go # NEW: Tests for contention
│   └── cost_test.go     # NEW: Tests for cost projection
├── project/
│   └── project.go       # EXISTING: Uses Todo, RunRecord, ReadAllRunRecords (no changes needed)
├── config/
│   └── config.go        # EXISTING: Uses MaxWorkers, InputTokenRate, OutputTokenRate (no changes needed)
├── daemon/
│   └── timewindow.go    # EXISTING: Uses NextAllowedRun (no changes needed)
└── cron/
    └── parser.go        # EXISTING: Uses Parse().Next() (no changes needed)
```

**Structure Decision**: New `internal/forecast` package keeps forecast logic isolated from daemon runtime. The forecast engine is a read-only projection that doesn't modify state, so it belongs in its own package rather than polluting the daemon package. CLI command in `cmd/anvil/task_forecast.go` follows the established pattern of one file per subcommand.

## Complexity Tracking

No constitution violations to justify.
