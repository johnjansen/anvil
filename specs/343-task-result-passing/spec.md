# Feature Specification: Task Result Passing Between Dependent Tasks

**Feature Branch**: `343-task-result-passing`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add task result passing to pass output between dependent tasks. Currently there is no way to pass data from a completed task to its dependent. Users must use external storage (files, databases) to share results between tasks. Add native result passing via capture_output frontmatter, ##anvil:result output protocol, template variables, environment variables, and CLI visibility."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Capture and Pass Results to Dependent Task (Priority: P1)

As a user with chained tasks, I want a completed task to pass its output to the next dependent task so that I can build data pipelines without external storage.

**Why this priority**: This is the core value proposition — enabling native data flow between dependent tasks eliminates the primary pain point.

**Independent Test**: Can be fully tested by creating two tasks where the first captures output via `##anvil:result` and the second reads it via the `ANVIL_DEPENDENCY_RESULTS` environment variable.

**Acceptance Scenarios**:

1. **Given** a task with `capture_output: true` that prints `##anvil:result {"count": 42}`, **When** the task completes successfully, **Then** the result JSON is stored in the run record
2. **Given** a dependent task that reads `ANVIL_DEPENDENCY_RESULTS`, **When** it executes after its dependency completes, **Then** the environment variable contains a JSON object keyed by dependency task name with the captured result
3. **Given** a task with `capture_output: true` that prints no `##anvil:result` line, **When** the task completes, **Then** the result is stored as null and dependents receive null for that key
4. **Given** a task with `capture_output: true` that fails, **When** the dependent task would execute, **Then** the dependent does not execute (existing dependency behavior) and no result is passed

---

### User Story 2 - View Captured Results via CLI (Priority: P2)

As a user, I want to inspect captured task results so that I can debug data flow between tasks and verify outputs.

**Why this priority**: Visibility into captured results is essential for debugging and building confidence in the result passing system.

**Independent Test**: Can be fully tested by running a task with captured output and then using `anvil task results <task>` to view the stored data.

**Acceptance Scenarios**:

1. **Given** a task that has completed with captured results, **When** I run `anvil task results <task>`, **Then** I see the most recent captured result
2. **Given** a task with no captured results, **When** I run `anvil task results <task>`, **Then** I see a message indicating no results are available
3. **Given** a dependent task, **When** I run `anvil task results <task> --preview`, **Then** I see the dependency results that would be passed on the next run

---

### User Story 3 - Template Access to Dependency Results (Priority: P2)

As a user, I want to reference dependency results directly in my task body using template variables so that I can configure dependent tasks dynamically.

**Why this priority**: Template variables provide a convenient shorthand for accessing specific fields from dependency results without writing parsing code.

**Independent Test**: Can be fully tested by creating a dependent task that uses `{{ .DependencyResults.fetch-data.count }}` in its body and verifying the template is rendered with the actual value.

**Acceptance Scenarios**:

1. **Given** a dependent task body containing `{{ .DependencyResults.fetch-data.count }}`, **When** the task executes after fetch-data completes with `{"count": 42}`, **Then** the template renders as `42`
2. **Given** a template referencing a missing dependency result field, **When** the task executes, **Then** the template renders an empty string and the task proceeds

---

### User Story 4 - Cross-Project Result Passing (Priority: P3)

As a user with cross-project dependencies, I want results to pass across project boundaries so that I can build multi-project data pipelines.

**Why this priority**: Cross-project result passing extends the feature to multi-project setups, but most users will start with single-project workflows.

**Independent Test**: Can be fully tested by creating tasks in two different projects where one depends on the other, and verifying results flow across projects.

**Acceptance Scenarios**:

1. **Given** a task in project B that depends on `projectA:fetch-data`, **When** fetch-data in project A completes with captured results, **Then** project B's task receives those results via `ANVIL_DEPENDENCY_RESULTS`

---

### Edge Cases

- What happens when a task produces multiple `##anvil:result` lines? Only the last one is captured.
- What happens when `##anvil:result` contains invalid JSON? The raw string is stored as-is.
- What happens when dependency results exceed a reasonable size? Results are truncated at 1MB with a warning.
- What happens when a task has `capture_output: false` (default) but a dependent tries to read results? The dependent receives null for that dependency key.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support `capture_output: true` frontmatter option to enable result capture for a task
- **FR-002**: System MUST capture the last `##anvil:result <data>` line from task stdout as the task's result
- **FR-003**: System MUST store captured results in the task's run record
- **FR-004**: System MUST pass dependency results to dependent tasks via the `ANVIL_DEPENDENCY_RESULTS` environment variable as a JSON object keyed by dependency task name
- **FR-005**: System MUST support template variable `.DependencyResults.<task-name>` for accessing dependency results in task body
- **FR-006**: System MUST provide `anvil task results <task>` command to display the most recent captured result
- **FR-007**: System MUST provide `anvil task results <task> --preview` to show what dependency results a task would receive
- **FR-008**: System MUST support result passing for cross-project dependencies using the existing `project:task` syntax
- **FR-009**: System MUST handle missing or null results gracefully — dependents receive null for dependencies without captured results
- **FR-010**: System MUST limit captured result size to 1MB and warn when truncation occurs

### Key Entities

- **CapturedResult**: The result data extracted from a task's `##anvil:result` output line, stored as part of the RunRecord
- **DependencyResults**: A map of dependency task names to their captured results, passed to dependent tasks at execution time

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can set up result passing between two dependent tasks in under 5 minutes
- **SC-002**: Captured results are available to dependent tasks within 1 second of the producing task completing
- **SC-003**: `anvil task results` displays captured output within 1 second
- **SC-004**: Result passing works correctly for chains of 3+ dependent tasks (A -> B -> C)
- **SC-005**: Cross-project result passing works with the same reliability as local result passing
