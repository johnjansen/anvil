# Tasks: Task Kill with Checkpoint

**Input**: Design documents from `/specs/291-kill-checkpoint/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new project setup needed. All changes are to existing files. This phase is empty.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core data model and daemon infrastructure changes that all user stories depend on.

**CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T001 [P] Add `CheckpointGracePeriod` field to `Todo` struct and parse `checkpoint_grace_period` from YAML frontmatter (default 30s) in `internal/project/project.go`
- [ ] T002 [P] Add `GracefulStop` channel (`chan struct{}`), `ShuttingDown` bool, and `ChildPID` int fields to `RunningTask` struct in `internal/daemon/daemon.go`
- [ ] T003 [P] Add `Checkpoint` bool field to `KillRequest` struct in `internal/daemon/daemon.go`
- [ ] T004 Store child process PID in `RunningTask.ChildPID` when starting task execution in `runTask` function in `internal/daemon/daemon.go`
- [ ] T005 Initialize `GracefulStop` channel when creating `RunningTask` instance in `runTask` function in `internal/daemon/daemon.go`

**Checkpoint**: Foundation ready - RunningTask tracks child PID and has graceful stop channel, KillRequest supports checkpoint flag, Todo parses grace period.

---

## Phase 3: User Story 1 - Graceful Stop with Checkpoint (Priority: P1) MVP

**Goal**: User can run `anvil task kill my-task --checkpoint` to gracefully stop a checkpoint-enabled task, preserving checkpoint data in the run record.

**Independent Test**: Start a checkpoint-enabled task, run `anvil task kill --checkpoint`, verify SIGTERM is sent, task exits gracefully, and RunRecord has status "stopped-with-checkpoint" with checkpoint data.

### Implementation for User Story 1

- [ ] T006 [US1] Add `--checkpoint` / `-c` bool flag to `taskKillCmd` in `cmd/anvil/task_lifecycle.go`
- [ ] T007 [US1] Pass checkpoint flag in `SendKillRequest` call, updating the request payload to include `Checkpoint: true` in `cmd/anvil/task_lifecycle.go`
- [ ] T008 [US1] Update `SendKillRequest` function to include `Checkpoint` field in JSON payload sent to daemon in `internal/daemon/daemon.go`
- [ ] T009 [US1] Update `handleKill` to validate checkpoint preconditions: reject with error if `--checkpoint` is true but task's `Todo.Checkpoint` is false; reject if task is already shutting down in `internal/daemon/daemon.go`
- [ ] T010 [US1] Update `handleKill` to close `GracefulStop` channel (instead of calling `Cancel()`) when `Checkpoint: true` and preconditions pass in `internal/daemon/daemon.go`
- [ ] T011 [US1] Add graceful shutdown detection in `runTask`: select on `GracefulStop` channel, send SIGTERM to child process via `ChildPID`, start grace period timer in `internal/daemon/daemon.go`
- [ ] T012 [US1] Add grace period timeout handling in `runTask`: if child doesn't exit within grace period after SIGTERM, send SIGKILL and call `Cancel()` in `internal/daemon/daemon.go`
- [ ] T013 [US1] Update RunRecord construction in `runTask` to set `Error: "stopped-with-checkpoint"` when task exits after graceful stop with checkpoint data present, or `Error: "killed-after-grace-period"` when grace period expires in `internal/daemon/daemon.go`
- [ ] T014 [US1] Update activity log entry in `handleKill` to include `"method": "checkpoint"` in Details when checkpoint kill is used in `internal/daemon/daemon.go`

**Checkpoint**: User Story 1 complete. `anvil task kill --checkpoint` gracefully stops tasks and saves checkpoint data in RunRecord.

---

## Phase 4: User Story 2 - Resume from Checkpoint After Graceful Stop (Priority: P2)

**Goal**: After a task is stopped with checkpoint, the next run automatically resumes from the saved checkpoint data.

**Independent Test**: Stop a task with checkpoint, trigger next run, verify `ANVIL_CHECKPOINT_DATA` env var contains the checkpoint data from the stopped run.

### Implementation for User Story 2

- [ ] T015 [US2] Verify that existing `LatestCheckpointData` function in `internal/project/project.go` correctly reads checkpoint data from a RunRecord with `Error: "stopped-with-checkpoint"` status (may need to read from `current.json` or latest completed run)
- [ ] T016 [US2] Verify that existing checkpoint injection in `runTask` (setting `ANVIL_CHECKPOINT_DATA` env var) works correctly when the previous run was a checkpoint stop - adjust `LatestCheckpointData` if it filters by `Success: true` in `internal/project/project.go`

**Checkpoint**: User Story 2 complete. Tasks resume from checkpoint after graceful stop.

---

## Phase 5: User Story 3 - Checkpoint Status Visibility (Priority: P3)

**Goal**: Users can see "stopped-with-checkpoint" status in `anvil task history` output.

**Independent Test**: View task history after a checkpoint stop, verify the status column shows "stopped-with-checkpoint".

### Implementation for User Story 3

- [ ] T017 [US3] Update status display logic in `taskHistoryCmd` to show "stopped-with-checkpoint" as a distinct status (not just generic "error") in `cmd/anvil/task_lifecycle.go`

**Checkpoint**: User Story 3 complete. Task history clearly shows checkpoint stop status.

---

## Phase 6: User Story 4 - Error on Non-Checkpoint Task (Priority: P3)

**Goal**: Clear error messages when `--checkpoint` is used incorrectly.

**Independent Test**: Run `anvil task kill --checkpoint` on a task without checkpoint enabled, verify clear error message.

### Implementation for User Story 4

- [ ] T018 [US4] Add CLI-side error message formatting for checkpoint-not-enabled and task-not-running errors returned from daemon in `cmd/anvil/task_lifecycle.go`

**Checkpoint**: User Story 4 complete. Users get clear feedback for invalid checkpoint kill usage.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T019 Verify edge case: duplicate `kill --checkpoint` commands are rejected when task is already shutting down in `internal/daemon/daemon.go`
- [ ] T020 Verify edge case: `anvil task kill` (without `--checkpoint`) still works unchanged for all tasks in `internal/daemon/daemon.go`
- [ ] T021 Run `go build ./...` and `go test ./...` to verify no compilation errors or test regressions

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies - can start immediately
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion - BLOCKS other stories
- **User Story 2 (Phase 4)**: Depends on Phase 3 (needs "stopped-with-checkpoint" status to exist)
- **User Story 3 (Phase 5)**: Depends on Phase 3 (needs "stopped-with-checkpoint" status to display)
- **User Story 4 (Phase 6)**: Depends on Phase 3 (needs checkpoint validation in handleKill)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Foundation only - core feature
- **User Story 2 (P2)**: Depends on US1 (needs checkpoint stop to produce data for resume)
- **User Story 3 (P3)**: Depends on US1 (needs new status to display)
- **User Story 4 (P3)**: Depends on US1 (needs validation logic in handleKill)

### Within Each User Story

- Models/structs before logic
- Daemon changes before CLI changes
- Core flow before edge cases

### Parallel Opportunities

- T001, T002, T003 can all run in parallel (different structs, different files)
- T004, T005 depend on T002 (RunningTask struct changes)
- US3 (T017) and US4 (T018) can run in parallel after US1 completes

---

## Parallel Example: Foundational Phase

```bash
# Launch all struct changes together:
Task: "Add CheckpointGracePeriod to Todo in internal/project/project.go"
Task: "Add GracefulStop/ShuttingDown/ChildPID to RunningTask in internal/daemon/daemon.go"
Task: "Add Checkpoint to KillRequest in internal/daemon/daemon.go"
```

## Parallel Example: After User Story 1

```bash
# Launch US3 and US4 together (different files, no dependencies):
Task: "Update history display in cmd/anvil/task_lifecycle.go"
Task: "Add CLI error formatting in cmd/anvil/task_lifecycle.go"
# Note: These touch the same file so may need to be sequential
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (T001-T005)
2. Complete Phase 3: User Story 1 (T006-T014)
3. **STOP and VALIDATE**: Test `anvil task kill --checkpoint` end-to-end
4. Deploy if ready - core feature works

### Incremental Delivery

1. Foundation → Ready for stories
2. Add User Story 1 → Graceful kill works → Deploy (MVP!)
3. Add User Story 2 → Resume from checkpoint works → Deploy
4. Add User Story 3 + 4 → Visibility and error handling → Deploy
5. Polish → Edge cases verified → Final release

---

## Notes

- All changes are to existing files - no new files created
- The existing checkpoint system (capture, storage, resume) is fully leveraged
- Key insight: `handleKill` currently uses context cancellation; this feature adds SIGTERM path via `GracefulStop` channel
- The `runTask` goroutine has access to the child process, so it handles SIGTERM delivery
- Total: 21 tasks across 7 phases
