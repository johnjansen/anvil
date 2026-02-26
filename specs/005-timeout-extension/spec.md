# Feature Specification: Task Execution Timeout Extension

**Feature Branch**: `005-timeout-extension`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task execution timeout extension during runtime"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Manual Timeout Extension (Priority: P1)

A user has a running task that is approaching its timeout limit but is still making progress. Rather than letting it timeout and lose work, the user extends the timeout from the command line to give it more time to complete.

**Why this priority**: This is the core value proposition — preventing data loss and wasted work by allowing users to intervene when tasks need more time. Without this, users have no recourse except watching tasks fail.

**Independent Test**: Run a task with a short timeout (e.g., 2 minutes). Before it expires, run `anvil task extend-timeout <name> 5m` and verify the task continues running past the original timeout.

**Acceptance Scenarios**:

1. **Given** a task is running with 2 minutes remaining on its timeout, **When** the user runs `anvil task extend-timeout my-task 5m`, **Then** the task's deadline is extended by 5 minutes from the current time and the command confirms the new deadline.
2. **Given** a task is running, **When** the user runs `anvil task extend-timeout my-task 1h --absolute`, **Then** the task's deadline is set to 1 hour from now (replacing the remaining time).
3. **Given** no task with the given name is currently running, **When** the user runs `anvil task extend-timeout my-task 5m`, **Then** the command outputs an error indicating the task is not running.

---

### User Story 2 - Timeout Visibility (Priority: P2)

A user wants to see how much time a running task has left before timeout, how many times the timeout has been extended, and the original vs current timeout values.

**Why this priority**: Visibility is essential for making informed decisions about whether to extend a timeout. Without it, users are flying blind.

**Independent Test**: Run a task with a timeout, extend it once, then verify `anvil task timeout <name>` and `anvil task get <name>` show the original timeout, current deadline, remaining time, and extension count.

**Acceptance Scenarios**:

1. **Given** a task is running with a 30-minute timeout and has been extended once by 15 minutes, **When** the user runs `anvil task timeout my-task`, **Then** output shows original timeout (30m), current timeout (45m), remaining time, and extensions used (1).
2. **Given** multiple tasks are running, **When** the user runs `anvil ps`, **Then** the output includes a timeout countdown column showing remaining time for each task.
3. **Given** a task has been extended, **When** the user runs `anvil task get my-task`, **Then** the output includes timeout extension information.

---

### User Story 3 - Automatic Timeout Extension (Priority: P2)

A user configures a task to automatically extend its timeout when it is still making progress (evidenced by checkpoint output). This prevents timeout failures for tasks with unpredictable runtimes without requiring manual intervention.

**Why this priority**: Same priority as visibility because auto-extend delivers significant value for unattended tasks (ETL, data processing) that can't rely on a human to manually extend.

**Independent Test**: Create a task with `auto_extend` configuration, run it with a short timeout, verify that the timeout is automatically extended when checkpoints are detected, and that extension stops after `max_extensions` is reached.

**Acceptance Scenarios**:

1. **Given** a task is configured with `auto_extend: {enabled: true, max_extensions: 3, extension_duration: 15m}` and timeout is 30m, **When** the task emits a checkpoint within 5 minutes of the deadline, **Then** the timeout is automatically extended by 15m and the extension count increments.
2. **Given** a task has already used its maximum number of auto-extensions (3 of 3), **When** the task approaches the deadline again, **Then** no further extension occurs and the task times out normally.
3. **Given** a task has `auto_extend` enabled but has not emitted any checkpoint recently, **When** the deadline approaches, **Then** the timeout is NOT automatically extended (the task appears stalled).

---

### User Story 4 - Timeout Warning Hook (Priority: P3)

A user configures a hook that fires when a task is approaching its timeout deadline, allowing external notification (e.g., Slack alert) so the user can decide whether to extend the timeout.

**Why this priority**: Nice-to-have alerting that builds on the extension mechanism. The core extension and auto-extend features provide most of the value.

**Independent Test**: Create a task with `on_timeout_warning` set to a shell command, run it with a short timeout, and verify the hook fires when 5 minutes remain (or a configurable threshold).

**Acceptance Scenarios**:

