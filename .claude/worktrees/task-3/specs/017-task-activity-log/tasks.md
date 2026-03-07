# Tasks: Task Activity Log

**Input**: Design documents from `/specs/017-task-activity-log/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli.md

**Tests**: No tests explicitly requested. Test tasks omitted.

**Organization**: Tasks grouped by user story for independent implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create data structures and storage helpers

- [ ] T001 Add ActivityEntry struct, ActivitiesPath(), WriteActivity(), and ReadActivities() functions in internal/project/project.go
- [ ] T002 [P] Create cmd/anvil/activity.go with taskActivityCmd() skeleton (flag parsing, help text) per contracts/cli.md

---

## Phase 2: Foundational (Activity Recording)

**Purpose**: Add activity logging calls to all existing code paths

- [ ] T003 Add activity logging for task creation in AddTodo() in internal/project/project.go (action: "created", details: priority, schedule)
- [ ] T004 [P] Add activity logging for run completion in runTask() in internal/daemon/daemon.go (action: "run", details: run_id, exit_code, success, duration)
- [ ] T005 [P] Add activity logging for task kill in handleKill() in internal/daemon/daemon.go (action: "killed", details: graceful flag)
- [ ] T006 [P] Add activity logging for force-run in handleRun() in internal/daemon/daemon.go (action: "force-run")
- [ ] T007 [P] Add activity logging for pause in taskPauseCmd() in cmd/anvil/main.go (action: "paused")
- [ ] T008 [P] Add activity logging for resume in taskResumeCmd() in cmd/anvil/main.go (action: "resumed")
- [ ] T009 [P] Add activity logging for unlock in taskUnlockCmd() in cmd/anvil/main.go (action: "unlocked")
- [ ] T010 Add activity logging for edit in taskEditCmd() in cmd/anvil/main.go (action: "edited", details: changed fields with old/new values)

**Checkpoint**: All 7+ activity types are being recorded to .anvil/activities/<task-id>.jsonl

---

## Phase 3: User Story 1 - View Task Activity History (Priority: P1) MVP

**Goal**: anvil task activity <name> shows complete activity history

**Independent Test**: Create a task, run it, pause it, view activity — verify all events appear

### Implementation for User Story 1

- [ ] T011 [US1] Implement display logic in taskActivityCmd() in cmd/anvil/activity.go: load activities, reverse chronological order, format table output (TIMESTAMP, ACTION, DETAILS columns)
- [ ] T012 [US1] Add "activity" case to taskCmd() dispatcher in cmd/anvil/main.go
- [ ] T013 [US1] Add --limit flag support in cmd/anvil/activity.go (default 100)

**Checkpoint**: anvil task activity <name> shows formatted activity table

---

## Phase 4: User Story 2 - Filter Activity (Priority: P2)

**Goal**: Filter activity by type and date

**Independent Test**: Use --type run and --since flags to narrow results

### Implementation for User Story 2

- [ ] T014 [US2] Add --type filter in cmd/anvil/activity.go: validate against known types, filter entries before display
- [ ] T015 [US2] Add --since filter in cmd/anvil/activity.go: parse YYYY-MM-DD date, filter entries after that date

**Checkpoint**: Filtering by type and date works correctly

---

## Phase 5: User Story 3 - Export Activity (Priority: P3)

**Goal**: Export activity to JSON file or stdout

**Independent Test**: Export activity with --export flag, verify file content

### Implementation for User Story 3

- [ ] T016 [US3] Add --export flag in cmd/anvil/activity.go: write filtered entries to JSON file
- [ ] T017 [US3] Add --json flag in cmd/anvil/activity.go: output JSON to stdout

**Checkpoint**: Export and JSON output work with all filters

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T018 Add help text for "activity" subcommand in the task help output in cmd/anvil/main.go
- [ ] T019 Handle edge cases: no activity file exists, empty results, invalid flags per contracts/cli.md error cases

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies
- **Phase 2 (Foundational)**: Depends on T001 (ActivityEntry struct + WriteActivity)
- **Phase 3 (US1)**: Depends on T001 (ReadActivities) and T002 (activity.go skeleton)
- **Phase 4 (US2)**: Depends on T011 (display logic)
- **Phase 5 (US3)**: Depends on T011 (display logic)
- **Phase 6 (Polish)**: Depends on T011

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- T003-T010 can largely run in parallel (different functions, but all depend on T001)
- T014 and T015 can run in parallel (independent filter types)
- T016 and T017 can run in parallel (independent output modes)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T010) — recording all events
3. Complete Phase 3: User Story 1 (T011-T013) — viewing events
4. **STOP and VALIDATE**: Run anvil task activity <name> and verify events appear

### Incremental Delivery

1. Setup + Foundational → Events being recorded
2. User Story 1 → View activity (MVP!)
3. User Story 2 → Filter by type/date
4. User Story 3 → Export/JSON
5. Polish → Help text + edge cases

---

## Notes

- Total tasks: 19
- US1: 3 tasks, US2: 2 tasks, US3: 2 tasks, Setup: 2 tasks, Foundation: 8 tasks, Polish: 2 tasks
- All recording tasks (Phase 2) modify existing files; display/filter/export tasks create new file (activity.go)
