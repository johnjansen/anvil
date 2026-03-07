# Tasks: Task SLA Tracking

**Input**: Design documents from `/specs/004-task-sla-tracking/`
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

- [x] T001 [P] Add `SLAConfig` struct (MaxDelay time.Duration, Strict bool) to `internal/project/project.go` and add `SLA SLAConfig` and `OnSLAViolation string` fields to the `Todo` struct
- [x] T002 [P] Add `SLAGlobalConfig` struct (DefaultMaxDelay string) to `internal/config/config.go` and add `SLA SLAGlobalConfig` field to the `Config` struct
- [x] T003 Add SLA fields to `RunRecord` struct in `internal/project/project.go`: `ScheduledTime time.Time`, `DispatchDelay time.Duration`, `SLAViolation bool`, `SLAMaxDelay time.Duration`, `SLASkipped bool` (all with `json` tags and `omitempty`)
- [x] T004 Parse `sla` and `on_sla_violation` from task frontmatter YAML in the frontmatter parsing section of `internal/project/project.go` (add `SLA *SLAFrontmatter` and `OnSLAViolation string` to `fmData` struct, parse `max_delay` via `time.ParseDuration`, map to `Todo.SLA` and `Todo.OnSLAViolation`)
- [x] T005 Create `internal/daemon/sla.go` with helper functions: `getEffectiveSLA(todo project.Todo, globalCfg config.SLAGlobalConfig) (time.Duration, bool)` that returns effective max_delay and strict flag (per-task overrides global), and `checkSLA(todo project.Todo, globalCfg config.SLAGlobalConfig, now time.Time) (violation bool, delay time.Duration, scheduledTime time.Time, err error)` that uses `cron.Parse` + `Prev()` to calculate delay and compare against effective max_delay
- [x] T006 Create `internal/daemon/sla_test.go` with tests for: `getEffectiveSLA` (per-task only, global only, per-task overrides global, neither configured), `checkSLA` (on time, within threshold, violation detected, no SLA configured, non-cron task returns no violation)

**Checkpoint**: Foundation ready — SLAConfig types exist, SLA evaluation logic is tested, RunRecord has SLA fields

---

## Phase 3: User Story 1 — Per-Task SLA Configuration and Violation Detection (Priority: P1) MVP

**Goal**: Tasks with `sla.max_delay` in frontmatter are checked at dispatch time. Violations are recorded in RunRecord and the `on_sla_violation` hook fires. Strict mode skips late tasks.

**Independent Test**: Create a task with `sla: {max_delay: 1m}` and verify that when dispatched more than 1 minute late, a violation is recorded in the run record and the hook fires.

### Implementation for User Story 1

- [x] T007 [US1] Add SLA check in the daemon dispatch loop in `internal/daemon/daemon.go` tick function — after time window/quiet hours checks and before stopped-task check: call `checkSLA`, if violation and strict then skip (continue) with "SLA strict: skipped" reason, otherwise record violation data to pass along with dispatch
- [x] T008 [US1] Add `runSLAViolationHook` method to Daemon in `internal/daemon/daemon.go` that executes `todo.OnSLAViolation` as a shell command (goroutine, 60s timeout) with environment variables: `ANVIL_TASK_NAME`, `ANVIL_PROJECT`, `ANVIL_SLA_SCHEDULED_TIME`, `ANVIL_SLA_ACTUAL_TIME`, `ANVIL_SLA_DELAY`, `ANVIL_SLA_MAX_DELAY` — follow existing `runHook` pattern
- [x] T009 [US1] Call `runSLAViolationHook` from the dispatch loop when a non-strict SLA violation is detected (after recording violation, before queuing task) in `internal/daemon/daemon.go`
- [x] T010 [US1] Update the worker completion code in `internal/daemon/daemon.go` where `WriteRunRecord` is called — populate `ScheduledTime`, `DispatchDelay`, `SLAViolation`, `SLAMaxDelay`, and `SLASkipped` fields on the RunRecord based on the violation data passed from dispatch
- [x] T011 [US1] Add test in `internal/daemon/sla_test.go` for `checkSLA` with various delay scenarios: task on time (no violation), task within threshold (no violation), task exceeds threshold (violation), strict mode flag propagation

**Checkpoint**: Per-task SLA detection works — violations are recorded in run records, hooks fire, strict mode skips late tasks

---

## Phase 4: User Story 2 — SLA Status in Task Info (Priority: P2)

**Goal**: `anvil task get <name>` shows SLA configuration and most recent run's SLA status

**Independent Test**: Run `anvil task get` for a task with SLA configured and verify output includes SLA threshold and last run violation status

### Implementation for User Story 2