1. **Given** a task is configured with `on_timeout_warning: "echo warning >> /tmp/test"` and has 5 minutes remaining, **When** the daemon checks running tasks, **Then** the hook command is executed with appropriate environment variables.
2. **Given** a task has auto-extend remaining, **When** the timeout warning fires, **Then** the environment variable `ANVIL_AUTO_EXTEND_REMAINING` indicates how many auto-extensions are left.

---

### Edge Cases

- What happens when the user extends a timeout for a task that has already finished? The command reports that the task is not running.
- What happens when the user extends a timeout by a negative or zero duration? The command rejects the input with an error.
- What happens when auto-extend fires but the task finishes before the extension takes effect? The extension is a no-op.
- What happens when multiple manual extensions are applied in quick succession? Each extension is additive (or absolute if --absolute is used), with the latest extension taking effect.
- What happens when a persistent task has auto-extend configured? Auto-extend applies to each individual run cycle, not the persistent task lifetime.
- What happens when a task has no timeout configured but the user tries to extend? The command reports that the task has no timeout to extend.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a CLI command `anvil task extend-timeout <name> <duration>` that extends the timeout of a currently running task by the specified duration.
- **FR-002**: The extend-timeout command MUST support an `--absolute` flag that sets the new deadline to `<duration>` from now, instead of adding to the remaining time.
- **FR-003**: The extend-timeout command MUST communicate the extension to the daemon, which updates the running task's context deadline.
- **FR-004**: System MUST track the number of timeout extensions applied to each running task and the original timeout value.
- **FR-005**: The `anvil task timeout` command MUST display: original timeout, current deadline, remaining time, and number of extensions used for a specified task (or all tasks with `--all`).
- **FR-006**: The `anvil ps` output MUST include timeout countdown information for running tasks that have a timeout configured.
- **FR-007**: The `anvil task get` output MUST include timeout extension information when a task has been extended.
- **FR-008**: System MUST support `auto_extend` configuration in task frontmatter with fields: `enabled` (boolean), `max_extensions` (integer), and `extension_duration` (duration string).
- **FR-009**: When auto-extend is enabled, the system MUST automatically extend the timeout when a checkpoint is detected within a configurable warning window before the deadline (default: 5 minutes).
- **FR-010**: Auto-extend MUST NOT extend beyond the configured `max_extensions` limit.
- **FR-011**: System MUST support an `on_timeout_warning` frontmatter field that specifies a shell command to execute when a task approaches its deadline.
- **FR-012**: The timeout warning hook MUST receive environment variables: `ANVIL_TASK_NAME`, `ANVIL_PROJECT`, `ANVIL_TIMEOUT_REMAINING`, `ANVIL_TIMEOUT_ORIGINAL`, `ANVIL_EXTENSIONS_USED`, `ANVIL_AUTO_EXTEND_REMAINING`.
- **FR-013**: Manual timeout extension via CLI MUST work regardless of auto-extend configuration (they are independent mechanisms).
- **FR-014**: Extension data (count, original timeout, current deadline) MUST be persisted in the run record for post-run analysis.

### Key Entities

- **TimeoutExtension**: Tracks timeout modifications for a running task — original timeout, current deadline, extension count, extension history.
- **AutoExtendConfig**: Per-task configuration for automatic extension — enabled flag, max extensions, extension duration.
- **RunningTaskTimeout**: Runtime state of a task's timeout — context deadline, warning threshold, last checkpoint time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can extend a running task's timeout within 2 seconds of issuing the command.
- **SC-002**: Timeout extension information is visible in all relevant CLI outputs (ps, task get, task timeout) within the same daemon tick after extension.
- **SC-003**: Auto-extend prevents timeout for tasks still making progress, reducing unnecessary timeout failures for configured tasks.
- **SC-004**: Timeout warning hooks fire reliably when tasks approach their deadline, giving users at least the configured warning window to respond.

## Assumptions

- The warning window for `on_timeout_warning` defaults to 5 minutes before the deadline.
- Auto-extend only activates when a checkpoint has been emitted recently (within the warning window), indicating the task is still making progress.
- The daemon's tick interval (default 10 seconds) is sufficient for timely detection of approaching deadlines and checkpoint activity.
- Manual extension and auto-extend are independent and additive — a task can have both configured.
- Extension data is stored in-memory on the daemon (for running tasks) and persisted in RunRecord on completion. Extensions do not survive daemon restarts (the task's original timeout applies on restart).
