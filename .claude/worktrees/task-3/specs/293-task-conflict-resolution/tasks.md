# Tasks: Task Conflict Resolution for Concurrent Dependency Failures

**Input**: Design documents from `/specs/293-task-conflict-resolution/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: The examples below include test tasks. Tests are OPTIONAL - only include them if explicitly requested in the feature specification.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `src/`, `tests/` at repository root
- Paths shown below assume single project - adjust based on plan.md structure

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create project structure per implementation plan
- [ ] T002 Initialize Go project dependencies
- [ ] T003 [P] Configure linting and formatting tools

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Setup dependency policy configuration in internal/project/project.go
- [ ] T005 [P] Extend Todo struct with DependencyPolicy and SkipDependencies fields
- [ ] T006 [P] Extend RunRequest struct with Force field in internal/daemon/daemon.go
- [ ] T007 Setup dependency failure tracking in internal/project/project.go
- [ ] T008 Configure notification hook system for dependency failures

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Force-run Tasks Despite Failed Dependencies (Priority: P1) 🎯 MVP

**Goal**: Allow users to override dependency failure checks and force-run tasks

**Independent Test**: Run a task with failed dependencies using `--force` flag and verify it executes rather than being skipped

### Tests for User Story 1 (OPTIONAL - only if tests requested) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T010 [P] [US1] Integration test for force-run functionality in tests/integration/test_force_run.go

### Implementation for User Story 1

- [ ] T011 [P] [US1] Modify workItem struct to include force context in internal/daemon/daemon.go
- [ ] T012 [US1] Update runTask function to bypass dependency checks when force flag is set in internal/daemon/daemon.go
- [ ] T013 [P] [US1] Add --force flag handling in CLI command parsing in cmd/anvil/main.go
- [ ] T014 [US1] Implement skip_dependencies configuration support in task frontmatter
- [ ] T015 [US1] Add global skip_dependencies configuration support in config

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - View Cascading Failures (Priority: P2)

**Goal**: Display cascading failure information in task queue output

**Independent Test**: Create a dependency failure and check the task queue output for cascading failure information

### Tests for User Story 2 (OPTIONAL - only if tests requested) ⚠️

- [ ] T016 [P] [US2] Integration test for cascading failure display in tests/integration/test_cascade_display.go

### Implementation for User Story 2

- [ ] T017 [P] [US2] Add dependency failure tracking to project state in internal/project/project.go
- [ ] T018 [US2] Modify task queue display to show cascading failure information in cmd/anvil/main.go
- [ ] T019 [P] [US2] Update queue command output format in cmd/anvil/main.go
- [ ] T020 [US2] Implement dependency failure count tracking in daemon logic

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Configure Dependency Policies (Priority: P3)

**Goal**: Allow users to configure how dependency failures are handled per task

**Independent Test**: Configure different dependency policies and verify task execution behavior with mixed dependency success/failure states

### Tests for User Story 3 (OPTIONAL - only if tests requested) ⚠️

- [ ] T021 [P] [US3] Integration test for dependency policies in tests/integration/test_dependency_policies.go

### Implementation for User Story 3

- [ ] T022 [P] [US3] Implement dependency_policy configuration parsing in internal/project/project.go
- [ ] T023 [US3] Update checkDependenciesMet function to respect policy configuration in internal/daemon/daemon.go
- [ ] T024 [P] [US3] Add policy validation logic in internal/project/project.go
- [ ] T025 [US3] Implement require_all and require_any policy logic

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: User Story 4 - Receive Failure Notifications (Priority: P4)

**Goal**: Notify users when dependency failures affect multiple tasks

**Independent Test**: Configure notification hooks and verify they fire when dependency failures occur

### Tests for User Story 4 (OPTIONAL - only if tests requested) ⚠️

- [ ] T026 [P] [US4] Integration test for dependency failure notifications in tests/integration/test_dependency_notifications.go

### Implementation for User Story 4

- [ ] T027 [P] [US4] Add on_dependency_failure hook configuration in internal/project/project.go
- [ ] T028 [US4] Implement notification firing logic in daemon when dependencies fail
- [ ] T029 [P] [US4] Add template variable support for notification hooks
- [ ] T030 [US4] Implement cascade count tracking for notifications

**Checkpoint**: All user stories should now be independently functional

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T031 [P] Documentation updates in docs/
- [ ] T032 Code cleanup and refactoring
- [ ] T033 Performance optimization across all stories
- [ ] T034 [P] Additional unit tests in tests/unit/
- [ ] T035 Security hardening
- [ ] T036 Run quickstart.md validation
- [ ] T037 Update README with new feature documentation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - May integrate with US1/US2 but should be independently testable
- **User Story 4 (P4)**: Can start after Foundational (Phase 2) - May integrate with previous stories but should be independently testable

### Within Each User Story

- Tests (if included) MUST be written and FAIL before implementation
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together (if tests requested):
Task: "Integration test for force-run functionality in tests/integration/test_force_run.go"

# Launch all struct extensions for User Story 1 together:
Task: "Modify workItem struct to include force context in internal/daemon/daemon.go"
Task: "Add --force flag handling in CLI command parsing in cmd/anvil/main.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Add User Story 4 → Test independently → Deploy/Demo
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
   - Developer D: User Story 4
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence