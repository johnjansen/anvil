# Tasks: Task Execution Timeout Extension

**Input**: Design documents from `/specs/005-timeout-extension/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Tests are included — this is a Go project with existing test patterns.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, timer replacement, and extension helpers that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T001 [P] Add `AutoExtendConfig` struct (Enabled bool, MaxExtensions int, ExtensionDuration time.Duration) to `internal/project/project.go` and add `AutoExtend AutoExtendConfig`, `OnTimeoutWarning string` fields to the `Todo` struct
- [x] T002 [P] Add timeout extension fields to `RunRecord` struct in `internal/project/project.go`: `OriginalTimeout time.Duration`, `FinalTimeout time.Duration`, `ExtensionCount int`, `TotalExtended time.Duration`, `AutoExtensions int` (all with `json` tags and `omitempty`)
- [x] T003 Parse `auto_extend` and `on_timeout_warning` from task frontmatter YAML in `internal/project/project.go` (add `AutoExtend *struct{ Enabled bool; MaxExtensions int; ExtensionDuration string }` and `OnTimeoutWarning string` to `fmData` struct, parse `extension_duration` via `time.ParseDuration`, map to `Todo.AutoExtend` and `Todo.OnTimeoutWarning`)
- [x] T004 Add timeout state fields to `RunningTask` struct in `internal/daemon/daemon.go`: `OriginalTimeout time.Duration`, `CurrentDeadline time.Time`, `ExtensionCount int`, `TotalExtended time.Duration`, `AutoExtensions int`, `TimeoutTimer *time.Timer`, `WarningTimer *time.Timer`, `WarningFired bool`, `LastCheckpointTime time.Time`, `AutoExtendConfig project.AutoExtendConfig`
- [x] T005 Replace `context.WithTimeout` with `context.WithCancel` + `time.AfterFunc` timer in the `runTask` function of `internal/daemon/daemon.go` — change the timeout context creation (around line 611) to use `context.WithCancel(context.Background())` and create a `time.AfterFunc(timeout, func() { cancel() })` timer, store the timer and original timeout in `RunningTask`
- [x] T006 Create `internal/daemon/timeout.go` with helper functions: `extendTimeout(task *RunningTask, duration time.Duration, absolute bool) (newDeadline time.Time, remaining time.Duration, err error)` that stops the old timer and creates a new `time.AfterFunc` with the updated deadline, and `setupWarningTimer(d *Daemon, task *RunningTask, todo project.Todo, projectPath string)` that creates a warning timer for `on_timeout_warning` hook execution
- [x] T007 Create `internal/daemon/timeout_test.go` with tests for `extendTimeout`: extend by duration (additive), extend with absolute flag, extend when no timeout configured (error), and verify timer replacement works correctly

**Checkpoint**: Foundation ready — RunningTask tracks timeout state, timer-based timeout replaces context deadline, extension helper tested

---

## Phase 3: User Story 1 — Manual Timeout Extension (Priority: P1) MVP

**Goal**: Users can run `anvil task extend-timeout <name> <duration>` to extend a running task's timeout. The daemon receives the request and updates the task's deadline via the timer.

**Independent Test**: Run a task with a short timeout, run `anvil task extend-timeout <name> 5m`, verify the task continues past the original timeout.

### Implementation for User Story 1

- [x] T008 [US1] Add `/extend-timeout` HTTP handler in `internal/daemon/daemon.go` that accepts JSON `{task_key, duration, absolute}`, finds the running task by key/name, calls `extendTimeout`, and returns JSON `{ok, new_deadline, remaining, extension_count}` — follow the existing `/kill` handler pattern (lines 1463-1501)
- [x] T009 [US1] Add `SendExtendTimeoutRequest(projectPath, taskName, duration string, absolute bool) error` function in `internal/daemon/daemon.go` (near existing `SendRunRequest`, `SendKillRequest`) that sends HTTP POST to the daemon socket `/extend-timeout` endpoint
- [x] T010 [US1] Add `taskExtendTimeoutCmd` function in `cmd/anvil/main.go` that parses `<name> <duration> [--absolute]` args, calls `daemon.SendExtendTimeoutRequest`, and displays the result
- [x] T011 [US1] Wire `taskExtendTimeoutCmd` into the task subcommand dispatch in `cmd/anvil/main.go` (add `"extend-timeout"` case in the task command switch)
- [x] T012 [US1] Register the `/extend-timeout` handler in the daemon's HTTP mux setup (around line 1208-1222 in `internal/daemon/daemon.go`)

**Checkpoint**: Manual timeout extension works — users can extend running tasks from the CLI

---

## Phase 4: User Story 2 — Timeout Visibility (Priority: P2)

**Goal**: `anvil task timeout`, `anvil task get`, and `anvil ps` show extension info (original timeout, current deadline, extensions used, remaining time).

**Independent Test**: Extend a running task, then verify `anvil task timeout <name>` shows original timeout, current timeout, extension count, and remaining time.

### Implementation for User Story 2

- [x] T013 [P] [US2] Add `ExtensionCount int`, `OriginalTimeout string`, `TotalExtended string` fields to `TaskInfo` struct in `internal/daemon/daemon.go` and populate them in the `handlePs` handler from `RunningTask` extension state
- [x] T014 [US2] Update the `handleTimeout` handler in `internal/daemon/daemon.go` to include extension info in its response — add extension count, original timeout, and total extended to the timeout data returned
- [x] T015 [US2] Update `taskTimeoutCmd` in `cmd/anvil/main.go` to display extension info — add "EXTENSIONS" column showing "Nx +Ym" format (e.g., "2x +30m")
- [x] T016 [US2] Update `taskGetCmd` in `cmd/anvil/main.go` to display timeout extension info when a task is running and has been extended — show "Timeout: 30m (original), 45m (current), 1 extension"
- [x] T017 [US2] Update `anvil ps` output formatting in `cmd/anvil/main.go` to show extension info in the timeout column — display "Xm left (Nx extended)" when extensions have been applied

**Checkpoint**: Timeout visibility works — users can see extension info in all relevant CLI commands

---

## Phase 5: User Story 3 — Automatic Timeout Extension (Priority: P2)

**Goal**: Tasks with `auto_extend` config automatically extend their timeout when a checkpoint is detected within the warning window (5 minutes before deadline), up to `max_extensions` times.

**Independent Test**: Create a task with `auto_extend: {enabled: true, max_extensions: 3, extension_duration: 15m}` and a short timeout. Emit checkpoints and verify the timeout extends automatically.

### Implementation for User Story 3

- [x] T018 [US3] Add auto-extend logic to the checkpoint callback in `internal/daemon/daemon.go` — in the checkpoint handler (around line 830), when a checkpoint is received, check if auto-extend is enabled, the task is within the warning window (5 minutes of deadline), and extensions remaining > 0, then call `extendTimeout` and increment `AutoExtensions`
- [x] T019 [US3] Store `AutoExtendConfig` from `Todo` into `RunningTask` at task start in `internal/daemon/daemon.go` — when populating the RunningTask struct (around line 630), copy `todo.AutoExtend` into `RunningTask.AutoExtendConfig`
- [x] T020 [US3] Update `RunningTask.LastCheckpointTime` in the checkpoint callback in `internal/daemon/daemon.go` to track when the most recent checkpoint was emitted
- [x] T021 [US3] Add test in `internal/daemon/timeout_test.go` for auto-extend: verify that `extendTimeout` is called when checkpoint arrives within warning window, verify it stops after max_extensions, verify it does NOT extend when checkpoint is outside warning window

**Checkpoint**: Auto-extend works — tasks with checkpoints automatically get more time up to the configured limit

---

## Phase 6: User Story 4 — Timeout Warning Hook (Priority: P3)

**Goal**: `on_timeout_warning` hook fires when a task approaches its deadline (5 minutes before), with environment variables for the task name, remaining time, and extension info.

**Independent Test**: Create a task with `on_timeout_warning` set to a test command, run with short timeout, verify the hook fires with correct environment variables.

### Implementation for User Story 4

- [x] T022 [US4] Implement `setupWarningTimer` in `internal/daemon/timeout.go` that creates a `time.AfterFunc` timer to fire `on_timeout_warning` hook when the task enters the warning window (deadline minus 5 minutes) — the timer should execute the hook command as a goroutine with 60s timeout following the existing `runHook` pattern
- [x] T023 [US4] Add `runTimeoutWarningHook` method to Daemon in `internal/daemon/daemon.go` that executes `todo.OnTimeoutWarning` as a shell command with environment variables: `ANVIL_TASK_NAME`, `ANVIL_PROJECT`, `ANVIL_TIMEOUT_REMAINING`, `ANVIL_TIMEOUT_ORIGINAL`, `ANVIL_EXTENSIONS_USED`, `ANVIL_AUTO_EXTEND_REMAINING`
- [x] T024 [US4] Call `setupWarningTimer` at task start in `runTask` in `internal/daemon/daemon.go` when `todo.OnTimeoutWarning` is set — also reschedule the warning timer after each timeout extension (both manual and auto)
- [x] T025 [US4] Set `WarningFired = false` after each extension in `extendTimeout` in `internal/daemon/timeout.go` so the warning fires again for the new deadline

**Checkpoint**: Timeout warning hooks fire — users get alerted when tasks approach their deadline

---

## Phase 7: Persistence and Polish

**Purpose**: Persist extension data in RunRecord, update help text, final validation

- [x] T026 Update the `RunRecord` construction in `runTask` completion code in `internal/daemon/daemon.go` to populate `OriginalTimeout`, `FinalTimeout`, `ExtensionCount`, `TotalExtended`, and `AutoExtensions` from `RunningTask` state
- [x] T027 Stop `TimeoutTimer` and `WarningTimer` in task cleanup code in `internal/daemon/daemon.go` when a task completes (before writing RunRecord) to prevent timer leaks
- [x] T028 Update help text for `anvil task` in `cmd/anvil/main.go` to list the new `extend-timeout` subcommand
- [x] T029 Run `go test ./...` and `go build ./cmd/anvil/` to verify all tests pass and project compiles

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — can start immediately
- **US1 (Phase 3)**: Depends on Phase 2 (T001-T007) — needs timer replacement and extendTimeout helper
- **US2 (Phase 4)**: Depends on US1 (T008, T012) — needs extension state populated by handler
- **US3 (Phase 5)**: Depends on Phase 2 (T005, T006) — needs timer and extendTimeout; can run in parallel with US1
- **US4 (Phase 6)**: Depends on Phase 2 (T006) — needs setupWarningTimer helper
- **Polish (Phase 7)**: Depends on all user stories

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 — No dependencies on other stories
- **US2 (P2)**: Depends on US1 — needs extend-timeout handler to populate extension state
- **US3 (P2)**: Can start after Phase 2 — Independent of US1 (auto-extend uses same helpers)
- **US4 (P3)**: Can start after Phase 2 — Independent of US1/US3

### Parallel Opportunities

- T001 and T002 can run in parallel (different struct additions in same file, but adjacent locations)
- T013 can run in parallel with other US2 tasks (modifies TaskInfo struct, not cmd)
- US3 and US4 can proceed in parallel after Phase 2 (independent mechanisms)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (T001-T007)
2. Complete Phase 3: User Story 1 (T008-T012)
3. **STOP and VALIDATE**: Test manual timeout extension independently
4. Deploy/demo if ready

### Incremental Delivery

1. Phase 2 → Foundation ready (timer replacement, helpers)
2. US1 → Manual extend-timeout works (MVP!)
3. US2 → Timeout visibility in all CLI commands
4. US3 → Auto-extend via checkpoint detection
5. US4 → Timeout warning hooks
6. Polish → Persistence, help text, final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- The critical architectural change is T005 (timer replacement) — this must be carefully implemented to avoid breaking existing timeout behavior
- Auto-extend (US3) and warning hooks (US4) share the warning window concept (5 minutes before deadline)
- Timer cleanup (T027) is essential to prevent goroutine/timer leaks
