# Feature Specification: Task Output Diffing

**Feature Branch**: `013-task-output-diff`
**Created**: 2026-02-28
**Status**: Draft
**Input**: Issue #321: "Add task output diffing to compare execution results"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Compare Last Two Runs (Priority: P1)

A user wants to quickly see what changed between the most recent executions of a task. They run `anvil task diff my-task` and see a unified diff showing additions and deletions between the last two runs.

**Why this priority**: This is the core value proposition — without comparing at least two runs, there's no diff to show.

**Independent Test**: Can be tested by creating a task that produces deterministic output, running it twice with different results, and verifying the diff correctly identifies the changes.

**Acceptance Scenarios**:

1. **Given** a task "my-task" with output "Hello World" in run A and "Hello Universe" in run B, **When** the user runs `anvil task diff my-task`, **Then** a unified diff is displayed with run A shown with `-` prefix and run B with `+` prefix.
2. **Given** a task with identical output in last two runs, **When** the user runs `anvil task diff my-task`, **Then** the output indicates "No differences found" or shows an empty diff.
3. **Given** a task with only one historical run, **When** the user runs `anvil task diff my-task`, **Then** an error message indicates insufficient runs to compare.
4. **Given** a task that has never run, **When** the user runs `anvil task diff my-task`, **Then** an error message indicates the task has no run history.

---

### User Story 2 - Compare Specific Runs (Priority: P1)

A user wants to compare a known successful run against a failed run to understand what changed. They specify run IDs explicitly with `--run1` and `--run2` flags.

**Why this priority**: Enables root cause analysis by comparing any two runs, not just the most recent ones.

**Independent Test**: Can be tested by running a task multiple times, noting run IDs, and verifying that specifying those IDs produces the correct diff.

**Acceptance Scenarios**:

1. **Given** run IDs "abc123" and "def456" for the same task, **When** the user runs `anvil task diff my-task --run1 abc123 --run2 def456`, **Then** the diff compares exactly those two runs.
2. **Given** an invalid run ID, **When** the user runs `anvil task diff my-task --run1 invalid`, **Then** an error message indicates the run was not found.
3. **Given** run IDs from two different tasks, **When** the user runs `anvil task diff my-task --run1 abc123 --run2 xyz789` (where xyz789 belongs to a different task), **Then** an error message indicates the runs belong to different tasks.

---

### User Story 3 - Cross-Task Comparison (Priority: P2)

A user wants to compare outputs between two different tasks to understand differences in behavior. They provide two task names to compare the most recent runs of each.

**Why this priority**: Enables comparison across similar tasks (e.g., "fetch-data" vs "fetch-data-v2").

**Independent Test**: Can be tested by running two different tasks and verifying their outputs can be compared.

**Acceptance Scenarios**:

1. **Given** task "task-a" with output "Result: 100" and task "task-b" with output "Result: 200", **When** the user runs `anvil task diff task-a task-b`, **Then** the diff compares the most recent run of each task.
2. **Given** two tasks where one has no runs, **When** the user runs `anvil task diff task-a task-b`, **Then** an error message indicates one task has no run history.

---

### User Story 4 - Diff Options (Priority: P2)

A user wants to customize the diff output for their use case. They use `--context`, `--ignore-whitespace`, and `--json` flags to control the diff presentation.

**Why this priority**: Provides flexibility for different workflows (scripts, debugging, code review).

**Independent Test**: Can be tested by creating tasks with whitespace differences and verifying options work correctly.

**Acceptance Scenarios**:

1. **Given** output with whitespace differences, **When** the user runs `anvil task diff my-task --ignore-whitespace`, **Then** the diff ignores whitespace changes.
2. **Given** a large diff, **When** the user runs `anvil task diff my-task --context 10`, **Then** only 10 lines of context are shown around each change.
3. **Given** programmatic access is needed, **When** the user runs `anvil task diff my-task --json`, **Then** the output is valid JSON with structured diff data.

---

### User Story 5 - Run Information in Diff Header (Priority: P3)

A user wants to understand which runs are being compared at a glance. The diff header shows run IDs, timestamps, and status for both runs.

**Why this priority**: Provides context about what is being compared without needing to look up run details separately.

**Acceptance Scenarios**:

1. **Given** run A (abc123) from 2026-02-27 10:00 with status SUCCESS and run B (def456) from 2026-02-27 11:00 with status FAILED, **When** the user runs `anvil task diff my-task`, **Then** the diff header shows "--- Run abc123 (2026-02-27 10:00) SUCCESS" and "+++ Run def456 (2026-02-27 11:00) FAILED".

---

### Edge Cases

- What happens when output is binary or non-text? The diff should handle this gracefully, possibly showing a message that the output cannot be diffed.
- What happens when one output is empty? The diff should show all lines as added or deleted as appropriate.
- What happens when outputs are very large? Consider pagination or limiting output size for performance.
- What happens when comparing runs with different encodings? Assume UTF-8; handle encoding errors gracefully.
- How are runs ordered if they have the same timestamp? Use run ID or creation order as tiebreaker.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `anvil task diff <task-name>` command that compares the last two runs of the specified task.
- **FR-002**: System MUST support `--run1 <run-id>` flag to specify the first run for comparison.
- **FR-003**: System MUST support `--run2 <run-id>` flag to specify the second run for comparison.
- **FR-004**: System MUST support comparing two different tasks by providing two task names: `anvil task diff <task-a> <task-b>`.
- **FR-005**: System MUST output unified diff format by default.
- **FR-006**: System MUST support `--context <n>` flag to control the number of context lines shown (default: 3).
- **FR-007**: System MUST support `--ignore-whitespace` flag to ignore whitespace changes in the diff.
- **FR-008**: System MUST support `--json` flag to output structured JSON instead of human-readable diff.
- **FR-009**: System MUST display run metadata in the diff header including run ID, timestamp, and execution status.
- **FR-010**: System MUST return an error when comparing a task with fewer than 2 runs.
- **FR-011**: System MUST return an error when specified run IDs do not exist or belong to different tasks.
- **FR-012**: System MUST read run output from the existing run record storage location (`.anvil/runs/<task-id>/<run-id>/output`).

### Key Entities

- **Run Record**: Existing entity containing task execution results including output, status, start/end times, and metadata.
- **Diff Result**: Structured output containing the comparison between two run outputs, including added/removed/changed lines.

## Success Criteria *(mandurable)*

### Measurable Outcomes

- **SC-001**: Users can compare any two runs of a task with a single command.
- **SC-002**: Users can programmatically access diff results via JSON output.
- **SC-003**: Diff output follows standard unified diff format for familiarity.
- **SC-004**: Error messages are clear and actionable when comparison is not possible.
- **SC-005**: The command works with the existing run record storage system without modification.

## Assumptions

- Run output is stored as plain text in the existing location.
- Run IDs are unique within a task.
- The diff algorithm uses the standard unified diff format.
- JSON output includes both raw diff data and metadata for the compared runs.
