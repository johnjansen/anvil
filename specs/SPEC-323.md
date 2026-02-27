# Feature Specification: Task Fan-in/Fan-out Patterns for Parallel Execution

**Feature Branch**: `[323-task-fan-out]`
**Created**: 2026-02-28
**Status**: Draft
**Input**: User description: "Add task fan-in/fan-out patterns for parallel execution"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Fan-out: Spawn Parallel Children (Priority: P1)

User wants a single task to trigger multiple parallel child tasks that run simultaneously, with the parent task completing when all children complete.

**Why this priority**: Core feature - enables parallel processing of independent workloads (e.g., processing multiple repositories, data sources, or entities in parallel).

**Independent Test**: Can be tested by creating a parent task with `spawn:` list and verifying children run in parallel and parent waits for all.

**Acceptance Scenarios**:

1. **Given** a task with `spawn: [task-a, task-b, task-c]` in frontmatter, **When** parent task runs, **Then** tasks task-a, task-b, and task-c spawn in parallel
2. **Given** spawned children are running, **When** parent checks completion status, **Then** parent waits until ALL children complete before marking itself complete
3. **Given** one child task fails, **When** parent has `continue_on_error: true`, **Then** parent continues waiting for other children, then completes with partial failure status
4. **Given** one child fails without `continue_on_error`, **When** child fails, **Then** parent fails with first child failure reason

---

### User Story 2 - Dynamic Spawning via Output (Priority: P1)

User wants a task to dynamically decide at runtime what other tasks to trigger based on its output.

**Why this priority**: Enables dynamic workflows where the number and type of child tasks isn't known at configuration time (e.g., process each file found, each user found, etc.).

**Independent Test**: Can be tested by running a task that outputs `##anvil:spawn` directives and verifying corresponding tasks are created.

**Acceptance Scenarios**:

1. **Given** a task outputs `##anvil:spawn process-data --repo=repo-a` to stdout, **When** task runs, **Then** a new task instance is created with those arguments
2. **Given** multiple `##anvil:spawn` lines in output, **When** task completes, **Then** all specified tasks are spawned
3. **Given** a spawned child task fails, **When** parent task has no error handling, **Then** parent continues and spawns remaining children

---

### User Story 3 - Fan-in: Multiple Dependencies (Priority: P1)

User wants a task to wait for multiple dependent tasks to complete before starting (already supported via `depends_on` list).

**Why this priority**: Required for pipeline patterns where multiple parallel branches must converge before the next stage.

**Independent Test**: Already testable via existing `depends_on` functionality.

**Acceptance Scenarios**:

1. **Given** a task with `depends_on: [task-a, task-b, task-c]`, **When** task-a, task-b, and task-c are all running, **Then** dependent task waits until all three complete successfully
2. **Given** one dependency fails, **When** dependent task is scheduled, **Then** dependent task fails immediately without running

---

### User Story 4 - Task Tree Visibility (Priority: P1)

User wants to see the parent/child relationship tree of spawned tasks.

**Why this priority**: Provides visual clarity on task hierarchies and debugging support for complex workflows.

**Independent Test**: Can be tested by running `anvil task tree <parent>` and verifying the tree structure displays.

**Acceptance Scenarios**:

1. **Given** a parent task with spawned children, **When** `anvil task tree <parent>` is run, **Then** output shows hierarchical tree with parent at root and children as leaf nodes
2. **Given** a task with no children, **When** `anvil task tree <task>` is run, **Then** output shows just the single task (no tree)
3. **Given** deeply nested spawned tasks (parent → child → grandchild), **When** tree is displayed, **Then** all levels are shown with proper indentation

---

### User Story 5 - Fan-out Status in Process List (Priority: P2)

User wants to see fan-out status in the process list (`anvil ps`).

**Why this priority**: Provides visibility into parallel task execution without needing to run separate tree command.

**Independent Test**: Can be tested by running `anvil ps` with active fan-out and verifying children are shown.

**Acceptance Scenarios**:

1. **Given** a parent with active child tasks, **When** `anvil ps` is run, **Then** child tasks show parent reference in STATUS or INFO column
2. **Given** a parent task with 3 children (2 complete, 1 running), **When** parent status is shown, **Then** it shows progress like "3 children: 2 done, 1 running"

---

### User Story 6 - Stop Children on Parent Cancel (Priority: P2)

User wants child tasks to be cancelled when parent is cancelled.

**Why this priority**: Prevents orphaned tasks from continuing to run after parent is no longer needed.

**Independent Test**: Can be tested by starting a parent with children, then cancelling the parent.

**Acceptance Scenarios**:

1. **Given** a parent with running child tasks, **When** parent is cancelled (Ctrl+C or `anvil task kill`), **Then** all active child tasks are also cancelled
2. **Given** a child task completes after parent cancellation, **When** it was already running, **Then** its output is recorded but marked as orphaned

---

### Edge Cases

- What happens when a spawned task has the same name as an existing task? → Create new instance with unique ID
- What happens when child task spawns its own children? → Support nesting up to 5 levels deep
- What happens when parent task has both `spawn` and `depends_on`? → `spawn` runs first, then parent waits on `depends_on`
- How does priority inheritance work? → Children inherit parent priority by default, can override
- What happens when daemon restarts with active fan-out? → Rebuild parent-child relationships from persisted state

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tasks MUST support `spawn:` frontmatter field containing a list of task names to spawn in parallel
- **FR-002**: Spawned child tasks MUST run in parallel (not sequential)
- **FR-003**: Parent task MUST wait for ALL children to complete before completing itself
- **FR-004**: Task output containing `##anvil:spawn <task-name> [--args]` lines MUST trigger dynamic spawning
- **FR-005**: `anvil task tree <task>` command MUST display parent/child hierarchy
- **FR-006**: Child tasks MUST show parent reference in `anvil ps` output
- **FR-007**: Parent cancellation MUST propagate to all active child tasks
- **FR-008**: Fan-out relationships MUST be persisted and survive daemon restart
- **FR-009**: `depends_on:` with multiple tasks MUST support fan-in pattern (already exists)

### Key Entities *(include if feature involves data)*

- **TaskSpawn**: Represents a spawned child task, links to parent Task via ParentID
- **TaskTree**: In-memory structure tracking parent-child relationships for a spawn group
- **SpawnConfig**: New config field in Task for `spawn:` list

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A parent task with 3 spawned children completes only after all 3 children finish
- **SC-002**: Tasks spawned via `##anvil:spawn` appear in task queue with correct arguments
- **SC-003**: `anvil task tree` correctly shows parent → children hierarchy up to 5 levels
- **SC-004**: `anvil ps` shows child tasks with parent reference
- **SC-005**: Cancelling a parent with active children cancels all children within 5 seconds

