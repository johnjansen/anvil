# Feature Specification: Task Forecasting

**Feature Branch**: `275-task-forecasting`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add task forecasting to predict future resource usage"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Upcoming Scheduled Runs (Priority: P1)

A user wants to see what tasks will run over the next few days so they can plan maintenance windows, anticipate workload, and verify their schedule configuration is correct. They run `anvil task forecast` and see a chronological list of upcoming task executions with task names, scheduled times, and estimated durations.

**Why this priority**: This is the foundational capability. Without schedule projection, no other forecasting feature (cost, contention) can work. It also delivers immediate standalone value by giving users visibility into their task schedule.

**Independent Test**: Can be fully tested by configuring several tasks with known cron schedules, running `anvil task forecast`, and verifying the output matches expected execution times. Delivers value as a standalone schedule preview tool.

**Acceptance Scenarios**:

1. **Given** a project with 3 tasks on different cron schedules, **When** the user runs `anvil task forecast`, **Then** they see a chronological list of all task executions over the next 7 days with task name, scheduled time, and estimated duration.
2. **Given** a project with tasks, **When** the user runs `anvil task forecast --days 14`, **Then** they see task executions projected over the next 14 days.
3. **Given** a project with tasks, **When** the user runs `anvil task forecast --task my-task`, **Then** they see only executions for the specified task.
4. **Given** a project with no tasks configured, **When** the user runs `anvil task forecast`, **Then** they see a clear message indicating no tasks are scheduled.

---

### User Story 2 - Predict Resource Contention (Priority: P2)

A user wants to know when multiple tasks will overlap and whether the worker pool can handle the load. They run `anvil task forecast --contention` and see time windows where more tasks are scheduled simultaneously than there are available workers.

**Why this priority**: Contention prediction helps users avoid task failures and delays caused by resource saturation. It builds directly on the schedule projection from P1 and requires knowing the worker pool size.

**Independent Test**: Can be tested by configuring several tasks that overlap at the same time, setting a worker pool size smaller than the overlap count, running `anvil task forecast --contention`, and verifying bottleneck windows are identified.

**Acceptance Scenarios**:

1. **Given** 5 tasks scheduled at the same time and a worker pool of 3, **When** the user runs `anvil task forecast --contention`, **Then** they see the time window flagged as a bottleneck with task count exceeding worker count.
2. **Given** tasks that never overlap, **When** the user runs `anvil task forecast --contention`, **Then** they see a message indicating no contention detected.
3. **Given** tasks with varying durations that partially overlap, **When** the user runs `anvil task forecast --contention`, **Then** contention windows account for task duration, not just start time.

---

### User Story 3 - Project Cost Estimates (Priority: P3)

A user wants to understand the cost implications of their task schedule. They run `anvil task forecast --cost` and see estimated token usage and costs based on historical averages from past runs.

**Why this priority**: Cost projection is valuable but depends on having historical run data. It's an enhancement on top of schedule projection and is less universally needed than contention detection.

**Independent Test**: Can be tested by running tasks that generate known token usage, then running `anvil task forecast --cost` and verifying the projection multiplies historical averages by forecasted run count.

**Acceptance Scenarios**:

1. **Given** tasks with historical run records containing token usage, **When** the user runs `anvil task forecast --cost`, **Then** they see projected token usage and estimated cost for the forecast period.
2. **Given** a task with no historical runs, **When** the user runs `anvil task forecast --cost`, **Then** that task shows "no data" for cost estimates rather than failing.
3. **Given** tasks with varying historical costs, **When** the user runs `anvil task forecast --cost`, **Then** cost estimates use an average of recent runs (not just the last run).

---

### User Story 4 - What-If Analysis for New Tasks (Priority: P4)

A user is considering adding a new task and wants to see how it would affect their schedule, contention, and costs before committing. They run `anvil add --dry-run -s "0 9 * * *" "New task"` and see the projected impact without actually adding the task.

