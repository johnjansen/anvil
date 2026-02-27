# Tasks: Task Diff and Versioning

**Input**: Design documents from `/specs/014-task-versioning/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Core types and utility functions needed by all user stories

- [x] T001 Add TaskVersion struct and version path functions in internal/project/project.go
- [x] T002 [P] Add getAuthor() utility function in internal/project/project.go
- [x] T003 [P] Create unified diff function in internal/project/diff.go

---

## Phase 2: Foundational (Auto-Versioning Daemon Integration)

**Purpose**: Automatic version snapshot creation on task file changes -- MUST complete before CLI commands work

**CRITICAL**: Without auto-versioning, there are no versions to display, diff, or restore.

- [x] T004 Add WriteTaskVersion() and ReadAllVersions() functions in internal/project/project.go
- [x] T005 Add ComputeFileHash() function in internal/project/project.go
- [x] T006 Add taskHashes map to Daemon struct and initialize in NewDaemon or Start in internal/daemon/daemon.go
- [x] T007 Add version snapshot logic in tick() after LoadTodos() in internal/daemon/daemon.go -- compute hash for each task, compare with stored hash, call WriteTaskVersion() on change

**Checkpoint**: Daemon now auto-creates version snapshots when task files change

---

## Phase 3: User Story 1 - View Task Version History (Priority: P1) MVP

**Goal**: Operators can view a chronological list of all versions of a task file

**Independent Test**: Modify a task file several times, run `anvil task history --versions <name>` and verify all versions appear with timestamps, authors, and change summaries.

### Implementation for User Story 1

- [x] T008 [US1] Add --versions flag to taskHistoryCmd() in cmd/anvil/main.go -- parse flag, load versions via ReadAllVersions(), display table with VERSION, DATE, AUTHOR, SUMMARY columns
- [x] T009 [US1] Add --json support for --versions output in cmd/anvil/main.go
- [x] T010 [US1] Handle error cases: task not found, no versions found in cmd/anvil/main.go

**Checkpoint**: `anvil task history --versions <name>` displays version list

---

## Phase 4: User Story 2 - Diff Between Versions (Priority: P2)

**Goal**: Operators can compare two versions of a task to see exactly what changed

**Independent Test**: Modify a task's schedule and retry settings, run `anvil task diff my-task v1 v2` and verify the diff shows exact frontmatter changes.

### Implementation for User Story 2

- [x] T011 [US2] Add `case "diff":` to taskCmd() dispatcher in cmd/anvil/main.go
- [x] T012 [US2] Implement taskDiffCmd() in cmd/anvil/main.go -- parse args (name, v1, optional v2), load version content from ReadAllVersions(), read current file if v2 omitted, call diff function, print unified diff output
- [x] T013 [US2] Handle error cases in taskDiffCmd(): task not found, version not found, identical content in cmd/anvil/main.go

**Checkpoint**: `anvil task diff <name> <v1> [v2]` shows unified diff

---

## Phase 5: User Story 3 - Restore Previous Version (Priority: P3)

**Goal**: Operators can restore a task to a previous version

**Independent Test**: Modify a task, run `anvil task restore my-task v1` and verify the task file content matches v1 snapshot.

### Implementation for User Story 3

- [x] T014 [US3] Add `case "restore":` to taskCmd() dispatcher in cmd/anvil/main.go
- [x] T015 [US3] Implement taskRestoreCmd() in cmd/anvil/main.go -- parse args (name, version), load version content, compare with current file, write restored content to task file, create new version snapshot with "restored from vN" summary
- [x] T016 [US3] Handle error cases in taskRestoreCmd(): task not found, version not found, already at version, no changes in cmd/anvil/main.go

**Checkpoint**: `anvil task restore <name> <version>` reverts task and creates new version

---

## Phase 6: User Story 4 - Git Blame Integration (Priority: P4)

**Goal**: Operators can see git blame information for a task file

**Independent Test**: Run `anvil task blame my-task` on a git-tracked task and verify line-by-line attribution.

### Implementation for User Story 4

- [x] T017 [US4] Add `case "blame":` to taskCmd() dispatcher in cmd/anvil/main.go
- [x] T018 [US4] Implement taskBlameCmd() in cmd/anvil/main.go -- resolve task file path, check if project is git-tracked (exec `git rev-parse --git-dir`), exec `git blame <path>` and pipe output to stdout
- [x] T019 [US4] Handle error cases in taskBlameCmd(): task not found, not a git repository in cmd/anvil/main.go

**Checkpoint**: `anvil task blame <name>` shows git blame output

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Build verification and cleanup

- [x] T020 Run `go build ./...` to verify all changes compile
- [x] T021 Add help text for new commands (diff, restore, blame, --versions) to helpCmd() in cmd/anvil/main.go

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies -- T001, T002, T003 can run in parallel
- **Foundational (Phase 2)**: Depends on T001 (TaskVersion struct) -- T004-T007 sequential
- **US1 (Phase 3)**: Depends on T004 (ReadAllVersions) -- can start after Phase 2
- **US2 (Phase 4)**: Depends on T003 (diff function) and T004 (ReadAllVersions) -- can start after Phase 2
- **US3 (Phase 5)**: Depends on T004 (WriteTaskVersion, ReadAllVersions) -- can start after Phase 2
- **US4 (Phase 6)**: Only depends on Phase 1 (task path resolution) -- can start after Phase 1
- **Polish (Phase 7)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Independent after Phase 2
- **US2 (P2)**: Independent after Phase 2
- **US3 (P3)**: Independent after Phase 2
- **US4 (P4)**: Independent after Phase 1 (no version system needed)

### Parallel Opportunities

- T001, T002, T003 can all run in parallel (different functions/files)
- After Phase 2: US1 (T008-T010), US2 (T011-T013), US3 (T014-T016), US4 (T017-T019) can all run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Auto-versioning daemon integration (T004-T007)
3. Complete Phase 3: Version history display (T008-T010)
4. **STOP and VALIDATE**: Test `anvil task history --versions` independently

### Incremental Delivery

1. Setup + Foundational -> Auto-versioning working
2. Add US1 -> Version history display -> Validate
3. Add US2 -> Diff between versions -> Validate
4. Add US3 -> Restore previous version -> Validate
5. Add US4 -> Git blame integration -> Validate
6. Polish -> Help text, build check

---

## Notes

- Total tasks: 21
- Tasks per user story: US1=3, US2=3, US3=3, US4=3, Setup=3, Foundation=4, Polish=2
- All user stories are independently testable after Phase 2
- US4 (blame) is completely independent -- only needs task file path resolution
