# Implementation Plan: Task Wait Conditions for Multi-Criteria Triggering

**Branch**: `340-task-wait-conditions` | **Date**: 2026-03-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/340-task-wait-conditions/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

This feature adds sophisticated task triggering capabilities to the Anvil task scheduler, allowing users to define complex multi-criteria conditions for when tasks should execute. The implementation will extend the existing task configuration system to support AND/OR logic for trigger conditions, file existence checks, environment variable checks, and polling-based triggers with timeout functionality.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/cron, internal/project, internal/config, internal/daemon
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system)
**Testing**: Go testing package with table-driven tests
**Target Platform**: Cross-platform CLI tool (Linux, macOS, Windows)
**Project Type**: CLI tool with daemon/background services
**Performance Goals**: Task evaluation should complete within 100ms under normal conditions
**Constraints**: Must maintain backward compatibility with existing task configurations
**Scale/Scope**: Designed to handle hundreds of concurrent tasks with minimal resource overhead

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Based on the project constitution principles, this feature must adhere to:

1. **Library-First**: Implementation should be modular and testable as standalone components
2. **CLI Interface**: New functionality must be exposed via CLI commands
3. **Test-First**: All new code must be developed with comprehensive tests
4. **Integration Testing**: Changes to task triggering logic require integration tests
5. **Observability**: Task evaluation and execution must be logged appropriately

No violations expected - this is an extension of existing functionality rather than a new architectural approach.

## Project Structure

### Documentation (this feature)

```text
specs/340-task-wait-conditions/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/
├── project/
│   ├── trigger.go       # New trigger condition logic
│   ├── task_config.go   # Extended task configuration
│   └── condition_eval.go # Condition evaluation logic
├── daemon/
│   ├── task_scheduler.go # Updated task scheduling with condition checking
│   └── polling_manager.go # New polling-based trigger manager
cmd/
└── anvil/
    └── task_trigger_check.go # New CLI command for manual trigger evaluation

tests/
├── unit/
│   ├── trigger_test.go
│   ├── condition_eval_test.go
│   └── polling_manager_test.go
└── integration/
    └── multi_criteria_trigger_test.go
```

**Structure Decision**: This feature extends the existing task triggering system by adding new condition evaluation capabilities. The implementation will primarily reside in the `internal/project/` package with supporting logic in `internal/daemon/` and a new CLI command in `cmd/anvil/`.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
