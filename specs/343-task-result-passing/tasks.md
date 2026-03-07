# Tasks: Task Result Passing

**Input**: Design documents from `/specs/343-task-result-passing/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new files or projects needed — this feature extends existing code. Setup phase ensures the foundational types are in place.

- [ ] T001 Add `CaptureOutput` field to `Todo` struct and parse from frontmatter `capture_output` in `internal/project/project.go`
- [ ] T002 Add `ResultData` field to `RunRecord` struct in `internal/project/project.go`

---

## Phase 2: Foundational (Result Capture Infrastructure)

**Purpose**: Core result capture mechanism that all user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T003 Add `resultPrefix` constant (`##anvil:result `) and `onResult` callback to `statusWriter` in `internal/runner/runner.go`
- [ ] T004 Wire `onResult` callback in daemon task execution to capture last result line and store in `RunRecord.ResultData` in `internal/daemon/daemon.go`
- [ ] T005 Add 1MB size limit check for result data with warning log in `internal/daemon/daemon.go`

**Checkpoint**: Result capture infrastructure ready — tasks with `capture_output: true` now store results in run records

---

## Phase 3: User Story 1 - Capture and Pass Results to Dependent Task (Priority: P1) 🎯 MVP

**Goal**: Enable tasks to capture output and pass it to dependent tasks via environment variable

**Independent Test**: Create two tasks where first captures `##anvil:result` and second reads `ANVIL_DEPENDENCY_RESULTS` env var

### Implementation for User Story 1

- [ ] T006 [US1] Build `CollectDependencyResults` function that reads `ResultData` from each dependency's latest `RunRecord` and returns a `map[string]json.RawMessage` in `internal/project/dependencies.go`
- [ ] T007 [US1] Inject `ANVIL_DEPENDENCY_RESULTS` env var (JSON-serialized dependency results map) into dependent task's merged env in `internal/daemon/daemon.go`
- [ ] T008 [US1] Add unit test for `CollectDependencyResults` with local dependencies, missing results, and null handling in `internal/project/dependencies_test.go`
- [ ] T009 [US1] Add unit test for result capture via `statusWriter` with `##anvil:result` prefix in `internal/runner/runner_test.go`

**Checkpoint**: User Story 1 fully functional — tasks capture results and pass them to dependents via env var

---

## Phase 4: User Story 2 - View Captured Results via CLI (Priority: P2)

**Goal**: Provide `anvil task results` command for inspecting captured output

**Independent Test**: Run `anvil task results <task>` and `anvil task results <task> --preview` to view stored and projected results

### Implementation for User Story 2

- [ ] T010 [US2] Add `task results` subcommand with `--preview`, `--run`, and `--json` flags in `cmd/anvil/task_results.go`
- [ ] T011 [US2] Implement result display logic: read latest `RunRecord.ResultData`, format output, handle no-results case in `cmd/anvil/task_results.go`
- [ ] T012 [US2] Implement `--preview` mode: call `CollectDependencyResults` for the task's dependencies and display projected results in `cmd/anvil/task_results.go`

**Checkpoint**: CLI visibility complete — users can inspect results and preview dependency data

---

## Phase 5: User Story 3 - Template Access to Dependency Results (Priority: P2)

**Goal**: Make dependency results available as Go template variables in task body

**Independent Test**: Create a dependent task using `{{ index .DependencyResults "fetch-data" }}` and verify template renders with actual values

### Implementation for User Story 3

- [ ] T013 [US3] Add `DependencyResults` field to the template context used when rendering task body in `internal/daemon/daemon.go`
- [ ] T014 [US3] Populate `DependencyResults` template context from collected dependency results before task body rendering in `internal/daemon/daemon.go`

**Checkpoint**: Template variables work — dependent tasks can reference `{{ .DependencyResults }}` in their body

---

## Phase 6: User Story 4 - Cross-Project Result Passing (Priority: P3)

**Goal**: Extend result passing to work across project boundaries

**Independent Test**: Create tasks in two projects with cross-project dependency and verify results flow via `ANVIL_DEPENDENCY_RESULTS`

### Implementation for User Story 4

- [ ] T015 [US4] Extend `CollectDependencyResults` to handle cross-project dependencies using existing `ResolveDependencyRunRecord` in `internal/project/dependencies.go`
- [ ] T016 [US4] Add unit test for cross-project result collection in `internal/project/dependencies_test.go`

**Checkpoint**: Cross-project result passing works with same reliability as local

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases and final validation

- [ ] T017 Handle multiple `##anvil:result` lines (last one wins) — verify in `statusWriter` logic in `internal/runner/runner.go`
- [ ] T018 Handle invalid JSON in `##anvil:result` — store raw string as-is in `internal/daemon/daemon.go`
- [ ] T019 Run quickstart.md validation — create sample tasks and verify end-to-end flow

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001, T002) — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion
- **User Story 2 (Phase 4)**: Depends on Phase 3 (needs `CollectDependencyResults` from T006)
- **User Story 3 (Phase 5)**: Depends on Phase 3 (needs dependency results collection)
- **User Story 4 (Phase 6)**: Depends on Phase 3 (extends `CollectDependencyResults`)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational — no dependencies on other stories
- **User Story 2 (P2)**: Depends on US1's `CollectDependencyResults` function (T006)
- **User Story 3 (P2)**: Depends on US1's dependency results collection (T006, T007)
- **User Story 4 (P3)**: Depends on US1's `CollectDependencyResults` function (T006)

### Parallel Opportunities

- T001 and T002 can run in parallel (different struct modifications)
- T003 and T004 are sequential (T004 wires T003's callback)
- T008 and T009 can run in parallel (different test files)
- US2 (Phase 4), US3 (Phase 5), and US4 (Phase 6) can run in parallel after US1 completes

---

## Parallel Example: User Story 1

```bash
# After foundational phase, these can run in parallel:
Task T008: "Unit test for CollectDependencyResults in internal/project/dependencies_test.go"
Task T009: "Unit test for result capture via statusWriter in internal/runner/runner_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T005)
3. Complete Phase 3: User Story 1 (T006-T009)
4. **STOP and VALIDATE**: Test result passing between two dependent tasks
5. Deploy if ready — core value is delivered

### Incremental Delivery

1. Setup + Foundational → Result capture works
2. Add User Story 1 → Tasks pass results via env var (MVP!)
3. Add User Story 2 → CLI visibility for debugging
4. Add User Story 3 → Template variables for convenience
5. Add User Story 4 → Cross-project support
6. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- This feature follows the established `##anvil:checkpoint` pattern closely — result capture mirrors checkpoint capture
- All changes extend existing files; no new packages needed (except `cmd/anvil/task_results.go`)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
