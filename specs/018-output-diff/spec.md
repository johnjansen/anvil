# Feature Specification: Task Output Diffing

**Feature Branch**: `018-output-diff`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "Add task output diffing to compare execution results"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Compare Last Two Runs (Priority: P1)

A user wants to quickly see what changed between the most recent two runs of a task. They run `anvil task diff my-task` and see a unified diff showing the differences in output between the two most recent executions.

**Why this priority**: This is the most common use case — users want a quick way to see "what changed since last time" without needing to know run IDs.

**Independent Test**: Can be fully tested by running a task twice with different outputs and verifying the diff command shows the differences in unified diff format.

**Acceptance Scenarios**:

1. **Given** a task with at least two completed runs, **When** the user runs `anvil task diff my-task`, **Then** the system displays a unified diff comparing the second-most-recent run output to the most recent run output with headers showing run IDs, timestamps, and status.
2. **Given** a task with only one completed run, **When** the user runs `anvil task diff my-task`, **Then** the system displays an error message indicating there are not enough runs to compare.
3. **Given** a task with two runs that have identical output, **When** the user runs `anvil task diff my-task`, **Then** the system displays a message indicating the outputs are identical.

---

### User Story 2 - Compare Specific Runs (Priority: P2)

A user wants to compare the output of two specific runs, perhaps a known-good run and a recent failure. They specify run IDs with `--run1` and `--run2` flags to select exact runs.

**Why this priority**: Targeted comparison is important for debugging but less frequent than the default last-two comparison.

**Independent Test**: Can be tested by creating multiple runs, selecting two by ID, and verifying the diff output matches those specific runs.

**Acceptance Scenarios**:

1. **Given** a task with multiple runs, **When** the user runs `anvil task diff my-task --run1 abc123 --run2 def456`, **Then** the system displays a unified diff comparing the output of run abc123 to run def456.
2. **Given** an invalid run ID, **When** the user runs `anvil task diff my-task --run1 invalid`, **Then** the system displays an error indicating the run was not found.

---

### User Story 3 - Programmatic Diff Output (Priority: P3)

A user or script needs to consume diff results programmatically. They use `--json` to get structured JSON output containing the diff details.

**Why this priority**: Important for automation and integration but not the primary interactive use case.

**Independent Test**: Can be tested by running the diff command with `--json` and verifying the output is valid JSON with the expected fields.

**Acceptance Scenarios**:

1. **Given** a task with at least two runs, **When** the user runs `anvil task diff my-task --json`, **Then** the system outputs valid JSON containing run metadata and diff hunks.

---

### Edge Cases

- What happens when a task has no runs at all?
- What happens when a run has no captured output (empty output summary)?
- What happens when the specified task does not exist?
- What happens when `--run1` is provided without `--run2`?
- How does the diff handle very large outputs?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `diff` subcommand under `anvil task` that compares task run outputs
- **FR-002**: System MUST default to comparing the two most recent runs when no run IDs are specified
- **FR-003**: System MUST support `--run1` and `--run2` flags to compare specific runs by run ID
- **FR-004**: System MUST display output in unified diff format with configurable context lines via `--context N` (default 3)
- **FR-005**: System MUST support `--ignore-whitespace` flag to ignore whitespace differences
- **FR-006**: System MUST support `--json` flag for structured JSON output of diff results
- **FR-007**: System MUST display diff headers showing run ID, timestamp, and success/failure status for each run
- **FR-008**: System MUST display a clear message when outputs are identical (no differences found)
- **FR-009**: System MUST display appropriate error messages for invalid inputs (missing task, insufficient runs, invalid run IDs)

### Key Entities

- **RunRecord**: Existing entity storing task execution results including output summary, timestamps, and status
- **DiffResult**: The computed difference between two run outputs, containing metadata and change hunks

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can compare task outputs in under 2 seconds for typical run sizes
- **SC-002**: Diff output matches standard unified diff format that users familiar with `git diff` or `diff -u` will recognize
- **SC-003**: JSON output is parseable by standard JSON tools and contains all diff information
- **SC-004**: All error cases produce actionable error messages that guide users to correct usage

## Assumptions

- Output comparison uses the `OutputSummary` field from existing RunRecord data
- Run IDs can be partial prefixes as long as they uniquely identify a run
- The unified diff algorithm follows standard line-by-line comparison
- Context lines default to 3 (matching standard diff behavior)
