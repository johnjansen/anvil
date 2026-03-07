# Feature Specification: Task Execution Snapshots for Debugging

**Feature Branch**: `024-task-snapshots`
**Created**: 2026-03-01
**Status**: Draft
**Input**: User description: "Add task execution snapshots for debugging failed runs"

## User Scenarios & Testing

### User Story 1 - Debug Failed Task Execution (Priority: P1)

As a developer debugging a failed task, I want to access the complete execution context so that I can understand what went wrong.

**Why this priority**: This is the primary use case - when a task fails, developers need immediate access to configuration, environment, and prompt details to diagnose the issue.

**Independent Test**: Can be tested by running a task that fails and verifying all snapshot files are captured with correct content.

**Acceptance Scenarios**:

1. **Given** a task has been executed, **When** I run `anvil task snapshot <task-name>`, **Then** I can see the latest run's snapshot with config, env vars, prompt, and file listing
2. **Given** a task has multiple runs, **When** I specify `--run <run-id>`, **Then** I see the snapshot for that specific run
3. **Given** a task runs and completes (success or failure), **When** the run finishes, **Then** a snapshot is automatically created in `.anvil/runs/<task-id>/<run-id>/snapshot/`

---

### User Story 2 - Inspect Specific Snapshot Files (Priority: P2)

As a user, I want to view specific files from a snapshot to focus on the information I need.

**Why this priority**: Sometimes users only need to check one piece of context (e.g., just the prompt or just environment variables) without viewing everything.

**Independent Test**: Can be tested by requesting individual files and verifying correct content is displayed.

**Acceptance Scenarios**:

1. **Given** a snapshot exists for a run, **When** I run `anvil task snapshot <task-name> --file prompt.txt`, **Then** only the expanded prompt is displayed
2. **Given** a snapshot exists, **When** I request a non-existent file, **Then** an error message indicates the file is not available

---

### User Story 3 - Compare Two Run Snapshots (Priority: P3)

As a developer, I want to compare two runs of the same task to understand what changed between them.

**Why this priority**: This helps identify whether changes in configuration, environment, or input caused different outcomes between runs.

**Independent Test**: Can be tested by running a task twice with different inputs and comparing the snapshots.

**Acceptance Scenarios**:

1. **Given** a task has at least two runs with snapshots, **When** I run `anvil task snapshot-diff <task-name> --run1 <id1> --run2 <id2>`, **Then** I see a diff showing differences between the two runs
2. **Given** the two runs have identical snapshots, **Then** the diff output indicates no differences found
3. **Given** one of the runs does not exist, **Then** an error message explains which run ID is invalid

---

### User Story 4 - Automatic Snapshot Pruning (Priority: P2)

As a user, I want snapshots to be automatically managed so they don't consume excessive disk space.

**Why this priority**: Without pruning, snapshots from many runs would accumulate and consume significant storage.

**Independent Test**: Can be tested by running many tasks and verifying old snapshots are removed according to retention policy.

**Acceptance Scenarios**:

1. **Given** snapshot retention is configured, **When** the limit is reached, **Then** older snapshots are automatically pruned
2. **Given** a task is deleted, **When** cleanup occurs, **Then** all associated snapshots are also removed

---

### Edge Cases

- What happens when the task runs in a directory that no longer exists?
- How does the system handle snapshot creation when disk space is low?
- What if the task is interrupted before completion - is a partial snapshot created?
- How are snapshots handled for tasks that run on remote workers?

## Requirements

### Functional Requirements

- **FR-001**: System MUST automatically capture a snapshot for every task run (success or failure)
- **FR-002**: Snapshot MUST contain the task's full configuration (frontmatter settings)
- **FR-003**: Snapshot MUST contain all resolved environment variables with their values
- **FR-004**: Snapshot MUST contain the expanded prompt with all variables substituted
- **FR-005**: Snapshot MUST contain a directory listing of the task's working directory at start
- **FR-006**: Users MUST be able to view the latest snapshot via `anvil task snapshot <name>`
- **FR-007**: Users MUST be able to view a specific run's snapshot via `anvil task snapshot <name> --run <id>`
- **FR-008**: Users MUST be able to view a specific file within a snapshot via `--file <filename>`
- **FR-009**: Users MUST be able to diff two snapshots via `anvil task snapshot-diff <name> --run1 <id1> --run2 <id2>`
- **FR-010**: System MUST automatically prune old snapshots based on retention settings
- **FR-011**: Snapshot storage path MUST be `.anvil/runs/<task-id>/<run-id>/snapshot/`

### Key Entities

- **Run Snapshot**: Collection of files capturing the execution context for a single task run
- **Snapshot File**: Individual file within a snapshot (config, env, prompt, files listing, run record)
- **Retention Policy**: Rules determining how many snapshots to retain and for how long

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can view complete execution context for any task run within 5 seconds of requesting
- **SC-002**: Snapshot creation adds less than 2 seconds to total task execution time
- **SC-003**: 100% of task runs (success or failure) produce a snapshot
- **SC-004**: Users can successfully compare any two snapshots of the same task
- **SC-005**: Snapshot storage is automatically managed to prevent unbounded disk growth
