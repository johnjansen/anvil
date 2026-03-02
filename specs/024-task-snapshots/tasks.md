# Tasks: Task Execution Snapshots for Debugging

**Input**: Design documents from `/specs/024-task-snapshots/`
**Prerequisites**: plan.md (required), spec.md (required for user stories)

**Tests**: Not requested in spec - skip test tasks

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Foundational (Core Infrastructure)

**Purpose**: Create the snapshot package that all user stories depend on

- [ ] T001 Create snapshot package in internal/project/snapshot.go
- [ ] T002 Define Snapshot struct and SnapshotFile struct
- [ ] T003 Implement WriteSnapshot function to create snapshot directory and files
- [ ] T004 Implement ReadSnapshot function to read snapshot files
- [ ] T005 Implement GetSnapshotPath helper function
- [ ] T006 Add snapshot to existing retention pruning in internal/project/retention.go

---

## Phase 2: User Story 1 - Debug Failed Task Execution (Priority: P1) 🎯 MVP

**Goal**: Automatically capture snapshots for every task run and provide basic viewing

**Independent Test**: Run a task, verify snapshot files exist in .anvil/runs/<task-id>/<run-id>/snapshot/

### Implementation for User Story 1

- [ ] T007 [P] [US1] Add snapshot capture call in internal/daemon/daemon.go after run completion
- [ ] T008 [US1] Capture task config (frontmatter) in snapshot
- [ ] T009 [US1] Capture resolved environment variables in snapshot
- [ ] T010 [US1] Capture expanded prompt in snapshot
- [ ] T011 [US1] Capture directory listing in snapshot
- [ ] T012 [US1] Add RunRecord to snapshot
- [ ] T013 [P] [US1] Create cmd/anvil/snapshot.go with taskSnapshotCmd
- [ ] T014 [US1] Implement --run flag to view specific run
- [ ] T015 [US1] Implement --file flag to view specific file
- [ ] T016 [US1] Register snapshot command in main.go

**Checkpoint**: User Story 1 should be fully functional - snapshots captured automatically and viewable

---

## Phase 3: User Story 2 - Inspect Specific Snapshot Files (Priority: P2)

**Goal**: Allow users to focus on specific snapshot files

**Independent Test**: Request specific file and verify only that file is displayed

### Implementation for User Story 2

- [ ] T017 [US2] Add file type validation in snapshot command
- [ ] T018 [US2] Add error handling for non-existent files
- [ ] T019 [US2] Test file-specific viewing

**Checkpoint**: User Story 2 should work alongside US1 - no new infrastructure needed

---

## Phase 4: User Story 3 - Compare Two Run Snapshots (Priority: P3)

**Goal**: Allow users to diff two snapshots

**Independent Test**: Run task twice, compare snapshots, verify diff output

### Implementation for User Story 3

- [ ] T020 [P] [US3] Create cmd/anvil/snapshot_diff.go
- [ ] T021 [US3] Implement snapshotDiffCmd with --run1 and --run2 flags
- [ ] T022 [US3] Implement diff logic comparing two snapshots
- [ ] T023 [US3] Handle identical snapshots case
- [ ] T024 [US3] Handle invalid run IDs
- [ ] T025 [US3] Register snapshot-diff command in main.go

**Checkpoint**: User Story 3 complete - all snapshot features available

---

## Phase 5: User Story 4 - Automatic Snapshot Pruning (Priority: P2)

**Goal**: Automatically manage snapshot storage

**Independent Test**: Run many tasks, verify old snapshots are pruned

### Implementation for User Story 4

- [ ] T026 [US4] Extend PruneTask in retention.go to include snapshot directories
- [ ] T027 [US4] Ensure snapshots cleaned up when task deleted

**Checkpoint**: All user stories complete

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final integration and documentation

- [ ] T028 Update CLI help text for snapshot commands
- [ ] T029 Add quickstart.md documentation
- [ ] T030 Manual end-to-end testing of all user stories

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: Must complete before any user story
- **User Stories (Phase 2-5)**: All depend on Foundational phase
- **Polish (Phase 6)**: Depends on all user stories

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational - Builds on US1
- **User Story 3 (P3)**: Can start after Foundational - Independent
- **User Story 4 (P2)**: Can start after Foundational - Builds on US1

### Parallel Opportunities

- T007-T012 (snapshot capture) can run in parallel
- T013, T014, T015 can run in parallel (different files)
- T020-T022 can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational
2. Complete Phase 2: User Story 1
3. **STOP and VALIDATE**: Test snapshot capture and basic viewing
4. Deploy/demo if ready

### Incremental Delivery

1. Complete Foundational → Foundation ready
2. Add User Story 1 → Test → Deploy/Demo (MVP!)
3. Add User Story 2 → Test → Deploy/Demo
4. Add User Story 3 → Test → Deploy/Demo
5. Add User Story 4 → Test → Deploy/Demo
