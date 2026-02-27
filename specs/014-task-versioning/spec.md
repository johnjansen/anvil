# Feature Specification: Task Diff and Versioning

**Feature Branch**: `014-task-versioning`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task diff and versioning for tracking changes over time"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Task Version History (Priority: P1)

As an operator, I want to see a chronological list of all versions of a task file so I can understand when and how a task was modified over time.

**Why this priority**: Version visibility is the foundation -- without seeing versions, diff and restore are meaningless.

**Independent Test**: Modify a task file several times. Run `anvil task history --versions <name>` and verify all versions appear with timestamps, authors, and change summaries.

**Acceptance Scenarios**:

1. **Given** a task that has been modified 3 times, **When** the user runs `anvil task history --versions my-task`, **Then** the output shows 3 versions (v1, v2, v3) with dates, authors, and brief change descriptions.
2. **Given** a task that has never been modified (only created), **When** the user runs `anvil task history --versions my-task`, **Then** the output shows a single version (v1) marked as "initial version".
3. **Given** a nonexistent task name, **When** the user runs `anvil task history --versions no-such-task`, **Then** the output shows an error "task not found".

---

### User Story 2 - Diff Between Versions (Priority: P2)

As an operator, I want to compare two versions of a task to see exactly what changed (frontmatter and content) so I can diagnose why a task started behaving differently.

**Why this priority**: Diffing is the primary diagnostic tool -- operators need it to understand behavioral changes.

**Independent Test**: Modify a task's schedule and retry settings. Run `anvil task diff my-task v1 v2` and verify the diff shows the exact frontmatter changes.

**Acceptance Scenarios**:

1. **Given** a task with versions v1 and v3, **When** the user runs `anvil task diff my-task v1 v3`, **Then** the output shows a unified diff of the task file between those versions.
2. **Given** only one version specified, **When** the user runs `anvil task diff my-task v2`, **Then** the system diffs version v2 against the current file.
3. **Given** an invalid version number, **When** the user runs `anvil task diff my-task v99`, **Then** the output shows an error "version not found".

---

### User Story 3 - Restore Previous Version (Priority: P3)

As an operator, I want to restore a task to a previous version so I can recover from accidental changes or revert a bad modification.

**Why this priority**: Restore is a safety net that builds on version history.

**Independent Test**: Modify a task, then run `anvil task restore my-task v1` and verify the task file content matches the v1 snapshot.

**Acceptance Scenarios**:

1. **Given** a task at v3, **When** the user runs `anvil task restore my-task v1`, **Then** the task file content is replaced with v1 content and a new version v4 is created noting "restored from v1".
2. **Given** a task at v2, **When** the user runs `anvil task restore my-task v2`, **Then** the output says "task is already at v2" and no changes are made.

---

### User Story 4 - Git Blame Integration (Priority: P4)

As an operator with a git-tracked project, I want to see git blame information for a task file so I can see who changed each line and when.

**Why this priority**: Git blame is a convenience feature that leverages existing git infrastructure.

**Independent Test**: Run `anvil task blame my-task` on a git-tracked task and verify line-by-line attribution.

**Acceptance Scenarios**:

1. **Given** a git-tracked task file, **When** the user runs `anvil task blame my-task`, **Then** the output shows git blame output with commit hash, author, date, and line content.
2. **Given** a project not under git, **When** the user runs `anvil task blame my-task`, **Then** the output says "git blame not available: project is not in a git repository".

---

### Edge Cases

- What happens when a task file is deleted and recreated? Version history starts fresh.
- What happens when the versions directory is missing? It is created automatically on first version save.
- What happens when the task file is modified outside of anvil (e.g., direct editor)? The daemon detects changes on next tick and auto-snapshots.
- What happens with very large task files? Versions are stored as full snapshots (not diffs) for simplicity.
- What happens when restoring creates a file identical to the current version? Skipped with "no changes" message.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST automatically create a version snapshot when a task file is created or modified.
- **FR-002**: Version snapshots MUST include the full task file content, timestamp, author, and a sequential version number.
- **FR-003**: System MUST provide `anvil task history --versions <name>` to display all versions of a task.
- **FR-004**: System MUST provide `anvil task diff <name> <v1> [v2]` to show a unified diff between versions (or between a version and the current file).
- **FR-005**: System MUST provide `anvil task restore <name> <version>` to revert a task to a previous version.
- **FR-006**: Restoring a version MUST create a new version snapshot (not overwrite history).
- **FR-007**: System MUST provide `anvil task blame <name>` that delegates to git blame if the project is git-tracked.
- **FR-008**: Version metadata MUST include timestamp and author (from git config or system username).
- **FR-009**: The daemon MUST detect task file modifications and automatically create version snapshots.

### Key Entities

- **Task Version**: A snapshot of a task file at a point in time (version number, content, timestamp, author, change summary)
- **Version Metadata**: Information about a version (sequential number, creation date, author name, brief description of changes)
- **Version Store**: The storage location for task version snapshots (per-task directory structure)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can view the complete version history of any task in under 1 second.
- **SC-002**: Operators can compare any two versions of a task and see exact changes in under 1 second.
- **SC-003**: Operators can restore a task to any previous version with a single command.
- **SC-004**: All task modifications are automatically versioned with no manual intervention required.
- **SC-005**: Version history survives daemon restarts (persisted to disk).

## Assumptions

- Version snapshots are stored as full file copies in `.anvil/versions/<task-name>/` for simplicity.
- Version numbers are sequential integers starting at 1 (v1, v2, v3...).
- Author is determined from git config (user.name) if available, otherwise from the system username.
- The daemon's existing task file hash tracking (from the audit feature) can be extended to trigger auto-versioning on changes.
- Diff output uses unified diff format, similar to `diff -u`.
- Version storage does not have a configurable retention limit in the initial implementation; all versions are kept.
