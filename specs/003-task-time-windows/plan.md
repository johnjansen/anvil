# Implementation Plan: Task Execution Time Windows

**Branch**: `003-task-time-windows` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/003-task-time-windows/spec.md`

## Summary

Add time window constraints to anvil task scheduling so users can restrict when tasks execute (e.g., business hours only, no weekends) and configure global quiet hours. Time windows are evaluated at dispatch time after cron matching. Includes `--force` flag for manual bypass and `anvil task next` command for visibility.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `gopkg.in/yaml.v3`, standard library (`time`, `fmt`, `strconv`, `strings`)
**Storage**: File-based (YAML frontmatter in task files, YAML config file)
**Testing**: Standard Go `testing` package
**Target Platform**: macOS, Linux (cross-platform CLI)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Window evaluation adds < 1ms overhead per tick cycle
**Constraints**: Zero breaking changes to existing task configurations
**Scale/Scope**: Evaluates windows for all due tasks per tick (typically < 100 tasks)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not configured for this project (template placeholder). No gates to check.

## Project Structure

### Documentation (this feature)

```text
specs/003-task-time-windows/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/anvil/main.go               # CLI commands: taskRunCmd (--force), taskNextCmd (new)
internal/project/project.go     # Todo struct: AllowedWindow field, frontmatter parsing
internal/config/config.go       # Config struct: QuietHoursConfig
internal/daemon/daemon.go       # Dispatch logic: window evaluation in tick()
internal/daemon/daemon.go       # handleRun: force-bypass support
internal/daemon/timewindow.go   # NEW: window evaluation helpers
internal/daemon/timewindow_test.go  # NEW: window evaluation tests
```

**Structure Decision**: All changes fit within existing package structure. One new file (`timewindow.go`) in the daemon package to keep window evaluation logic isolated and testable. No new packages needed.

## Complexity Tracking

No constitution violations to justify.
