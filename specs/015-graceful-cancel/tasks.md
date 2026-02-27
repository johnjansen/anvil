# Tasks: Graceful Cancel with Partial Result Capture

**Input**: Design documents from `/specs/015-graceful-cancel/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: Add new fields to data model structs and wire up partial capture infrastructure

- [x] T001 [P] Add PartialResults and TerminationMethod fields to RunRecord struct in internal/project/project.go
- [x] T002 [P] Add OnKill field to Todo struct and on_kill to frontmatter parsing in internal/project/project.go
- [x] T003 [P] Add `##anvil:partial` prefix constant and onPartial callback to statusWriter in internal/runner/runner.go
- [x] T004 [P] Add onPartial parameter to Runner.Run() signature in internal/runner/runner.go

---

## Phase 2: Foundational (Kill Infrastructure)

**Purpose**: Core graceful kill mechanism that all user stories build on

- [x] T005 Add Graceful bool field to KillRequest struct in internal/daemon/daemon.go
- [x] T006 Update SendKillRequest() to accept graceful parameter in internal/daemon/daemon.go
- [x] T007 Update handleKill() to support graceful mode -- send SIGTERM to child process, start grace period timer, run on_kill hook, force kill after timeout in internal/daemon/daemon.go
- [x] T008 Wire onPartial callback in runTask() -- capture partial result in variable and store in RunRecord in internal/daemon/daemon.go
- [x] T009 Set TerminationMethod field in RunRecord at all exit paths in internal/daemon/daemon.go

**Checkpoint**: Daemon supports graceful kill with SIGTERM, on_kill hook, and partial capture

---

## Phase 3: User Story 1 - Graceful Kill CLI (Priority: P1) MVP

**Goal**: Operators can gracefully cancel tasks with --graceful flag

**Independent Test**: Run a long task, run `anvil task kill my-task --graceful`, verify SIGTERM sent and on_kill hook runs.

### Implementation for User Story 1

- [x] T010 [US1] Add --graceful/-g and --force flag parsing to taskKillCmd() in cmd/anvil/main.go
- [x] T011 [US1] Update SendKillRequest call to pass graceful flag in cmd/anvil/main.go
- [x] T012 [US1] Add ANVIL_IS_KILLED=true env var to on_kill hook execution context in internal/daemon/daemon.go

**Checkpoint**: `anvil task kill --graceful` sends SIGTERM, runs on_kill hook, then force-kills after grace period

---

## Phase 4: User Story 2 - Partial Result Capture and Viewing (Priority: P2)

**Goal**: Tasks can emit partial results and operators can view them

**Independent Test**: Run a task that emits `##anvil:partial` markers, kill it, run `anvil task partial my-task`.

### Implementation for User Story 2

- [x] T013 [US2] Add `case "partial":` to taskCmd() dispatcher in cmd/anvil/main.go
- [x] T014 [US2] Implement taskPartialCmd() in cmd/anvil/main.go -- load latest RunRecord, display PartialResults field

**Checkpoint**: `anvil task partial` shows partial results captured from task output

---

## Phase 5: User Story 3 - Resume from Partial (Priority: P3)

**Goal**: Operators can resume tasks with previous partial results injected as env var

**Independent Test**: Kill a task with partial results, run `anvil task run my-task --resume`, verify ANVIL_PARTIAL_RESULTS env var.

### Implementation for User Story 3

- [x] T015 [US3] Add Resume bool field to RunRequest struct and update SendRunRequest() in internal/daemon/daemon.go
- [x] T016 [US3] Add --resume flag parsing to taskRunCmd() in cmd/anvil/main.go
- [x] T017 [US3] Handle resume in daemon handleRun() -- look up latest RunRecord, inject ANVIL_PARTIAL_RESULTS env var into task dispatch in internal/daemon/daemon.go

**Checkpoint**: `anvil task run --resume` injects previous partial results into task environment

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T018 Run `go build ./...` to verify all changes compile

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies -- T001-T004 all parallel (different files)
- **Foundational (Phase 2)**: Depends on Phase 1 -- T005-T009 sequential within daemon.go
- **US1 (Phase 3)**: Depends on Phase 2 -- CLI wiring for graceful kill
- **US2 (Phase 4)**: Depends on T008 (partial capture) -- CLI for viewing partials
- **US3 (Phase 5)**: Depends on T001 (PartialResults field) -- resume from partials
- **Polish (Phase 6)**: Depends on all phases

### Parallel Opportunities

- T001, T002, T003, T004 can all run in parallel (different files)
- After Phase 2: US1-US3 CLI tasks can run in parallel (different functions in main.go)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Kill infrastructure (T005-T009)
3. Complete Phase 3: Graceful kill CLI (T010-T012)
4. **STOP and VALIDATE**: Test `anvil task kill --graceful`

### Incremental Delivery

1. Setup + Foundation -> Core infrastructure
2. Add US1 -> Graceful kill working -> Validate
3. Add US2 -> Partial capture + viewing -> Validate
4. Add US3 -> Resume from partial -> Validate
5. Polish -> Build check

---

## Notes

- Total tasks: 18
- Tasks per user story: US1=3, US2=2, US3=3, Setup=4, Foundation=5, Polish=1
- All user stories independently testable after Phase 2
