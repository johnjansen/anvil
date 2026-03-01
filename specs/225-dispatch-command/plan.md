# Implementation Plan: Add `anvil dispatch` Command

**Branch**: `225-dispatch-command` | **Date**: 2026-03-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/225-dispatch-command/spec.md`

**Note**: This template is filled in byit.plan` command the `/speck. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a new `anvil dispatch` command that creates a one-shot task, waits for completion, and returns the output_summary to stdout. This enables LLM-to-LLM delegation without race conditions. The command reuses existing internals: `AddTodo()` for task creation, `taskWaitCmd()` polling logic for completion detection, and `ReadCurrentRunRecord()` for fetching results.

## Technical Context

**Language/Version**: Go 1.24+
**Primary Dependencies**: `internal/project`, `internal/daemon`, existing CLI infrastructure
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system)
**Testing**: Go testing framework (`go test`)
**Target Platform**: macOS, Linux (CLI tool)
**Project Type**: CLI tool (Go)
**Performance Goals**: Sub-100ms overhead for dispatch operation
**Constraints**: Must integrate with existing daemon for task execution
**Scale/Scope**: Single command addition, ~200-400 lines expected

## Constitution Check

*This feature is a straightforward CLI command addition. No constitutional gates apply.*

## Phase 0: Research

No additional research needed. The feature specification clearly identifies:
- Task creation: reuse `AddTodo()` from `project.go`
- Task waiting: reuse polling logic from `taskWaitCmd()` in `main.go`
- Result retrieval: reuse `ReadCurrentRunRecord()` from `project.go`
- All required fields in `RunRecord` struct are already defined

## Phase 1: Design

### Key Technical Decisions

1. **Command structure**: New top-level `dispatch` command (like `anvil dispatch`)
2. **Task creation**: Call `AddTodo()` with empty schedule (one-shot), capture returned path and internally generated UUID
3. **Wait mechanism**: Reuse `taskWaitCmd()` polling logic with 2-second interval
4. **Result retrieval**: Use `ReadCurrentRunRecord()` after task completes
5. **Output**: Print `output_summary` to stdout, progress to stderr

### Implementation Locations

| Component | File | Role |
|-----------|------|------|
| `dispatchCmd()` | `main.go` | New command entry point |
| `AddTodo()` | `project.go:629` | Creates task, generates UUID |
| `ReadCurrentRunRecord()` | `project.go:820` | Fetches run record |
| Polling logic | `main.go:6466-6501` | Wait for completion |

### Data Model

The `RunRecord` struct already contains all needed fields:
- `TaskID`: Unique task identifier
- `RunID`: Execution run identifier
- `Success`: Boolean success status
- `OutputSummary`: Output text for return
- `Error`: Error message if failed

No new data structures required.

## Project Structure

### Documentation (this feature)

```
specs/225-dispatch-command/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # (not needed - implementation details clear)
├── data-model.md        # (not needed - uses existing RunRecord)
├── quickstart.md        # User guide for dispatch command
├── contracts/           # (not needed - CLI command, no external API)
└── tasks.md             # Task breakdown (via /speckit.tasks)
```

### Source Code (repository root)

```
cmd/anvil/
├── main.go              # Add dispatchCmd() here
└── [existing files unchanged]

internal/project/
├── project.go           # Add method to return UUID from AddTodo
└── [existing files unchanged]

tests/
└── [existing test structure]
```

**Structure Decision**: Single Go CLI command addition. No new packages or directories required.

## Complexity Tracking

No complexity violations. This is a straightforward command implementation reusing existing infrastructure.

## Next Steps

1. Run `/speckit.tasks` to generate task breakdown
2. Implement `dispatchCmd()` in `main.go`
3. Add tests for the new command
4. Document in quickstart.md
