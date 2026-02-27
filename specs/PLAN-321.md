# Implementation Plan: Task Output Diffing

**Branch**: `013-task-output-diff` | **Date**: 2026-02-28 | **Spec**: [SPEC-321.md](SPEC-321.md)
**Input**: Issue #321: "Add task output diffing to compare execution results"

## Summary

Add CLI command `anvil task diff` to compare outputs between task runs. Supports comparing last two runs of a task, specific runs via `--run1`/`--run2`, cross-task comparison, and JSON output. Uses unified diff format matching standard Unix diff conventions.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/project` (RunRecord, ReadAllRunRecords), standard library (`bytes`, `strings`)
**Storage**: Existing run record locations (`.anvil/runs/<task-id>/`) and log files (`.anvil/logs/<task-id>/`)
**Testing**: `go test ./...` with table-driven tests
**Target Platform**: macOS, Linux (CLI tool)
**Project Type**: CLI tool
**Performance Goals**: Diff operation should be fast (<100ms for typical outputs)
**Constraints**: No changes to daemon or existing storage format
**Scale/Scope**: Per-project task management (typically 1-50 tasks per project)

## Constitution Check

*GATE: Must pass before implementation. Re-check after research.*

- Tests included for all new logic: YES
- Backward compatible (no changes to existing behavior): YES
- Follows existing patterns (CLI commands in main.go, project helpers): YES

**Post-Research re-check**: Design follows established patterns. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/013-task-output-diff/
├── SPEC-321.md            # Feature specification (this file parent)
└── plan.md                # This file
```

### Source Code (repository root)

```text
cmd/anvil/
├── main.go               # Add taskDiffCmd, integrate into task subcommand
└── diff.go               # NEW: Diff command implementation

internal/project/
├── project.go            # Add ReadRunRecordByID helper function
└── diff.go               # NEW: Diff logic and data structures
```

**Structure Decision**: New `diff.go` files isolate diff logic from existing code. Project helper in `internal/project/diff.go` provides reusable diff functions. CLI handler in `cmd/anvil/diff.go` handles command-line parsing and output formatting.

## Technical Design

### 1. Data Structures

**DiffResult (new)**
```go
type DiffResult struct {
    Run1       RunInfo `json:"run1"`
    Run2       RunInfo `json:"run2"`
    Diff       string  `json:"diff"`        // unified diff format
    HasChanges bool    `json:"has_changes"`
}

type RunInfo struct {
    RunID     string    `json:"run_id"`
    TaskID    string    `json:"task_id"`
    TaskName  string    `json:"task_name"`
    Timestamp time.Time `json:"timestamp"`
    Success   bool      `json:"success"`
    Output    string    `json:"output,omitempty"`
}
```

### 2. Output Retrieval

- Primary: Read full log file from `.anvil/logs/<task-id>/<session-id>.log`
- Fallback: Use `OutputSummary` from RunRecord if log unavailable
- Error: Return clear message if no output available

### 3. Diff Algorithm

- Use Go's standard `diff` package or implement unified diff manually
- Support `--ignore-whitespace` via strings.Replace before diffing
- Context lines controlled by `--context` flag (default: 3)

### 4. CLI Interface

```
anvil task diff <task-name> [flags]           # compare last two runs
anvil task diff <task-name> --run1 ID         # compare run1 against second-to-last
anvil task diff <task-name> --run1 ID1 --run2 ID2  # compare two specific runs
anvil task diff <task-a> <task-b>             # cross-task comparison
```

Flags:
- `--run1, -1` string   : First run ID
- `--run2, -2` string   : Second run ID
- `--context, -C` int   : Lines of context (default 3)
- `--ignore-whitespace` : Ignore whitespace changes
- `--json, -j`         : JSON output

### 5. Error Handling

| Condition | Message |
|-----------|---------|
| Task not found | "task not found: <name>" |
| Task has 0 runs | "task '<name>' has no run history" |
| Task has 1 run | "task '<name>' has only 1 run, need at least 2 to diff" |
| Invalid run ID | "run not found: <run-id>" |
| Runs from different tasks | "run <run-id> belongs to task '<other-task>', not '<task>'" |

## Implementation Phases

### Phase 1: Core Diff Functionality

1. Add `ReadRunRecordByID(projectPath, taskID, runID)` helper to `internal/project/project.go`
2. Create `internal/project/diff.go` with DiffResult type and CompareRuns function
3. Create `cmd/anvil/diff.go` with taskDiffCmd handler
4. Add diff subcommand to task command tree in main.go
5. Add tests for diff logic

### Phase 2: CLI Enhancements

1. Implement `--json` flag output
2. Implement `--context` and `--ignore-whitespace` options
3. Add cross-task comparison support
4. Add tests for CLI options

### Phase 3: Polish

1. Verify error messages are helpful
2. Test edge cases (empty output, binary, large files)
3. Update documentation if needed

## Dependencies & Risks

**Dependencies**: None (uses existing storage)
**Risks**:
- Log file retention: If logs are pruned, diff may have limited history. Acceptable — show clear error.
- Large outputs: Consider limiting to last N lines if performance issues arise. Acceptable — document limitation.

## Acceptance Criteria Mapping

| AC | Implementation |
|----|----------------|
| `anvil task diff <name>` compares last two runs | taskDiffCmd with no --run1/--run2 |
| `--run1` and `--run2` compare specific runs | Flag handlers in taskDiffCmd |
| Unified diff format with context | difflib or custom unified format |
| `--json` for programmatic access | JSON marshal of DiffResult |

## Success Metrics

- Command returns exit code 0 on success, non-zero on error
- Diff output matches standard unified format
- JSON output is valid and parseable
- Error messages are actionable and specific
