# Feature Specification: File Watcher Trigger for Tasks

**Feature Branch**: `364-file-watch-trigger`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add file watcher trigger for tasks"
**Dependency**: Requires trigger framework from #363

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Watch a Directory for New Data Files (Priority: P1)

A user has a task that processes incoming data files. They configure the task with a `file_watch` trigger pointing at a directory with a glob pattern. When a new file matching the pattern appears, the task runs automatically.

**Why this priority**: This is the core use case — reacting to file creation events. Without it, the feature delivers no value.

**Independent Test**: Can be fully tested by creating a task with `file_watch` trigger, dropping a matching file into the watched directory, and verifying the task executes.

**Acceptance Scenarios**:

1. **Given** a task configured with `trigger: { type: file_watch, path: ./data/*.json, events: [create] }`, **When** a new `.json` file is created in `./data/`, **Then** the task is triggered and executes.
2. **Given** a task configured to watch `./data/*.json`, **When** a `.csv` file is created in `./data/`, **Then** the task is NOT triggered.
3. **Given** a task configured to watch `./data/*.json`, **When** the daemon is not running, **Then** file changes are not detected and tasks are not triggered.

---

### User Story 2 - Debounce Rapid File Changes (Priority: P1)

A user has a build tool that writes many files in quick succession. The task should wait until file changes settle before running, to avoid triggering multiple times for a single logical operation.

**Why this priority**: Without debounce, the feature would cause excessive task executions during bulk file operations, making it impractical for real-world use.

**Independent Test**: Can be fully tested by creating a task with a debounce setting, rapidly creating multiple files, and verifying the task only executes once after the debounce period.

**Acceptance Scenarios**:

1. **Given** a task with `debounce: 5s`, **When** 10 files are created within 3 seconds, **Then** the task runs exactly once, approximately 5 seconds after the last file change.
2. **Given** a task with `debounce: 2s`, **When** a file is created, then another 1 second later, then another 1 second later, **Then** the task runs once, 2 seconds after the third file change.
3. **Given** a task with no debounce configured, **When** a file is created, **Then** the task runs with a reasonable default debounce (e.g., 1 second).

---

### User Story 3 - React to File Modifications and Deletions (Priority: P2)

A user wants to trigger a task when configuration files are modified or when files are deleted from a directory.

**Why this priority**: Supporting modify and delete events broadens the use cases but is not essential for the initial file-watching capability.

**Independent Test**: Can be fully tested by configuring a task to watch for `modify` events, modifying a matching file, and verifying the task executes.

**Acceptance Scenarios**:

1. **Given** a task configured with `events: [modify]`, **When** an existing watched file is modified, **Then** the task is triggered.
2. **Given** a task configured with `events: [delete]`, **When** a watched file is deleted, **Then** the task is triggered.
3. **Given** a task configured with `events: [create]` only, **When** a file is modified, **Then** the task is NOT triggered.

---

### User Story 4 - Pass Changed File Information to Task (Priority: P2)

When a task is triggered by a file change, the user wants to know which file changed and what type of event occurred, so the task script can act on the specific file.

**Why this priority**: Enables targeted processing of changed files rather than re-scanning everything, but the feature is still useful without it.

**Independent Test**: Can be fully tested by triggering a task via file change and checking that the task receives file path and event type information through environment variables or similar mechanism.

**Acceptance Scenarios**:

1. **Given** a task triggered by a file creation event, **When** the task executes, **Then** the changed file path is available to the task.
2. **Given** a task triggered by a debounced batch of changes, **When** the task executes, **Then** all changed file paths from the batch are available to the task.

---

### User Story 5 - Watcher Lifecycle Management (Priority: P2)

The daemon should start file watchers when tasks are loaded and stop them when tasks are removed or the daemon shuts down. Watchers should not consume resources for inactive tasks.

**Why this priority**: Proper lifecycle management prevents resource leaks but is an operational concern rather than a core feature.

**Independent Test**: Can be fully tested by starting the daemon with a file-watch task, verifying the watcher is active, then removing the task and verifying the watcher stops.

**Acceptance Scenarios**:

1. **Given** a daemon with file-watch tasks, **When** the daemon starts, **Then** file watchers are created for each configured task.
2. **Given** a running daemon with file watchers, **When** the daemon is stopped, **Then** all file watchers are cleanly shut down.
3. **Given** a running daemon, **When** a file-watch task is added dynamically, **Then** a new watcher is started for it.

---

### Edge Cases

- What happens when the watched directory does not exist at daemon startup? The daemon should log a warning and retry periodically, or start watching once the directory is created.
- What happens when a file is created and immediately deleted before the debounce period ends? The event should still be reported if `create` is in the events list.
- What happens when the same file triggers multiple event types (e.g., create then modify)? Both events should be tracked but debounce should collapse them into a single task execution.
- What happens when a watched path is a symlink? The watcher should follow the symlink and watch the target.
- What happens when file permissions prevent reading the changed file? The task should still trigger, but the changed file info should note the permission issue.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a `file_watch` trigger type in task frontmatter configuration.
- **FR-002**: System MUST accept glob patterns for specifying which files to watch (e.g., `./data/*.json`, `./config/**/*.yaml`).
- **FR-003**: System MUST support filtering by event type: `create`, `modify`, and `delete`.
- **FR-004**: System MUST support a configurable debounce duration to prevent rapid-fire task executions.
- **FR-005**: System MUST apply a default debounce of 1 second when no debounce is specified.
- **FR-006**: System MUST pass changed file information (file path and event type) to the triggered task.
- **FR-007**: System MUST start file watchers when the daemon starts and tasks with file_watch triggers are loaded.
- **FR-008**: System MUST stop file watchers cleanly when the daemon shuts down or tasks are removed.
- **FR-009**: System MUST use platform-native filesystem notification mechanisms rather than polling for file changes.
- **FR-010**: System MUST integrate with the trigger framework established by #363.
- **FR-011**: System MUST log watcher start, stop, and trigger events for observability.
- **FR-012**: System MUST handle the case where a watched directory does not exist by logging a warning and watching for its creation.

### Key Entities

- **FileWatchTrigger**: A trigger configuration specifying the watch path (glob pattern), event types to monitor, and debounce duration. Extends the trigger framework from #363.
- **FileEvent**: A record of a detected file change, including the file path, event type (create/modify/delete), and timestamp.
- **FileWatcher**: A runtime component managed by the daemon that monitors a configured path and emits file events.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tasks configured with file_watch triggers execute within 2 seconds of a file change (plus debounce time).
- **SC-002**: Debounce correctly collapses multiple rapid file changes into a single task execution 100% of the time.
- **SC-003**: File watchers consume negligible system resources when idle (no polling overhead when using native OS notifications).
- **SC-004**: Users can configure a complete file-watch trigger in under 1 minute using the frontmatter syntax.
- **SC-005**: Watcher lifecycle is fully managed — no resource leaks after daemon restart or task removal.

## Assumptions

- The trigger framework (#363) provides a plugin/registration mechanism for new trigger types that this feature will use.
- The existing `TaskTrigger` struct in `internal/project/trigger.go` will be extended or composed with to support the new `file_watch` type.
- A Go library for cross-platform filesystem notifications (e.g., fsnotify) will be used rather than implementing platform-specific code directly.
- The debounce default of 1 second is reasonable for most use cases; users can override via configuration.
- File watchers operate within the daemon process; no separate watcher service is needed.
