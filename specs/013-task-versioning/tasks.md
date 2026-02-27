# Tasks: Task Versioning

**Input**: Design documents from `/specs/013-task-versioning/`
**Prerequisites**: plan.md, spec.md

**Tests**: Not explicitly requested. Tests omitted.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1-US6)

## Phase 1: Core Infrastructure

**Purpose**: Create versioning package with types and storage

### Implementation

- [ ] T001 Create `internal/versioning/` package directory
- [ ] T002 Define TaskVersion and VersionIndex structs in `internal/versioning/types.go` — TaskVersion with Version, Timestamp, Author, Content fields; VersionIndex with TaskID, CurrentVersion, Versions []VersionMeta (Version, Timestamp, Author)
- [ ] T003 [P] Implement VersionStore in `internal/versioning/store.go` — CreateVersion(taskID, content) that writes version JSON and updates index; GetVersion(taskID, version) to read specific version; ListVersions(taskID) returns VersionIndex; GetLatestVersion(taskID) returns most recent TaskVersion
- [ ] T004 [P] Create helper to get author name in `internal/versioning/store.go` — GetAuthor() checks git config (user.name) via `git config user.name`, falls back to system username via whoami

**Checkpoint**: Can create versions and list them for any task.

---

## Phase 2: Auto-Versioning Integration

**Purpose**: Hook versioning into task save operations

### Implementation

- [ ] T005 Add versioning hooks to `internal/project/project.go` — in AddTodo() and UpdateTodo(), call versioning.CreateVersion() after successful file write; compute diff for changes summary
- [ ] T006 Modify task file path resolution in `internal/project/project.go` — GetTodoPath() returns full path needed for versioning; expose GetTodoDir() for versioning package

**Checkpoint**: Any task create/update automatically creates a version.

---

## Phase 3: User Story 1 & 2 — Version History Display

**Goal**: Users can view version history with `anvil task history --versions`

### Implementation

- [ ] T007 [US1,US2] Add `--versions` flag to task history command in `cmd/anvil/main.go` — extend taskHistoryCmd with new flag, handle both run history (existing) and version history (new)
- [ ] T008 [US1,US2] Implement version list display — format output with columns: VERSION, DATE (RFC3339), AUTHOR; show "No versions yet" if empty
- [ ] T009 [US1,US2] Handle edge case: task not found, task has no versions

**Checkpoint**: `anvil task history --versions my-task` shows version list.

---

## Phase 4: User Story 3 — Diff Between Versions

**Goal**: Users can see what changed between versions with `anvil task diff`

### Implementation

- [ ] T010 [US3] Create new `taskDiffCmd` in `cmd/anvil/main.go` — command: `anvil task diff <name> [v1] [v2]`; if versions omitted, compare v1 to latest
- [ ] T011 [US3] Implement unified diff generation in `internal/versioning/differ.go` — DiffVersions(content1, content2) returns unified diff format string using diff algorithm
- [ ] T012 [US3] Handle edge cases: identical versions (show "No changes"), invalid version numbers, version not found

**Checkpoint**: `anvil task diff my-task v1 v3` shows diff between versions.

---

## Phase 5: User Story 4 — Restore Version

**Goal**: Users can revert to a previous version with `anvil task restore`

### Implementation

- [ ] T013 [US4] Create new `taskRestoreCmd` in `cmd/anvil/main.go` — command: `anvil task restore <name> <version>`; confirm before restore with prompt
- [ ] T014 [US4] Implement restore logic — read specified version content, overwrite task file, call CreateVersion() to record the restore as new version
- [ ] T015 [US4] Handle edge cases: invalid version, task not found, restore to current version (no-op)

**Checkpoint**: `anvil task restore my-task v2` restores and creates new version.

---

## Phase 6: User Story 5 & 6 — Git Blame and Metadata

**Goal**: Git blame integration and complete metadata

### Implementation

- [ ] T016 [US5] Create new `taskBlameCmd` in `cmd/anvil/main.go` — command: `anvil task blame <name>`; show help if not in git repo
- [ ] T017 [US5] Implement git blame in `internal/versioning/git.go` — GitBlame(taskPath) runs `git blame <path>` via os/exec, returns output; GitBlameAvailable() checks if task path is in git repo
- [ ] T018 [US5] Handle edge cases: not in git repo, git not installed, no commits yet

---

## Phase 7: Polish

### Implementation

- [ ] T019 Ensure backward compatibility — versioning is transparent, existing tasks work without versions
- [ ] T020 Handle migration: existing tasks get first version on first edit after this feature ships
- [ ] T021 Add help text for new commands: `anvil task diff --help`, `anvil task restore --help`, `anvil task blame --help`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Core)**: No dependencies
- **Phase 2 (Integration)**: Depends on Phase 1 (needs versioning package)
- **Phase 3 (History)**: Depends on Phase 2 (needs auto-versioning)
- **Phase 4 (Diff)**: Depends on Phase 1 (needs types)
- **Phase 5 (Restore)**: Depends on Phase 1 (needs store)
- **Phase 6 (Git)**: Depends on Phase 1 (can run parallel with others)
- **Phase 7 (Polish)**: Depends on all user stories

### Parallel Opportunities

- T003 and T004 can run in parallel — different files
- T007-T009 can run in parallel with T010-T012 — different commands
- T016-T018 can run in parallel with T013-T015 — different commands

---

## Implementation Strategy

### MVP First (Core + History)

1. Phase 1: Core Infrastructure (T001-T004)
2. Phase 2: Auto-Versioning (T005-T006)
3. Phase 3: History Display (T007-T009)
4. **STOP and VALIDATE**: Creating a task and running history --versions shows v1

### Incremental Delivery

1. Phase 4: Diff -> Deploy (Users can see changes)
2. Phase 5: Restore -> Deploy (Users can recover from mistakes)
3. Phase 6: Git Blame -> Deploy (Debugging who changed what)
4. Phase 7: Polish -> Deploy

---

## Notes

- All code uses Go stdlib only (encoding/json, os/exec, time, path/filepath)
- Version storage is per-task in project directory — versioned with project
- Total tasks: 21
