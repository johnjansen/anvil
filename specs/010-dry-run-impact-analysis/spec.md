# Feature Specification: Task Dry-Run Impact Analysis

**Feature Branch**: `010-dry-run-impact-analysis`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task dry-run impact analysis before adding new tasks"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Dry-Run Shows Impact (Priority: P1)

A user wants to see the impact of a new task before adding it to their schedule. They run `anvil add --dry-run -s "0 9 * * *" "My new task"` and see a formatted impact analysis showing conflicts, cost estimate, and worker load without actually creating the task.

**Why this priority**: This is the core value proposition — allowing users to preview impact before committing. Without this, users must add the task first to see conflicts.

**Independent Test**: Can be tested by running `anvil add --dry-run` and verifying it shows impact analysis without creating a task file.

**Acceptance Scenarios**:

1. **Given** a new task with schedule "0 9 * * *" that conflicts with 3 existing tasks, **When** the user runs `anvil add --dry-run`, **Then** the output shows "Conflicts: Conflicts with 3 tasks at 09:00" with the task names listed.
2. **Given** a new task with schedule "0 9 * * *" that has no conflicts, **When** the user runs `anvil add --dry-run`, **Then** the output shows "Conflicts: None" and proceeds without showing conflict details.
3. **Given** a new task without `--dry-run`, **When** the user adds the task normally, **Then** the task is created normally (existing behavior unchanged).

---

### User Story 2 - Cost Estimation (Priority: P1)

A user wants to know the estimated monthly cost before adding a task. The system estimates cost based on task content size, assuming average token density and current pricing.

**Why this priority**: Cost visibility is a key user request for impact analysis.

**Independent Test**: Can be tested by running `anvil add --dry-run` and verifying a cost estimate is shown.

**Acceptance Scenarios**:

1. **Given** a new task with ~500 words of content, **When** the user runs `anvil add --dry-run`, **Then** the output shows "Monthly Cost: +$X.XX (estimated)" based on ~1000 tokens per execution and 30 executions/month.
2. **Given** a new task with no schedule (one-shot), **When** the user runs `anvil add --dry-run`, **Then** the cost estimate is shown as "One-shot" with per-run estimate instead of monthly.
3. **Given** a task with `max_concurrent` > 1, **When** the user runs `anvil add --dry-run`, **Then** the cost estimate accounts for potential parallel executions.

---

### User Story 3 - Alternative Schedule Suggestions (Priority: P2)

A user sees conflicts with their proposed schedule and wants alternatives. The system suggests alternate schedules that avoid conflicts.

**Why this priority**: Helps users make informed decisions about scheduling to avoid conflicts.

**Independent Test**: Can be tested by adding a conflicting task and verifying alternative schedules are suggested.

**Acceptance Scenarios**:

1. **Given** a schedule "0 9 * * *" with 3 conflicts, **When** the user runs `anvil add --dry-run`, **Then** the output shows "Suggested alternatives to avoid conflicts" with at least 2-3 alternative schedules.
2. **Given** a schedule with no conflicts, **When** the user runs `anvil add --dry-run`, **Then** no alternative schedules are shown.
3. **Given** a complex schedule like "*/15 * * * *" (every 15 min), **When** conflicts exist, **Then** the suggestion explains the difficulty and suggests increasing worker count.

---

### User Story 4 - Interactive Confirmation (Priority: P2)

After seeing impact analysis, the user can proceed with adding the task or cancel.

**Why this priority**: Completes the dry-run workflow by allowing user action after reviewing impact.

**Independent Test**: Can be tested by running `anvil add --dry-run` and confirming the task is created when user responds "y".

**Acceptance Scenarios**:

