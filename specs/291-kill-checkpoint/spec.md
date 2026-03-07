# Feature Specification: Task Kill with Checkpoint

**Feature Branch**: `291-kill-checkpoint`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add task kill with checkpoint to save progress before stopping"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Graceful Stop with Checkpoint (Priority: P1)

A user has a long-running data processing task (e.g., processing 10,000 items). At item 5,000 they need to stop the task for maintenance or resource reasons. They run `anvil task kill my-task --checkpoint` which sends a graceful termination signal to the task. The task saves its current progress (checkpoint at item 5,000) and exits cleanly. The run is recorded as "stopped with checkpoint" so the user knows progress was preserved.

**Why this priority**: This is the core value proposition. Without graceful stop + checkpoint save, the entire feature has no purpose.

**Independent Test**: Can be fully tested by starting a checkpoint-enabled task, running `anvil task kill --checkpoint`, and verifying the task receives a graceful signal and the run record reflects "stopped-with-checkpoint" status.

**Acceptance Scenarios**:

1. **Given** a running task with `checkpoint: true` in its frontmatter, **When** the user runs `anvil task kill my-task --checkpoint`, **Then** the daemon sends a graceful termination signal to the task process and waits for it to exit.
2. **Given** a task that handles graceful termination and writes checkpoint data before exiting, **When** the task exits after receiving the graceful signal via `--checkpoint`, **Then** the run record is saved with status "stopped-with-checkpoint" and the checkpoint data is preserved.
3. **Given** a running task with `checkpoint: true`, **When** the task does not exit within the grace period after the graceful signal, **Then** the daemon force-kills the task and records the run as "stopped" without checkpoint.

---

### User Story 2 - Resume from Checkpoint After Graceful Stop (Priority: P2)

After a task was stopped with checkpoint, the user wants the next run to pick up where it left off. When the task runs again (on schedule or manually triggered), it resumes from the saved checkpoint rather than starting from scratch.

**Why this priority**: Saving checkpoint is only valuable if the next run can resume from it. This completes the user value loop.

**Independent Test**: Can be tested by stopping a task with checkpoint, then triggering the next run and verifying it starts from the saved checkpoint position.

**Acceptance Scenarios**:

1. **Given** a task that was previously stopped with checkpoint at item 5,000, **When** the task runs again, **Then** the checkpoint data from the previous run is available to the task so it can resume from item 5,000.
2. **Given** a task that completed successfully (not stopped with checkpoint), **When** the task runs again, **Then** no checkpoint data is carried forward and the task starts fresh.

---

### User Story 3 - Checkpoint Status Visibility (Priority: P3)

A user wants to see which tasks were stopped with checkpoint and what checkpoint state was saved. They use `anvil task history` to view run history including checkpoint status.

**Why this priority**: Visibility is important for user confidence but the feature works without it.

**Independent Test**: Can be tested by viewing task history after a checkpoint stop and verifying the checkpoint status column appears.

**Acceptance Scenarios**:

1. **Given** a task with mixed run history (some successful, some stopped with checkpoint), **When** the user runs `anvil task history my-task`, **Then** each run shows its status including "stopped-with-checkpoint" and whether checkpoint data exists.

---

### User Story 4 - Error on Non-Checkpoint Task (Priority: P3)

A user tries to kill a task with `--checkpoint` but the task does not have `checkpoint: true` in its frontmatter. The system provides a clear error message explaining that checkpoint is not enabled for this task.

**Why this priority**: Error handling for invalid usage; prevents confusion.

**Independent Test**: Can be tested by running `anvil task kill --checkpoint` on a task without checkpoint enabled and verifying the error message.

**Acceptance Scenarios**:

1. **Given** a running task without `checkpoint: true` in frontmatter, **When** the user runs `anvil task kill my-task --checkpoint`, **Then** the system displays a clear error message indicating checkpoint is not enabled for this task and does not kill the task.
2. **Given** a task that is not currently running, **When** the user runs `anvil task kill my-task --checkpoint`, **Then** the system displays an error indicating the task is not running.

---

### Edge Cases

- What happens when the task crashes during checkpoint save (after graceful signal but before clean exit)? The run should be recorded as "stopped" without checkpoint, not "stopped-with-checkpoint".
- What happens when multiple `kill --checkpoint` commands are issued for the same task? The second command should be ignored if the task is already shutting down.
- What happens when `anvil task kill` (without `--checkpoint`) is used on a checkpoint-enabled task? It should behave as today: immediate forced termination with no checkpoint save.
- What happens when the grace period expires? The daemon escalates to forced termination and records the run without checkpoint status.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a `--checkpoint` (shorthand `-c`) flag on the `anvil task kill` command.
- **FR-002**: System MUST send a graceful termination signal instead of immediate forced termination when `--checkpoint` flag is used.
- **FR-003**: System MUST wait for the task to exit gracefully within a configurable grace period after sending the graceful signal.
- **FR-004**: System MUST escalate to forced termination if the task does not exit within the grace period.
- **FR-005**: System MUST record the run status as "stopped-with-checkpoint" when a task exits gracefully after `--checkpoint` kill.
- **FR-006**: System MUST preserve checkpoint data written by the task during graceful shutdown.
- **FR-007**: System MUST make saved checkpoint data available to the next run of the same task so it can resume.
- **FR-008**: System MUST reject `--checkpoint` with a clear error if the target task does not have `checkpoint: true` in its frontmatter.
- **FR-009**: System MUST reject `--checkpoint` with a clear error if the target task is not currently running.
- **FR-010**: System MUST display checkpoint status in `anvil task history` output.
- **FR-011**: System MUST ignore duplicate `kill --checkpoint` commands if the task is already in graceful shutdown.

### Key Entities

- **RunRecord**: Existing entity representing a single task execution. Extended with checkpoint-related status ("stopped-with-checkpoint") and checkpoint data reference.
- **Todo**: Existing entity representing a task definition. The `checkpoint: true` frontmatter field indicates checkpoint support.
- **Grace Period**: Configurable duration the daemon waits after graceful signal before escalating to forced kill. Default: 30 seconds.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can gracefully stop a checkpoint-enabled task and have progress saved in a single command.
- **SC-002**: A task resumed after checkpoint stop picks up from the saved state, avoiding reprocessing of already-completed work.
- **SC-003**: Users can distinguish checkpoint-stopped runs from other statuses in task history within one glance.
- **SC-004**: The system provides immediate, clear feedback when `--checkpoint` is used on an incompatible task (no checkpoint enabled or not running).

## Assumptions

- The existing `checkpoint: true` frontmatter field and checkpoint data storage mechanism already exist in the codebase. This feature extends the existing checkpoint system, not creates it from scratch.
- Tasks are responsible for handling graceful termination signals and writing their own checkpoint data. The daemon's role is to send the signal and wait, not to extract checkpoint data from the task.
- The grace period default of 30 seconds is reasonable for most tasks. This can be overridden per-task in frontmatter if needed in the future.
- The existing `anvil task kill` command currently uses immediate forced termination. The `--checkpoint` flag changes this to graceful termination with fallback to forced termination.
