# Tasks: Cross-Project Dependency Status in Task Queue

**Input**: Design documents from `/specs/265-cross-project-queue/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Not explicitly requested. Test tasks omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: No new project setup needed — this feature extends existing files only.

_No setup tasks required. All changes are to existing files._

---

## Phase 2: Foundational (Data Structures)

**Purpose**: Add the new data structures that all user stories depend on.

- [ ] T001 Add `CrossDepStatus` struct to `internal/daemon/daemon.go` with fields: Project (string), Task (string), Status (string), Blocking (bool), LastRun (string, JSON timestamp)
- [ ] T002 Extend `TaskQueueInfo` struct in `internal/daemon/daemon.go` to add `CrossDeps []CrossDepStatus` field with json tag `cross_deps,omitempty`

**Checkpoint**: Data structures ready — user story implementation can now begin.

---

## Phase 3: User Story 1 - View cross-project blocking dependencies (Priority: P1) 🎯 MVP

**Goal**: When a user runs `anvil task queue`, tasks with cross-project dependencies show their dependency status (project name, task name, last run result, blocking/satisfied).

**Independent Test**: Create a task with a `depends_on: ["other-project:some-task"]` entry. Run `anvil task queue`. Verify the output shows the cross-project dependency with its status in both table and JSON output.

### Implementation for User Story 1

- [ ] T003 [US1] Update `handleQueue` in `internal/daemon/daemon.go` to resolve cross-project dependencies for each pending/skipped task: iterate `todo.DependsOn`, call `project.ParseDependency()`, filter for non-local deps, call `project.ResolveDependencyRunRecord()` to get last run status, populate `CrossDeps` field on `TaskQueueInfo`. Handle errors gracefully: set status to "unknown project" or "task not found" or "no runs" as appropriate.
- [ ] T004 [US1] Add CROSS-DEPS column to table output in `taskQueueCmd` in `cmd/anvil/task_queue.go`: for each task, render cross-project deps as compact `project:task(status)` entries. Show `-` when no cross-project deps exist.
- [ ] T005 [US1] Verify JSON output in `taskQueueCmd` automatically includes `cross_deps` field via existing `json.MarshalIndent(tasks)` — no additional changes needed since the struct field has the json tag.

**Checkpoint**: `anvil task queue` shows cross-project dependency status in both table and JSON output.

---

## Phase 4: User Story 2 - Filter queue with --all flag (Priority: P2)

**Goal**: `anvil task queue --all` includes cross-project dependency tasks as separate entries in the queue, giving a complete picture of blocking/satisfied dependencies across projects.

**Independent Test**: Create tasks with cross-project dependencies. Run `anvil task queue --all`. Verify that cross-project tasks appear as distinct entries with `[project-name]` prefix, showing their last run status.

### Implementation for User Story 2

- [ ] T006 [US2] Add `--all` flag parsing to `taskQueueCmd` in `cmd/anvil/task_queue.go`: detect `--all` or `-a` in args, set `showAll` boolean.
- [ ] T007 [US2] Add `IsCrossProject` bool field to `TaskQueueInfo` in `internal/daemon/daemon.go` with json tag `is_cross_project,omitempty`.
- [ ] T008 [US2] Update `handleQueue` in `internal/daemon/daemon.go` to accept an `?all=true` query parameter. When set, collect all unique cross-project dependencies across all tasks, resolve each to a `TaskQueueInfo` entry with `IsCrossProject: true`, project name in `Project` field, task name in `Name` field, last run status in `Status` field, and append to the result list.
- [ ] T009 [US2] Update `SendQueueRequest` in `internal/daemon/daemon.go` to accept and pass through the `all` query parameter.
- [ ] T010 [US2] Update table rendering in `taskQueueCmd` in `cmd/anvil/task_queue.go` to prefix cross-project entries with `[project-name]` and show `-` for priority.

**Checkpoint**: `anvil task queue --all` shows cross-project dependency tasks as separate queue entries.

---

## Phase 5: User Story 3 - Cross-project dependency info in JSON output (Priority: P2)

**Goal**: JSON output from `anvil task queue --json` includes complete cross-project dependency information.

**Independent Test**: Run `anvil task queue --json` with tasks that have cross-project deps. Verify JSON includes `cross_deps` array with project, task, status, blocking, and last_run fields.

### Implementation for User Story 3

- [ ] T011 [US3] Verify and validate that JSON output from `--json` and `--json --all` correctly serializes `CrossDeps` and `IsCrossProject` fields — this should work automatically from T002/T007 struct changes. Add any missing omitempty tags or formatting if needed in `internal/daemon/daemon.go`.

**Checkpoint**: JSON output is complete and consumable by scripts and dashboards.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases and robustness improvements.

- [ ] T012 Handle edge case in `handleQueue` in `internal/daemon/daemon.go`: when a cross-project dependency references an unknown project (not in watched directory), set CrossDepStatus.Status to "unknown project" and Blocking to true.
- [ ] T013 Handle edge case in `handleQueue` in `internal/daemon/daemon.go`: when a cross-project dependency references a task that doesn't exist in the target project, set CrossDepStatus.Status to "task not found" and Blocking to true.
- [ ] T014 Handle edge case in `handleQueue` in `internal/daemon/daemon.go`: when a cross-project dependency task has never been run (no run records), set CrossDepStatus.Status to "no runs" and Blocking to true.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — can start immediately
- **User Story 1 (Phase 3)**: Depends on Phase 2 (T001, T002)
- **User Story 2 (Phase 4)**: Depends on Phase 2 (T001, T002). Can run in parallel with US1.
- **User Story 3 (Phase 5)**: Depends on Phase 2 (T001, T002) and at least T003 from US1.
- **Polish (Phase 6)**: Can be done alongside or after US1 (T003 contains the main resolution logic).

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 2 — no dependencies on other stories
- **User Story 2 (P2)**: Can start after Phase 2 — independent from US1 (adds --all flag path)
- **User Story 3 (P2)**: Depends on US1 (T003) since JSON output relies on the daemon populating CrossDeps

### Parallel Opportunities

- T001 and T002 can be done together (same file, but closely related struct changes)
- T004 and T006 touch the same file (`task_queue.go`) — do sequentially
- T003 and T006-T010 work on different aspects of daemon.go — can overlap but recommend sequential to avoid conflicts
- T012, T013, T014 are all edge cases in the same function — do sequentially

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (T001-T002)
2. Complete Phase 3: User Story 1 (T003-T005)
3. **STOP and VALIDATE**: Run `anvil task queue` with cross-project deps and verify output
4. Deploy if ready

### Incremental Delivery

1. T001-T002 → Data structures ready
2. T003-T005 → Cross-project deps visible in queue (MVP!)
3. T006-T010 → --all flag adds cross-project entries
4. T011 → JSON output validated
5. T012-T014 → Edge cases hardened

---

## Notes

- All changes are to existing files — no new files created
- Primary files modified: `internal/daemon/daemon.go` and `cmd/anvil/task_queue.go`
- The existing `ResolveDependencyRunRecord` function from #259 does the heavy lifting
- Edge case handling (T012-T014) is partially addressed in T003 but broken out for clarity
