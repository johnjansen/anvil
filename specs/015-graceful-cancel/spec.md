# Feature Specification: Task Execution Cancellation with Partial Result Capture

**Feature Branch**: `015-graceful-cancel`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task execution cancellation with partial result capture"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Graceful Kill with State Capture (Priority: P1)

As an operator running a long-running task (e.g., data migration processing thousands of records), I want to gracefully cancel it and capture what was accomplished, so I don't lose all progress when I need to stop.

**Why this priority**: Core value proposition — without graceful cancellation, all other features (partial results, resume) have no foundation. This is the minimum viable improvement over the current "hard kill" behavior.

**Independent Test**: Run a long task, issue `anvil task kill my-task --graceful`, verify the task receives a signal, the on_kill hook runs, and the run record reflects the graceful termination.

**Acceptance Scenarios**:

1. **Given** a running task, **When** the operator runs `anvil task kill my-task --graceful`, **Then** the task receives a termination signal (SIGTERM), the on_kill hook executes, and after a grace period the task is forcefully terminated if still running.
2. **Given** a running task with an on_kill hook, **When** graceful kill is triggered, **Then** the hook command runs to completion (up to grace period) before the task is killed.
3. **Given** a running task, **When** the operator runs `anvil task kill my-task --force`, **Then** the task is immediately terminated without running hooks.
4. **Given** a running task, **When** graceful kill is triggered but the task does not exit within the grace period (default 30 seconds), **Then** the task is forcefully killed.

---

### User Story 2 - Partial Result Capture and Viewing (Priority: P2)

As an operator, I want my tasks to be able to emit partial progress markers during execution, and when a task is cancelled or fails, I want to see what was accomplished so I can pick up where I left off.

**Why this priority**: Builds on graceful kill by giving tasks a way to communicate progress. Without this, operators know the task was gracefully stopped but not what it accomplished.

**Independent Test**: Run a task that emits `##anvil:partial` markers in its output, kill it, then run `anvil task partial my-task` and verify the captured partial results are displayed.

**Acceptance Scenarios**:

1. **Given** a running task that emits `##anvil:partial {"records_processed": 500}` in stdout, **When** the daemon processes the output, **Then** the partial result is captured and stored in the run record.
2. **Given** a task with captured partial results, **When** the operator runs `anvil task partial my-task`, **Then** the partial results from the most recent run are displayed as formatted JSON.
3. **Given** a task that emits multiple partial markers, **When** the task completes or is killed, **Then** only the latest partial result is stored (not all intermediate ones).
4. **Given** a task with no partial results, **When** the operator runs `anvil task partial my-task`, **Then** a message indicates no partial results are available.

---

### User Story 3 - Resume from Partial State (Priority: P3)

As an operator, I want to resume a cancelled task from where it left off, using the partial results from the previous run as input, so that work isn't repeated.

**Why this priority**: Highest user value but depends on both graceful kill (US1) and partial capture (US2) being in place. This completes the full workflow.

**Independent Test**: Kill a task that has partial results, then run `anvil task run my-task --resume` and verify the previous partial results are passed to the task as environment variables.

**Acceptance Scenarios**:

1. **Given** a task with partial results from a previous run, **When** the operator runs `anvil task run my-task --resume`, **Then** the task is started with `ANVIL_PARTIAL_RESULTS` environment variable containing the JSON partial results.
2. **Given** a task being gracefully killed, **When** the kill signal is sent, **Then** the `ANVIL_IS_KILLED=true` environment variable is set (via the on_kill hook environment).
3. **Given** a task with no partial results from a previous run, **When** the operator runs `anvil task run my-task --resume`, **Then** the task starts normally with an empty `ANVIL_PARTIAL_RESULTS` value and a warning message.

---

### Edge Cases

- What happens when the on_kill hook itself hangs? It is subject to the same grace period timeout and force-killed along with the task.
- What happens when a task emits malformed JSON in a `##anvil:partial` marker? The malformed data is stored as a raw string rather than parsed JSON.
- What happens when multiple tasks with the same name are killed simultaneously? Each running instance is killed independently with its own graceful shutdown.
- What happens when the daemon is stopped while a graceful kill is in progress? The grace period is abandoned and the task is force-killed as part of daemon shutdown.
- What happens when `--graceful` and `--force` are both specified? `--force` takes precedence and the task is immediately killed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a `--graceful` (`-g`) flag on `anvil task kill` that sends SIGTERM before force-killing after a configurable grace period (default 30 seconds).
- **FR-002**: System MUST support an `on_kill` frontmatter field in task files that specifies a shell command to run before task termination during graceful kill.
- **FR-003**: System MUST support a `--force` flag on `anvil task kill` that immediately terminates the task without running hooks (current behavior).
- **FR-004**: System MUST capture partial results from task output when lines match the `##anvil:partial <JSON>` protocol marker, storing the most recent partial result in the run record.
- **FR-005**: System MUST provide an `anvil task partial <name>` command that displays partial results from the most recent run of a task.
- **FR-006**: System MUST set `ANVIL_IS_KILLED=true` environment variable in the on_kill hook execution context.
- **FR-007**: System MUST support a `--resume` flag on `anvil task run` that passes previous run's partial results via `ANVIL_PARTIAL_RESULTS` environment variable.
- **FR-008**: System MUST store partial results in the existing run record structure.
- **FR-009**: System MUST mark run records with the termination method (graceful, force, normal completion) so operators can distinguish how tasks ended.

### Key Entities

- **Partial Result**: JSON data emitted by a task during execution via the `##anvil:partial` protocol, representing the task's progress checkpoint. Stored in the run record.
- **On-Kill Hook**: A shell command defined in task frontmatter that executes before graceful termination, giving the task a chance to save state.
- **Grace Period**: The time window (default 30s) between SIGTERM and SIGKILL during graceful cancellation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can gracefully cancel a running task and have the on_kill hook complete within the grace period in 100% of cases where the hook is defined.
- **SC-002**: Partial results emitted by tasks are captured and viewable within 1 second of running `anvil task partial`.
- **SC-003**: Tasks resumed with `--resume` receive the correct partial results from the previous run in their environment.
- **SC-004**: The existing `anvil task kill` behavior (force kill) remains unchanged when neither `--graceful` nor `--force` flags are specified.
