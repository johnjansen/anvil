# Tasks: Task Rollback

**Feature**: 022-task-rollback
**Generated**: 2026-03-01

## Implementation Strategy

**MVP Scope**: User Stories 1-3 (List, Restore, Dry-run) - Core rollback functionality
**Delivery**: Incremental - each user story is independently testable

---

## Phase 1: Setup

- [ ] T001 Create rollback storage setup (verify .anvil/runs/ directory exists)
- [ ] T002 Review existing run record implementation in internal/project/project.go for patterns

---

## Phase 2: Foundational

- [ ] T003 Add OnRollback field to Todo struct in internal/project/project.go
- [ ] T004 Create internal/project/rollback.go with RollbackEvent struct definition
- [ ] T005 Implement ListRestorePoints() function in internal/project/rollback.go
- [ ] T006 Implement GetRunRecord() function to load run metadata in internal/project/rollback.go

---

## Phase 3: User Story 1 - List Available Restore Points (P1)

**Goal**: Users can list all successful runs for a task

**Independent Test**: Run `anvil task rollback my-task` and verify table displays with run ID, timestamp, status, output size

- [ ] T007 [US1] Implement CLI rollback list subcommand in cmd/anvil/rollback.go
- [ ] T008 [US1] Add table formatting for restore points in cmd/anvil/rollback.go
- [ ] T009 [US1] Handle case when no restore points exist in cmd/anvil/rollback.go
- [ ] T010 [US1] Register rollback command in cmd/anvil/main.go under task subcommand

---

## Phase 4: User Story 2 - Restore to Previous Run (P1)

**Goal**: Users can restore files from a specific successful run

**Independent Test**: Create files, run task to change them, rollback and verify files match previous run

- [ ] T011 [US2] Implement RestoreFiles() function in internal/project/rollback.go
- [ ] T012 [US2] Add run ID validation (must be successful run) in internal/project/rollback.go
- [ ] T013 [US2] Implement file copy logic from restore point to working directory in internal/project/rollback.go
- [ ] T014 [US2] Add CLI restore subcommand with run ID argument in cmd/anvil/rollback.go

---

## Phase 5: User Story 3 - Dry-run Preview (P1)

**Goal**: Users can preview what would be restored without making changes

**Independent Test**: Run with --dry-run and verify no files are modified but preview is shown

- [ ] T015 [US3] Add --dry-run flag to rollback command in cmd/anvil/rollback.go
- [ ] T016 [US3] Implement dry-run preview logic showing files to be restored/deleted in cmd/anvil/rollback.go

---

## Phase 6: User Story 4 - Restore Specific Files (P2)

**Goal**: Users can restore only certain files

**Independent Test**: Specify --files flag and verify only those files are restored

- [ ] T017 [US4] Add --files flag to accept comma-separated file list in cmd/anvil/rollback.go
- [ ] T018 [US4] Implement file filtering logic in internal/project/rollback.go
- [ ] T019 [US4] Add error handling for files not in restore point in internal/project/rollback.go

---

## Phase 7: User Story 5 - Rollback Hooks (P3)

**Goal**: Users can run custom scripts before rollback

**Independent Test**: Configure on_rollback hook and verify it executes with correct variables

- [ ] T020 [US5] Implement runRollbackHook() function in internal/project/rollback.go
- [ ] T021 [US5] Add template variable substitution (RunID, TaskName) in internal/project/rollback.go
- [ ] T022 [US5] Execute hook before file restoration in RestoreFiles() in internal/project/rollback.go
- [ ] T023 [US5] Add error handling if hook fails (abort rollback) in internal/project/rollback.go

---

## Phase 8: Polish & Cross-Cutting

- [ ] T024 Implement RollbackEvent recording to .anvil/runs/<task-id>/rollbacks.json
- [ ] T025 Run go build ./... to verify compilation
- [ ] T026 Run go test ./... to verify tests pass

---

## Dependencies

```
T001 ──► T003
T002 ──┘

T003 ──► T004 ──► T006 ──► T007 ──► T011 ──► T015 ──► T017 ──► T020 ──► T024
                           │           │           │           │           │
                           │           │           │           │           │
                     T005 ◄──┴──── T008 ◄──┴── T012 ◄──┴── T018 ◄──┴── T021
                           │           │           │
                     T009 ◄──┘     T013 ◄──┘     T016 ◄──┘
                           │           │           │
T010 ◄─────────────────────┴── T014 ◄──┴── T019 ◄─┴── T022
                                                   │
                                                   │
                                          T023 ◄───┘

T024 ──► T025 ──► T026
```

---

## Parallel Opportunities

- **T003, T004**: Config struct and core types can be created in parallel
- **T007, T011**: CLI and restore logic are independent until integration
- **T015, T017**: Dry-run and --files flags can be implemented in parallel

---

## Summary

| Metric | Value |
|--------|-------|
| Total Tasks | 26 |
| User Story 1 (P1) | 4 tasks |
| User Story 2 (P1) | 4 tasks |
| User Story 3 (P1) | 2 tasks |
| User Story 4 (P2) | 3 tasks |
| User Story 5 (P3) | 4 tasks |
| Setup/Foundational | 6 tasks |
| Polish | 3 tasks |
| Parallelizable | ~6 tasks |

**MVP Scope**: Tasks T001-T016 - Core rollback with list, restore, and dry-run
