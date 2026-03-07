# Implementation Plan: Task Output Validation with Assertions

**Branch**: `025-task-output-validation` | **Date**: 2026-03-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/025-task-output-validation/spec.md`

## Summary

Add output validation assertions for tasks. When a task completes, the system evaluates configured assertions against stdout/stderr and file outputs. If any hard assertion fails, the task is marked as failed and retry/failure hooks are triggered. Soft assertions log warnings without failing the task. Supports string matching, regex patterns, JSON validation, file existence/content checks, and clear error messaging.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/project` (Todo/RunRecord), `internal/config`, `internal/daemon`, `internal/runner`
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system)
**Testing**: `go test ./...` with table-driven tests
**Target Platform**: macOS, Linux (CLI + daemon)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Assertion evaluation adds minimal overhead to task completion
**Constraints**: Zero breaking changes to existing tasks; assertions only apply when configured
**Scale/Scope**: Per-project task management (typically 1-50 tasks per project)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

No project-specific constitution configured. No gates to evaluate. Proceeding with standard practices:
- Tests included for all new logic
- Backward compatible (no changes to tasks without assertion config)
- Follows existing patterns (hooks, run records, task configuration)

**Post-Phase 1 re-check**: Design follows established patterns. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/025-task-output-validation/
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
│   └── project.go       # Add AssertionConfig struct, Assert fields to Todo
├── config/
│   └── config.go        # Add AssertionConfig to Config if needed
├── runner/
│   ├── runner.go        # Add assertion evaluation after task execution
│   ├── assertion.go     # NEW: Assertion evaluation helpers (evaluateAssertions, checkStdout, checkFiles, etc.)
│   └── assertion_test.go # NEW: Tests for assertion logic
└── daemon/
    └── daemon.go        # Integrate assertion evaluation with task execution

cmd/
└── anvil/
    └── main.go          # Potentially add assertion info to task commands if needed
```

**Structure Decision**: Existing Go project structure. New assertion logic isolated in `internal/runner/assertion.go` following the pattern of other helper modules. All other changes are additions to existing files.