1. **Given** `anvil add --dry-run` shows impact, **When** the user types "y" or "yes", **Then** the task is created normally.
2. **Given** `anvil add --dry-run` shows impact, **When** the user types "n" or "no" or presses Enter, **Then** no task is created and the command exits.
3. **Given** `anvil add --dry-run` without TTY (non-interactive), **When** the impact shows conflicts, **Then** the command exits with error (can't prompt for confirmation).

---

### User Story 5 - Worker Load Analysis (Priority: P3)

A user wants to see how the new task affects worker utilization at the scheduled time.

**Why this priority**: Helps users understand potential bottlenecks in task execution.

**Independent Test**: Can be tested by adding a task and verifying worker load percentage is shown.

**Acceptance Scenarios**:

1. **Given** 10 tasks already scheduled at 09:00 and max_workers=10, **When** user adds another task at 09:00, **Then** "Worker Load: +10% at 09:00" is shown.
2. **Given** max_workers is not configured (default unlimited), **When** user runs `anvil add --dry-run`, **Then** worker load shows "N/A (unlimited workers)".

---

## Edge Cases

- What happens when the task content is empty? Show error "task content cannot be empty" before impact analysis.
- What happens when the schedule is invalid? Show parse error before impact analysis.
- What happens when there are 20+ conflicts? Truncate list to top 10 and show "and X more".
- What happens when running in CI/non-interactive mode with `--dry-run`? Exit with code 0 if no conflicts, exit with code 1 if conflicts exist (for scriptable use).
- What happens when cost estimation is impossible (e.g., persistent task)? Show "Cost: Variable" instead of estimate.
- What happens with `--dry-run` but no schedule? Still show cost estimate for one-shot, no conflicts.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support `--dry-run` (or `-n`) flag on `anvil add` that shows impact without creating the task.
- **FR-002**: System MUST show scheduling conflicts with existing tasks when `--dry-run` is used, including task names and their schedules.
- **FR-003**: System MUST show estimated monthly cost based on task content size and assumed token density (~4 chars per token).
- **FR-004**: System MUST calculate monthly runs based on cron schedule (e.g., daily=30, hourly=720) or show one-shot cost.
- **FR-005**: System MUST suggest 2-3 alternative schedules that avoid conflicts when conflicts exist.
- **FR-006**: System MUST prompt user to confirm after showing impact analysis in interactive mode.
- **FR-007**: System MUST exit gracefully without creating task when user declines or in non-interactive mode with conflicts.
- **FR-008**: System MUST create the task normally when user confirms with "y" or "yes".
- **FR-009**: System MUST show worker load impact at the scheduled time based on concurrent task count.
- **FR-010**: System MUST preserve all existing `anvil add` behavior when `--dry-run` is not specified.

### Key Entities

- **ImpactAnalysis**: A structured result containing conflicts, cost estimate, worker load, and suggested alternatives.
- **ConflictInfo**: Details about a scheduling conflict including task name and schedule.
- **CostEstimate**: Calculated cost with per-run and monthly projections.
- **AlternativeSchedule**: A suggested cron expression that avoids conflicts.

## Success Criteria *(mandurable)*

### Measurable Outcomes

- **SC-001**: Users can run `anvil add --dry-run` and see impact within 1 second.
- **SC-002**: Users can identify scheduling conflicts before adding a task.
- **SC-003**: Users can make informed decisions about scheduling based on cost and conflict data.
- **SC-004**: Users can proceed with adding a task after reviewing impact by confirming the prompt.
- **SC-005**: Existing `anvil add` workflows remain unchanged (backward compatible).

## Assumptions

- Token estimation assumes ~4 characters per token for content input.
- Cost calculation uses current config rates: input_token_rate ($3.00/1M) and output_token_rate ($15.00/1M).
- Monthly estimates assume ~30 executions for daily schedules, ~720 for hourly.
- Worker load percentage is calculated as (tasks at same time / max_workers) * 100, defaulting to "unlimited" when max_workers is not set.
- Alternative schedules are generated by offsetting minute field by 5, 10, 15 minutes and filtering for non-conflicting options.
