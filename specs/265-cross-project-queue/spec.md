# Feature Specification: Cross-Project Dependency Status in Task Queue

**Feature Branch**: `265-cross-project-queue`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add cross-project dependency status to anvil task queue command"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View cross-project blocking dependencies (Priority: P1)

A user running `anvil task queue` wants to see which of their tasks are blocked by cross-project dependencies and what the current status of those dependencies is. When a task depends on a task in another project (e.g., `other-project:build-assets`), the queue output should clearly show this dependency and whether it is satisfied (last run succeeded) or blocking (last run failed/never ran).

**Why this priority**: This is the core value of the feature — without visibility into cross-project blockers, users cannot diagnose why tasks are stuck waiting.

**Independent Test**: Can be fully tested by creating a task with a cross-project dependency, running `anvil task queue`, and verifying the output shows the dependency project name, task name, and its last run status.

**Acceptance Scenarios**:

1. **Given** a task with a cross-project dependency where the dependency's last run succeeded, **When** the user runs `anvil task queue`, **Then** the output shows the dependency with a "satisfied" or "success" status indicator.
2. **Given** a task with a cross-project dependency where the dependency's last run failed, **When** the user runs `anvil task queue`, **Then** the output shows the dependency as blocking with a "failed" status indicator.
3. **Given** a task with a cross-project dependency where the dependency has never run, **When** the user runs `anvil task queue`, **Then** the output shows the dependency as blocking with a "no runs" status indicator.

---

### User Story 2 - Filter queue to show cross-project items with --all flag (Priority: P2)

A user wants to see cross-project queue items alongside local tasks. By default, the queue shows only the current project's tasks. With `--all`, the queue additionally shows cross-project dependency entries so the user gets a complete picture of what is blocking execution across projects.

**Why this priority**: Enhances the core feature by giving users a broader view, but the basic cross-project status display (P1) delivers value on its own.

**Independent Test**: Can be tested by running `anvil task queue --all` and verifying that cross-project dependency tasks appear in the output alongside local tasks.

**Acceptance Scenarios**:

1. **Given** a project with tasks that have cross-project dependencies, **When** the user runs `anvil task queue --all`, **Then** cross-project dependency tasks are shown in the queue output with their project name, status, and last run result.
2. **Given** a project with no cross-project dependencies, **When** the user runs `anvil task queue --all`, **Then** the output is identical to `anvil task queue` (no extra entries).

---

### User Story 3 - Cross-project dependency info in JSON output (Priority: P2)

A user or script consuming the JSON output of `anvil task queue --json` needs cross-project dependency information included in the structured data for programmatic analysis or dashboard integration.

**Why this priority**: Enables tooling and automation around cross-project dependencies but is not required for human-readable queue inspection.

**Independent Test**: Can be tested by running `anvil task queue --json` and verifying the JSON includes cross-project dependency fields.

**Acceptance Scenarios**:

1. **Given** a task with cross-project dependencies, **When** the user runs `anvil task queue --json`, **Then** the JSON output includes dependency information with project name, task name, and last run status for each cross-project dependency.

---

### Edge Cases

- What happens when a cross-project dependency references a project that is not in the watched directory? The queue should show the dependency with an "unknown project" status rather than crashing.
- What happens when a cross-project dependency references a task that does not exist in the target project? The queue should show the dependency with a "task not found" status.
- What happens when the target project's daemon is not running? The queue should still show the dependency using the last known run record from disk, since run records are stored as JSON files and don't require the daemon.
- What happens when a task has both local and cross-project dependencies? Both types should be shown clearly, with cross-project ones distinguished by their project prefix.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `anvil task queue` command MUST display cross-project dependency information for any task that has cross-project dependencies, including the dependency's project name, task name, and last run status.
- **FR-002**: The queue output MUST indicate whether each cross-project dependency is currently blocking (failed/never run) or satisfied (last run succeeded).
- **FR-003**: The `--all` flag MUST include cross-project dependency entries in the queue output, showing them as distinct items with their source project clearly labeled.
- **FR-004**: The `--json` output MUST include cross-project dependency data in the structured response for each task that has cross-project dependencies.
- **FR-005**: The queue MUST gracefully handle unreachable or unknown cross-project dependencies by displaying an appropriate status (e.g., "unknown project", "task not found") rather than failing.
- **FR-006**: Cross-project dependency status MUST be resolved using existing run records on disk, not requiring the remote project's daemon to be running.

### Key Entities

- **Cross-Project Dependency**: A dependency reference in the format `project:task` that links a local task to a task in another watched project. Key attributes: project name, task name, resolution status, last run outcome.
- **Queue Entry**: An item in the task queue display. Extended to include cross-project dependency metadata: list of cross-project deps with their statuses.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can identify all cross-project blockers for any task in the queue within a single command invocation.
- **SC-002**: Cross-project dependency status is visible for 100% of tasks that have cross-project dependencies when running `anvil task queue`.
- **SC-003**: The `--all` flag shows cross-project dependency entries alongside local tasks without requiring users to inspect each task individually.
- **SC-004**: JSON output includes complete cross-project dependency information, enabling programmatic consumption without additional commands.

## Assumptions

- Cross-project dependency parsing and validation infrastructure is already implemented (#259).
- The `Dependency`, `ParseDependency`, and `ResolveDependencyRunRecord` functions in `internal/project/dependencies.go` provide the foundation for resolving cross-project dependency status.
- Run records for cross-project tasks are accessible on disk via the watched project path resolution mechanism.
- The existing `TaskQueueInfo` struct will be extended with cross-project dependency fields.
