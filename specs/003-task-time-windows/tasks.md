# Tasks: Task Execution Time Windows

**Input**: Design documents from `/specs/003-task-time-windows/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Tests are included — this is a Go project with existing test patterns.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: No project initialization needed — this is an existing Go project. Skip to foundational.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and helpers that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T001 Add `AllowedWindow` struct (Start, End, Days string fields) to `internal/project/project.go` and add `AllowedWindow` field to the `Todo` struct
- [x] T002 Add `QuietHoursConfig` struct (Enabled bool, Start/End string, ExcludePriority int) to `internal/config/config.go` and add `QuietHours` field to the `Config` struct
- [x] T003 Parse `allowed_window` from task frontmatter YAML in the frontmatter parsing section of `internal/project/project.go` (map `allowed_window.start`, `allowed_window.end`, `allowed_window.days` to `Todo.AllowedWindow`)
- [x] T004 Create `internal/daemon/timewindow.go` with helper functions: `parseHHMM(s string) (hour, minute int, err error)`, `parseDays(s string) (map[int]bool, error)` supporting range "1-5", list "1,3,5", and combined "1-5,0" notation, and `isInTimeWindow(now time.Time, start, end, days string) bool` that handles midnight-spanning windows
- [x] T005 Create `internal/daemon/timewindow_test.go` with tests for: normal window (09:00-18:00), midnight-spanning window (22:00-06:00), day filtering (weekdays only, weekends only), empty/missing fields (all-pass), edge cases (exactly at boundary times), invalid input handling

**Checkpoint**: Foundation ready — AllowedWindow and QuietHours types exist, window evaluation logic is tested

---

## Phase 3: User Story 1 — Per-Task Allowed Window (Priority: P1) MVP

**Goal**: Tasks with `allowed_window` in frontmatter are only executed when the current time falls within the specified window

**Independent Test**: Create a task with `allowed_window`, verify it is skipped when outside the window and executes when inside

### Implementation for User Story 1

- [x] T006 [US1] Add `isTaskInWindow(t project.Todo, now time.Time) bool` function in `internal/daemon/timewindow.go` that checks `t.AllowedWindow` fields — returns true if no window configured or if current time is within window
- [x] T007 [US1] Add time window check in the daemon dispatch loop in `internal/daemon/daemon.go` tick function — after dependency checks and before stopped-task check: if `!isTaskInWindow(pt.todo, now)` then skip (continue) with no error
- [x] T008 [US1] Add test in `internal/daemon/timewindow_test.go` for `isTaskInWindow` with Todo structs containing various AllowedWindow configurations
- [x] T009 [US1] Add integration test in `internal/daemon/timewindow_test.go` verifying that a Todo with AllowedWindow set and current time outside window returns false, and inside window returns true

**Checkpoint**: Per-task time windows work — tasks are skipped outside their window, execute inside it

---

## Phase 4: User Story 2 — Global Quiet Hours (Priority: P2)

**Goal**: Global quiet hours block non-exempt tasks during configured hours

**Independent Test**: Configure quiet hours in config, verify p2 tasks are blocked during quiet hours while p0 tasks execute

### Implementation for User Story 2

- [x] T010 [US2] Add `isInQuietHours(now time.Time, cfg config.QuietHoursConfig, taskPriority int) bool` function in `internal/daemon/timewindow.go` — returns true (blocked) if quiet hours enabled, time is within quiet window, and task priority > exclude_priority
- [x] T011 [US2] Add quiet hours check in the daemon dispatch loop in `internal/daemon/daemon.go` tick function — immediately after the time window check (T007): if `isInQuietHours(now, d.config.QuietHours, pt.todo.Priority)` then skip (continue)
- [x] T012 [US2] Add tests in `internal/daemon/timewindow_test.go` for `isInQuietHours`: quiet hours disabled (pass), within quiet hours with low priority (blocked), within quiet hours with exempt priority (pass), outside quiet hours (pass), midnight-spanning quiet hours

**Checkpoint**: Global quiet hours work — non-exempt tasks are blocked during configured hours

---

## Phase 5: User Story 3 — Force Run Bypassing Windows (Priority: P2)

**Goal**: `anvil task run <name> --force` bypasses all time window and quiet hour restrictions

**Independent Test**: Run `anvil task run <name> --force` during a restricted window and verify the task executes

### Implementation for User Story 3

- [x] T013 [US3] Add `Force bool` field to the `RunRequest` struct in `internal/daemon/daemon.go` and update `SendRunRequest` function signature in daemon client code to accept and pass a `force` parameter
- [x] T014 [US3] Update `handleRun` in `internal/daemon/daemon.go` to read `req.Force` and set a `forceWindow` flag on the dispatched todo copy (add `ForceWindow bool` field to `Todo` struct in `internal/project/project.go`)
- [x] T015 [US3] Update the time window and quiet hours checks in the dispatch loop (T007, T011) to skip evaluation when `pt.todo.ForceWindow` is true
- [x] T016 [US3] Add `--force` flag parsing to `taskRunCmd` in `cmd/anvil/main.go` — parse the flag from args and pass it through to `SendRunRequest`

**Checkpoint**: Force-run works — tasks execute immediately regardless of window restrictions when `--force` is used

---

## Phase 6: User Story 4 — View Next Allowed Run Time (Priority: P3)

**Goal**: `anvil task next <name> --verbose` shows the next valid execution time considering both cron and window constraints

**Independent Test**: Run `anvil task next <name>` for a windowed task and verify it shows a time that satisfies both cron and window

### Implementation for User Story 4

- [x] T017 [US4] Add `nextAllowedRun(schedule string, window project.AllowedWindow, quietHours config.QuietHoursConfig, priority int, after time.Time) (time.Time, error)` function in `internal/daemon/timewindow.go` — iterates `cron.Parser.Next()` checking each candidate against window and quiet hours, max 366 days forward
- [x] T018 [US4] Add `taskNextCmd` function in `cmd/anvil/main.go` that loads the task, calls `nextAllowedRun`, and prints the result — with `--verbose` flag showing window details (active constraints, next cron match vs next allowed match)
- [x] T019 [US4] Wire `taskNextCmd` into the task subcommand dispatch in `cmd/anvil/main.go` (add `"next"` case)
- [x] T020 [US4] Add test in `internal/daemon/timewindow_test.go` for `nextAllowedRun` verifying it skips cron matches outside the window and returns the first valid one

**Checkpoint**: `anvil task next` shows correct next execution time considering all constraints

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Help text, documentation, and final validation

- [x] T021 Update help text for `anvil task run` in `cmd/anvil/main.go` to document the `--force` flag
- [x] T022 Update help text for `anvil task` in `cmd/anvil/main.go` to list the new `next` subcommand
- [x] T023 Run `go test ./...` and `go build ./cmd/anvil/` to verify all tests pass and project compiles

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — can start immediately
- **US1 (Phase 3)**: Depends on Phase 2 (T001-T005)
- **US2 (Phase 4)**: Depends on Phase 2 (T001-T005); can run in parallel with US1
- **US3 (Phase 5)**: Depends on US1 (T007) and US2 (T011) — needs window checks to exist before adding bypass
- **US4 (Phase 6)**: Depends on Phase 2 (T004) for window evaluation helpers; can start after Phase 2
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 — No dependencies on other stories
- **US2 (P2)**: Can start after Phase 2 — Independent of US1
- **US3 (P2)**: Depends on US1 and US2 — needs both window checks in dispatch loop
- **US4 (P3)**: Can start after Phase 2 — Uses helpers independently of dispatch integration

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- T003 depends on T001 (same file)
- T004 and T005 can run in parallel with T003 (different files)
- US1 and US2 can proceed in parallel after Phase 2
- US4 can proceed in parallel with US1/US2 (only needs helpers from Phase 2)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (T001-T005)
2. Complete Phase 3: User Story 1 (T006-T009)
3. **STOP and VALIDATE**: Test per-task window independently
4. Deploy/demo if ready

### Incremental Delivery

1. Phase 2 → Foundation ready
2. US1 → Per-task windows work (MVP!)
3. US2 → Global quiet hours work
4. US3 → Force-run bypass works
5. US4 → Next-run visibility works
6. Polish → Help text, final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
