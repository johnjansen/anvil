# Feature Specification: Task Rollback

**Feature Branch**: `022-task-rollback`
**Created**: 2026-03-01
**Status**: Draft
**Input**: User description: "Add task rollback to revert to previous successful run"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - List available restore points (Priority: P1)

A user wants to see what previous successful runs are available for rollback.

**Scenario**: User runs `anvil task rollback my-task` to list all successful runs with timestamps and output sizes.

**Why this priority**: This is the primary entry point - users must be able to discover what restore points exist before choosing one.

**Independent Test**: Can be tested by running the command against a task with multiple historical runs and verifying the table displays correctly with run ID, timestamp, status, and output size.

**Acceptance Scenarios**:

1. **Given** a task has multiple successful runs, **When** user runs `anvil task rollback my-task`, **Then** table shows all successful runs sorted by timestamp (newest first)
2. **Given** a task has no previous runs, **When** user runs rollback command, **Then** display message indicating no restore points available
3. **Given** a task has failed runs mixed with successful runs, **When** user runs rollback command, **Then** only successful runs are displayed

---

### User Story 2 - Restore to previous run (Priority: P1)

A user wants to restore files from a specific previous successful run.

**Scenario**: User runs `anvil task rollback my-task abc123` to restore files from run abc123.

**Why this priority**: This is the core rollback functionality - the primary value of the feature.

**Independent Test**: Can be tested by creating files, running a new task that changes them, then rolling back and verifying files match the previous run.

**Acceptance Scenarios**:

1. **Given** a valid run ID exists, **When** user runs rollback command, **Then** files are restored to match that run's output
2. **Given** the run ID does not exist, **When** user runs rollback command, **Then** error message indicates invalid run ID
3. **Given** some files to restore no longer exist in current directory, **When** user runs rollback command, **Then** missing files are created from the restore point

---

### User Story 3 - Dry-run preview (Priority: P1)

A user wants to see what would happen without actually restoring files.

**Scenario**: User runs `anvil task rollback my-task abc123 --dry-run` to preview changes.

**Why this priority**: Safety feature - users need confidence before making destructive changes.

**Independent Test**: Can be tested by running with --dry-run and verifying no files are modified.

**Acceptance Scenarios**:

1. **Given** a valid run ID, **When** user runs with --dry-run, **Then** no files are modified but preview of changes is shown
2. **Given** --dry-run flag, **When** rollback preview runs, **Then** displays list of files that would be restored/deleted

---

### User Story 4 - Restore specific files (Priority: P2)

A user wants to restore only certain files rather than all output.

**Scenario**: User runs `anvil task rollback my-task abc123 --files output.json,report.csv` to restore only those files.

**Why this priority**: Common use case when user knows only certain files are problematic.

**Independent Test**: Can be tested by specifying certain files and verifying only those are restored.

**Acceptance Scenarios**:

1. **Given** a valid run ID and file list, **When** user runs with --files flag, **Then** only specified files are restored
2. **Given** a file in --files list doesn't exist in restore point, **When** user runs rollback, **Then** error indicates file not found in restore point
3. **Given** file list is empty or invalid, **When** user runs rollback, **Then** error indicates invalid file list

---

### User Story 5 - Rollback hooks (Priority: P3)

A user wants to run custom scripts before/after rollback.

**Scenario**: User configures `on_rollback` hook in task config to run custom script with run ID substitution.

**Why this priority**: Automation and integration with external systems.

**Independent Test**: Can be tested by configuring a hook and verifying it executes with correct variables.

**Acceptance Scenarios**:

1. **Given** on_rollback hook configured, **When** rollback executes, **Then** hook runs before files are restored
2. **Given** hook template includes `{{ .RunID }}`, **When** hook executes, **Then** variable is substituted with actual run ID
3. **Given** hook script fails (non-zero exit), **When** rollback runs, **Then** rollback is aborted and error returned

---

### Edge Cases

- What happens when rollback target run's output files have been deleted from storage?
- How does the system handle concurrent rollbacks to the same task?
- What happens when current working directory has changed since original run?
- How does rollback handle symlinks or special file types?
- What if the task config file references outputs that no longer exist?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST list all successful runs for a given task when no run ID is specified
- **FR-002**: System MUST display run ID, timestamp, status, and output size in tabular format
- **FR-003**: Users MUST be able to restore files from any specified successful run
- **FR-004**: System MUST support --dry-run flag to preview changes without applying them
- **FR-005**: Users MUST be able to specify --files flag to restore only certain files
- **FR-006**: System MUST execute on_rollback hook before restoring files
- **FR-007**: System MUST substitute template variables in hook command (RunID, TaskName)
- **FR-008**: System MUST abort rollback if hook fails (non-zero exit code)
- **FR-009**: System MUST record rollback events in task history
- **FR-010**: System MUST error when attempting to restore from a failed run

### Key Entities

- **RunRecord**: Represents a previous task execution with timestamp, status, output files, and metadata
- **RestorePoint**: A specific successful RunRecord that can be used as a rollback target
- **RollbackEvent**: Records when a rollback occurred, who initiated it, and which run was restored from

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can list all available restore points in under 2 seconds for tasks with 100+ historical runs
- **SC-002**: Users can restore files from a previous run with a single command
- **SC-003**: --dry-run correctly predicts all file changes without modifying any files
- **SC-004**: Rollback completes for tasks with 100+ output files in under 10 seconds
- **SC-005**: on_rollback hook executes successfully with correct variable substitution