**Why this priority**: What-if analysis is a power-user feature that depends on all other forecasting capabilities being in place. It provides planning value but is not essential for day-to-day monitoring.

**Independent Test**: Can be tested by running `anvil add --dry-run` with a schedule that would create contention, and verifying the forecast shows the impact without modifying the project's task list.

**Acceptance Scenarios**:

1. **Given** an existing set of tasks, **When** the user runs `anvil add --dry-run -s "0 9 * * *" "New task"`, **Then** they see a forecast that includes the hypothetical new task alongside existing tasks.
2. **Given** the dry-run completes, **When** the user checks their task list, **Then** no new task has been added.
3. **Given** the new task would create contention at 09:00, **When** running with `--dry-run`, **Then** the output highlights the new contention that would be introduced.

---

### Edge Cases

- What happens when a task has no cron schedule (manual-only tasks)? They should be excluded from the forecast.
- How does the system handle tasks with very frequent schedules (e.g., every minute) over a long forecast horizon? Output should be summarized rather than listing thousands of individual runs.
- What happens when the forecast period is set to 0 or a negative number? The system should reject invalid horizon values with a clear error message.
- How does the system handle timezone differences in cron schedules? Forecasts should use the project's configured timezone or local system time.
- What happens when historical run data shows highly variable durations? The system should use a reasonable average and optionally indicate variance.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST project task execution times forward from current time based on each task's cron schedule for a configurable number of days (default 7).
- **FR-002**: System MUST display forecasted runs in chronological order showing task name, scheduled time, and estimated duration.
- **FR-003**: System MUST support filtering forecasts by specific task name via `--task` flag.
- **FR-004**: System MUST support configurable forecast horizon via `--days` flag.
- **FR-005**: System MUST identify time windows where simultaneously scheduled tasks exceed available worker count when `--contention` flag is used.
- **FR-006**: Contention detection MUST account for estimated task durations, not just start times, to identify true overlaps.
- **FR-007**: System MUST calculate projected token usage and costs based on historical run averages when `--cost` flag is used.
- **FR-008**: System MUST gracefully handle tasks with no historical data by showing "no data" instead of failing.
- **FR-009**: System MUST support a `--dry-run` flag on `anvil add` that includes a hypothetical new task in the forecast without persisting it.
- **FR-010**: System MUST display a summary line showing total forecasted runs, total estimated runtime, and total estimated cost (when applicable).
- **FR-011**: System MUST exclude manual-only tasks (those without a cron schedule) from forecasts.
- **FR-012**: System MUST summarize output when the number of forecasted runs exceeds a reasonable threshold (e.g., group by day or hour) to avoid overwhelming output.

### Key Entities

- **Forecast**: A projected view of task executions over a time horizon, containing scheduled times, estimated durations, and optional cost estimates.
- **Contention Window**: A time period where the number of concurrently running tasks exceeds the available worker pool capacity.
- **Cost Projection**: An estimate of token usage and monetary cost derived from historical run record averages multiplied by forecasted run counts.
- **RunRecord**: Existing entity containing historical execution data including duration and token usage per task run.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view their complete task schedule for the next 7 days in under 2 seconds.
- **SC-002**: Users can identify resource contention windows before they occur, reducing task queue delays.
- **SC-003**: Users can estimate monthly costs within 20% accuracy of actual costs (given stable task behavior).
- **SC-004**: Users can evaluate the impact of a new task on their schedule without modifying their project configuration.
- **SC-005**: Forecast output remains readable and usable for projects with up to 100 configured tasks.

## Assumptions

- Token usage and cost data are available in existing RunRecord entries from `.anvil/runs/<task-id>/`.
- The worker pool size is available from project or daemon configuration.
- Cron schedule parsing is handled by the existing `internal/cron` package.
- Cost rates (price per token) will use sensible defaults that users can override in configuration.
- Task duration estimates are derived from the average of the last 10 runs (or all runs if fewer than 10 exist).
