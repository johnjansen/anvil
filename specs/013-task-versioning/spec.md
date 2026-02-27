# Feature Specification: Task Versioning

**Feature Branch**: `[013-task-versioning]`
**Created**: 2026-02-27
**Status**: Draft
**Input**: GitHub Issue #308: Add task diff and versioning for tracking changes over time

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Auto-Versioning on Task Save (Priority: P1)

User wants task changes automatically tracked so they can see history.

**Why this priority**: Core feature - automatic versioning is the foundation for all other features.

**Independent Test**: Can be tested by modifying a task and checking if version is created.

**Acceptance Scenarios**:

1. **Given** a task exists in the project, **When** task file is modified and saved, **Then** a new version is automatically created in `.anvil/versions/<task-id>/`
2. **Given** a new task is created, **When** task file is written, **Then** initial version (v1) is created
3. **Given** multiple changes to a task, **When** each change is saved, **Then** versions are numbered sequentially (v1, v2, v3...)

---

### User Story 2 - View Version History (Priority: P1)

User wants to see what versions exist and when they were created.

**Why this priority**: Core feature - essential for understanding task evolution.

**Independent Test**: Can be tested by running `anvil task history --versions <name>` and verifying output.

**Acceptance Scenarios**:

1. **Given** a task with multiple versions, **When** `anvil task history --versions <name>` is run, **Then** version list is displayed with version number, date, and author
2. **Given** a task with no versions (created before versioning), **When** command is run, **Then** helpful message is shown
3. **Given** task exists but no changes made yet, **When** command is run, **Then** only v1 is shown

---

### User Story 3 - Diff Between Versions (Priority: P1)

User wants to see exactly what changed between two versions.

**Why this priority**: Core feature - allows identifying what caused failures.

**Independent Test**: Can be tested by modifying a task and running diff.

**Acceptance Scenarios**:

1. **Given** a task with multiple versions, **When** `anvil task diff <name> v1 v3` is run, **Then** unified diff is displayed showing changes
2. **Given** a task with versions, **When** no versions specified, **Then** diff between v1 and latest is shown
3. **Given** identical versions, **Then** empty diff with message "No changes"

---

### User Story 4 - Restore Previous Version (Priority: P1)

User wants to revert to a previous version after a bad change.

**Why this priority**: Core feature - provides recovery from mistakes.

**Independent Test**: Can be tested by restoring a version and verifying content.

**Acceptance Scenarios**:

1. **Given** a task with multiple versions, **When** `anvil task restore <name> v2` is run, **Then** task file is restored to v2 content
2. **Given** a task is restored, **When** restoration completes, **Then** a new version is created (vN+1) recording the restore
3. **Given** invalid version specified, **When** restore is attempted, **Then** error message shows valid versions

---

### User Story 5 - Git Blame Integration (Priority: P2)

User wants to see who changed the task and when using git.

**Why this priority**: Nice-to-have - leverages existing git infrastructure.

**Independent Test**: Can be tested by modifying task in git and running blame.

**Acceptance Scenarios**:

1. **Given** task file is in a git repository, **When** `anvil task blame <name>` is run, **Then** git blame output is shown for the task file
2. **Given** task file is not in git, **When** blame is attempted, **Then** message explains git is not available

---

### User Story 6 - Version Metadata (Priority: P1)

User wants version history to include timestamp and author information.

**Why this priority**: Core feature - provides context for changes.

**Independent Test**: Can be tested by checking version metadata.

**Acceptance Scenarios**:

1. **Given** a version is created, **When** version metadata is stored, **Then** includes: timestamp (ISO 8601), author (git user.name or system username)
2. **Given** version is created via restore, **Then** author is recorded as the restorer

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Task versions MUST be stored in `.anvil/versions/<task-id>/` directory
- **FR-002**: Each version MUST be a JSON file containing: version number, timestamp, author, and full task file content
- **FR-003**: Version creation MUST happen automatically on task file write/modify
- **FR-004**: `anvil task history --versions <name>` MUST display version list
- **FR-005**: `anvil task diff <name> [v1] [v2]` MUST show unified diff between versions
- **FR-006**: `anvil task restore <name> <version>` MUST restore task file to specified version
- **FR-007**: Restore MUST create a new version recording the restore
- **FR-008**: `anvil task blame <name>` MUST show git blame if task is in git repo
- **FR-009**: Author MUST be determined from git config (user.name) or system username

### Key Entities

- **TaskVersion**: Struct containing Version (int), Timestamp (time.Time), Author (string), Content (string)
- **VersionStore**: Manages reading/writing versions in `.anvil/versions/<task-id>/`
- **TaskConfig**: Existing config - no changes needed (versioning is external)

### Data Model

```
.anvil/versions/<task-id>/
  versions.json          # Index file with all version metadata
  v1.json                # Version 1 content
  v2.json                # Version 2 content
  ...
```

Each version file:
```json
{
  "version": 1,
  "timestamp": "2026-02-27T10:00:00Z",
  "author": "john",
  "content": "---\nid: \"task-id\"\nschedule: \"0 9 * * *\"\n...\n# Task content"
}
```

versions.json index:
```json
{
  "task_id": "abc-123",
  "current_version": 3,
  "versions": [
    {"version": 1, "timestamp": "2026-02-25T08:00:00Z", "author": "john"},
    {"version": 2, "timestamp": "2026-02-26T09:00:00Z", "author": "john"},
    {"version": 3, "timestamp": "2026-02-27T10:00:00Z", "author": "john"}
  ]
}
```

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Task modifications automatically create version snapshots
- **SC-002**: Users can view version history with `anvil task history --versions`
- **SC-003**: Users can diff any two versions with `anvil task diff`
- **SC-004**: Users can restore previous versions with `anvil task restore`
- **SC-005**: Version metadata includes timestamp and author
- **SC-006**: Git blame works when task is in git repository
