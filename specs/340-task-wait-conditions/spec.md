# Feature Specification: Task Wait Conditions for Multi-Criteria Triggering

**Feature Branch**: `340-task-wait-conditions`
**Created**: 2026-03-02
**Status**: Draft
**Input**: User description: "Add task wait conditions for multi-criteria triggering. Currently, tasks trigger based on schedule OR manual trigger. Users need more sophisticated triggering: Run after BOTH schedule AND file exists, Run when queue has items (not just cron), Run based on external state changes. Add wait conditions with multiple trigger types including AND/OR logic, polling triggers, and manual trigger evaluation."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Complex Task Triggering (Priority: P1)

As a user, I want to trigger tasks based on multiple conditions so that I can automate complex workflows that depend on various system states.

**Why this priority**: This is the core functionality that enables sophisticated automation beyond simple scheduling.

**Independent Test**: Can be fully tested by creating a task with multiple trigger conditions and verifying it executes when all conditions are met.

**Acceptance Scenarios**:

1. **Given** a task with both schedule and file existence conditions, **When** the schedule time arrives AND the file exists, **Then** the task should execute
2. **Given** a task with multiple AND conditions, **When** only some conditions are met, **Then** the task should not execute
3. **Given** a task with OR conditions, **When** any one condition is met, **Then** the task should execute

### User Story 2 - Polling-Based Triggers (Priority: P2)

As a user, I want to trigger tasks based on polling conditions so that I can react to external state changes that don't have explicit notifications.

**Why this priority**: This enables reacting to external system changes without requiring webhook integrations.

**Independent Test**: Can be fully tested by creating a task with polling conditions and verifying it executes when the polled condition becomes true.

**Acceptance Scenarios**:

1. **Given** a task with a file polling condition, **When** the file appears after the task starts polling, **Then** the task should execute once
2. **Given** a task with polling and timeout conditions, **When** the timeout expires before the condition is met, **Then** the task should not execute

### User Story 3 - Manual Trigger Evaluation (Priority: P3)

As a user, I want to manually evaluate trigger conditions so that I can test and debug my task configurations.

**Why this priority**: This provides debugging capabilities and manual control over task execution.

**Independent Test**: Can be fully tested by running the manual trigger check command and verifying it evaluates conditions correctly.

**Acceptance Scenarios**:

1. **Given** a task with complex trigger conditions, **When** I run `anvil task trigger-check`, **Then** I should see the evaluation results

### Edge Cases

- What happens when polling conditions conflict with scheduled conditions?
- How does the system handle polling intervals shorter than the time it takes to evaluate conditions?
- What happens when a task with polling conditions times out?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support defining multiple trigger conditions for tasks
- **FR-002**: System MUST support AND logic where all conditions must be true for task execution
- **FR-003**: System MUST support OR logic where any condition can trigger task execution
- **FR-004**: System MUST support file existence conditions
- **FR-005**: System MUST support environment variable conditions
- **FR-006**: System MUST support polling-based conditions with configurable intervals
- **FR-007**: System MUST support timeout configuration for polling conditions
- **FR-008**: System MUST provide a command to manually evaluate trigger conditions
- **FR-009**: System MUST execute tasks only once when polling conditions are met

### Key Entities

- **TaskTrigger**: Represents the trigger configuration for a task with multiple conditions
- **Condition**: Represents a single condition that must be evaluated (file exists, env var set, etc.)
- **PollingCondition**: Represents a condition that is evaluated periodically until met or timeout

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can configure tasks with multiple trigger conditions in under 5 minutes
- **SC-002**: System correctly evaluates AND/OR logic for trigger conditions with 99.9% accuracy
- **SC-003**: Polling-based triggers respond to condition changes within the configured interval time plus 10%
- **SC-004**: Manual trigger evaluation provides clear feedback on condition status within 2 seconds
