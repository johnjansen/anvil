# Tasks: Task Output Diffing (#321)

## Phase 1: Core Diff Functionality

### Task 1.1: Add ReadRunRecordByID helper
- **File:** `internal/project/project.go`
- **Description:** Add `ReadRunRecordByID(projectPath, taskID, runID string) (RunRecord, error)` function that reads a specific run record by ID. Returns error if run not found or doesn't belong to the task.
- **Dependencies:** None
- **Estimate:** 0.5h

### Task 1.2: Create diff data structures
- **File:** `internal/project/diff.go` (NEW)
- **Description:** Create `DiffResult` and `RunInfo` structs with JSON tags. Define `CompareOptions` struct for diff options.
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 1.3: Implement CompareRuns function
- **File:** `internal/project/diff.go` (NEW)
- **Description:** Implement `CompareRuns(run1, run2 RunRecord, opts CompareOptions) (DiffResult, error)` that:
  - Reads full log output for both runs
  - Applies ignore-whitespace option if set
  - Generates unified diff format
  - Returns DiffResult with metadata
- **Dependencies:** 1.2
- **Estimate:** 1h

### Task 1.4: Add taskDiffCmd to CLI
- **File:** `cmd/anvil/diff.go` (NEW)
- **Description:** Implement `taskDiffCmd(args []string)` that:
  - Parses `--run1`, `--run2`, `--context`, `--ignore-whitespace`, `--json` flags
  - Supports 1 or 2 task name arguments
  - Loads project and finds task(s)
  - Gets run records (last 2 or specific IDs)
  - Calls CompareRuns and outputs result
- **Dependencies:** 1.3
- **Estimate:** 1.5h

### Task 1.5: Register diff subcommand
- **File:** `cmd/anvil/main.go`
- **Description:** Add `case "diff"` to taskCmd switch, import diff package
- **Dependencies:** 1.4
- **Estimate:** 0.1h

### Task 1.6: Add tests for diff logic
- **File:** `internal/project/diff_test.go` (NEW)
- **Description:** Add table-driven tests for:
  - Identical outputs (no diff)
  - Different outputs (shows diff)
  - Ignore whitespace option
  - Context line limiting
  - Error cases (not found, different tasks)
- **Dependencies:** 1.3
- **Estimate:** 1h

## Phase 2: CLI Enhancements

### Task 2.1: Implement JSON output format
- **File:** `cmd/anvil/diff.go`
- **Description:** Add JSON marshal logic for DiffResult when `--json` flag is set. Include run metadata and diff in JSON output.
- **Dependencies:** 1.4
- **Estimate:** 0.5h

### Task 2.2: Implement cross-task comparison
- **File:** `cmd/anvil/diff.go`
- **Description:** When 2 task names provided, compare most recent runs of each. Validate both tasks exist and have runs.
- **Dependencies:** 1.4
- **Estimate:** 0.5h

### Task 2.3: Improve error messages
- **File:** `cmd/anvil/diff.go`
- **Description:** Ensure all error conditions return specific, actionable messages per spec (task not found, insufficient runs, invalid run ID, etc.)
- **Dependencies:** 1.4
- **Estimate:** 0.25h

## Phase 3: Testing & Polish

### Task 3.1: Manual testing
- **Description:** Create test tasks, run them with different outputs, verify diff output is correct
- **Dependencies:** Phase 1 & 2
- **Estimate:** 0.5h

### Task 3.2: Edge case handling
- **Description:** Test empty outputs, single-line outputs, binary-like content, very large outputs
- **Dependencies:** 3.1
- **Estimate:** 0.25h

### Task 3.3: Add to usage text
- **File:** `cmd/anvil/main.go`
- **Description:** Add "diff" to task subcommands in printUsage()
- **Dependencies:** 1.5
- **Estimate:** 0.1h

## Summary

| Phase | Tasks | Estimated Time |
|-------|-------|---------------|
| 1. Core Diff Functionality | 6 | ~4.45h |
| 2. CLI Enhancements | 3 | ~1.25h |
| 3. Testing & Polish | 3 | ~0.85h |
| **Total** | **12** | **~6.5h** |