- [x] T012 [US2] Update `taskGetCmd` in `cmd/anvil/main.go` to display SLA configuration when `todo.SLA.MaxDelay > 0` — show "SLA: Xm max delay" and "strict" if enabled
- [x] T013 [US2] Update `taskGetCmd` in `cmd/anvil/main.go` to read most recent run record and display SLA status — show "Last Run: on time" or "Last Run: Xm late - SLA VIOLATION" based on `RunRecord.SLAViolation` and `RunRecord.DispatchDelay`
- [x] T014 [US2] Update the JSON output struct in `taskGetCmd` in `cmd/anvil/main.go` to include SLA fields (max_delay, strict, last_run_delay, last_run_violation) for `--json` flag

**Checkpoint**: `anvil task get` shows SLA info — users can check individual task SLA health

---

## Phase 5: User Story 3 — SLA Dashboard Command (Priority: P2)

**Goal**: `anvil task sla` command shows all tasks with SLA violations, with `--verbose` and `--reset` flags

**Independent Test**: Run `anvil task sla` and verify it lists tasks with SLA violations in the current project

### Implementation for User Story 3

- [x] T015 [US3] Add `taskSlaCmd` function in `cmd/anvil/main.go` that loads all projects and todos, filters for tasks with SLA configured, reads most recent run records, and displays tasks with SLA violations (task name, delay, max_delay, when)
- [x] T016 [US3] Add `--verbose` flag to `taskSlaCmd` in `cmd/anvil/main.go` — when set, show all SLA-configured tasks with their status (pass/fail) and delay info, not just violated ones
- [x] T017 [US3] Add `--reset` flag to `taskSlaCmd` in `cmd/anvil/main.go` — when set, iterate all run records for all tasks with SLA violations and clear SLA violation fields (set `SLAViolation: false`), then confirm reset count
- [x] T018 [US3] Wire `taskSlaCmd` into the task subcommand dispatch in `cmd/anvil/main.go` (add `"sla"` case in the task command switch)

**Checkpoint**: `anvil task sla` dashboard works — users can see all violations, get detailed view, and reset after downtime

---

## Phase 6: User Story 4 — Global SLA Defaults (Priority: P3)

**Goal**: Tasks without per-task SLA inherit from global `sla.default_max_delay` config

**Independent Test**: Set global `sla.default_max_delay: 30m` in config, create a task without per-task SLA, verify it uses the 30m global threshold

### Implementation for User Story 4

- [x] T019 [US4] Update the SLA check in the daemon dispatch loop in `internal/daemon/daemon.go` to pass `d.config.SLA` to `checkSLA` and `getEffectiveSLA` — this should already work from T005/T007 if `getEffectiveSLA` correctly falls back to global config
- [x] T020 [US4] Add test in `internal/daemon/sla_test.go` for `getEffectiveSLA` verifying global fallback: task with no per-task SLA but global `DefaultMaxDelay: "30m"` returns 30m, and task with per-task `MaxDelay: 15m` and global `DefaultMaxDelay: 30m` returns 15m (per-task wins)

**Checkpoint**: Global defaults work — tasks without per-task SLA use global threshold

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Help text, documentation, and final validation

- [x] T021 Update help text for `anvil task` in `cmd/anvil/main.go` to list the new `sla` subcommand
- [x] T022 Update help text for `anvil task get` in `cmd/anvil/main.go` to mention SLA status in output
- [x] T023 Run `go test ./...` and `go build ./cmd/anvil/` to verify all tests pass and project compiles

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — can start immediately
- **US1 (Phase 3)**: Depends on Phase 2 (T001-T006)
- **US2 (Phase 4)**: Depends on US1 (T010) — needs SLA data in run records
- **US3 (Phase 5)**: Depends on US1 (T010) — needs SLA data in run records; can run in parallel with US2
- **US4 (Phase 6)**: Depends on Phase 2 (T005) for `getEffectiveSLA` with global config; can start after Phase 2
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 — No dependencies on other stories
- **US2 (P2)**: Depends on US1 — needs SLA violation data in RunRecord to display
- **US3 (P2)**: Depends on US1 — needs SLA violation data in RunRecord to query
- **US4 (P3)**: Can start after Phase 2 — Uses `getEffectiveSLA` independently of dispatch integration

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- T003 depends on T001 (same file, adds to RunRecord)
- T004 depends on T001 (same file, adds frontmatter parsing)
- T005 and T006 can run in parallel with T003/T004 (different files)
- US2 and US3 can proceed in parallel after US1 (both read from run records)
- US4 can proceed in parallel with US1 (only needs helpers from Phase 2)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (T001-T006)
2. Complete Phase 3: User Story 1 (T007-T011)
3. **STOP and VALIDATE**: Test per-task SLA detection independently
4. Deploy/demo if ready

### Incremental Delivery

1. Phase 2 → Foundation ready
2. US1 → SLA violation detection works (MVP!)
3. US2 → SLA info in task get
4. US3 → SLA dashboard command
5. US4 → Global defaults
6. Polish → Help text, final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
