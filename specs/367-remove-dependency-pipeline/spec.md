# Feature Specification: Remove Task Dependency Pipeline

**Feature Branch**: `367-remove-dependency-pipeline`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Remove task dependency pipeline feature — depends_on is broken and unusable"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Clean removal of depends_on frontmatter (Priority: P1)

A developer currently has task files that use `depends_on` frontmatter fields. After this change, the system must ignore or reject `depends_on` fields without crashing. Existing task files that happen to still contain `depends_on` should continue to function — the field is simply ignored.

**Why this priority**: The core deliverable is removing the broken feature. Tasks must not break if stale `depends_on` fields remain in user task files.

**Independent Test**: Create a task file with a `depends_on` field, start the daemon, and verify the task runs on its own cron schedule without errors or dependency-checking behavior.

**Acceptance Scenarios**:

1. **Given** a task file with `depends_on: other-task`, **When** the daemon evaluates the task's schedule, **Then** the dependency field is ignored and the task runs based on its own cron schedule alone.
2. **Given** a task file with `depends_on: other-project:other-task` (cross-project syntax), **When** the daemon loads the task, **Then** the field is ignored without errors.

---

### User Story 2 - Removal of pipeline CLI command (Priority: P1)

A user who previously ran `anvil task pipeline` to visualize dependency chains should receive a clear error or help message indicating the command no longer exists.

**Why this priority**: Removing dead CLI surface area prevents confusion and keeps the CLI clean.

**Independent Test**: Run `anvil task pipeline` and verify it returns a "command not found" or equivalent error rather than crashing or producing stale output.

**Acceptance Scenarios**:

1. **Given** the updated CLI, **When** a user runs `anvil task pipeline`, **Then** the system reports an unknown command error.
2. **Given** the updated CLI, **When** a user runs `anvil help`, **Then** no pipeline-related commands or flags appear in the help output.

---

### User Story 3 - Removal of internal dependency types and logic (Priority: P1)

The codebase no longer contains `ParseDependency`, `ResolveDependencyRunRecord`, or `Dependency` types. The daemon's tick evaluation no longer checks dependencies. All related test code is removed.

**Why this priority**: Dead code removal reduces maintenance burden and prevents confusion for contributors.

**Independent Test**: Search the codebase for dependency-related types and functions; confirm none exist. Run the full test suite and verify all tests pass.

**Acceptance Scenarios**:

1. **Given** the updated codebase, **When** searching for `ParseDependency`, `ResolveDependencyRunRecord`, or `Dependency` type definitions, **Then** no results are found.
2. **Given** the updated codebase, **When** running the full test suite, **Then** all tests pass with no dependency-related test failures.

---

### User Story 4 - Documentation cleanup (Priority: P2)

All documentation references to `depends_on`, task pipelines, and cross-project dependencies are removed from SKILL.md, CLAUDE.md, CLI help text, and any other documentation files.

**Why this priority**: Documentation must reflect the actual feature set to avoid misleading users and contributors.

**Independent Test**: Search all documentation files for references to `depends_on`, `pipeline`, `ParseDependency`, and cross-project dependency syntax; confirm none remain.

**Acceptance Scenarios**:

1. **Given** the updated documentation, **When** searching for `depends_on` references, **Then** no results are found in documentation files.
2. **Given** the updated CLAUDE.md, **When** reviewing Active Technologies and Recent Changes sections, **Then** no references to dependency pipeline features remain.

---

### Edge Cases

- What happens when a user has task files with `depends_on` fields after the update? The field is silently ignored.
- What happens if third-party tooling or scripts reference `anvil task pipeline`? The command returns a standard "unknown command" error with exit code 1.
- What happens to run history records that were created by dependency-triggered runs? They remain intact — historical data is not modified.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST remove the `depends_on` frontmatter field parsing from task file loading.
- **FR-002**: System MUST remove the `ParseDependency`, `ResolveDependencyRunRecord`, and `Dependency` types from `internal/project`.
- **FR-003**: System MUST remove dependency-checking logic from the daemon's task schedule evaluation.
- **FR-004**: System MUST remove the `anvil task pipeline` CLI command and its implementation (`task_pipeline.go`).
- **FR-005**: System MUST remove cross-project dependency support (the `project:task` syntax).
- **FR-006**: System MUST remove all tests related to dependency resolution and pipeline visualization.
- **FR-007**: System MUST update all documentation (SKILL.md, CLAUDE.md, CLI help text) to remove references to dependency pipelines.
- **FR-008**: System MUST silently ignore any `depends_on` fields found in existing task files without producing errors.
- **FR-009**: System MUST continue to function correctly for all non-dependency features after the removal.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero references to `ParseDependency`, `ResolveDependencyRunRecord`, or `Dependency` types exist in the codebase after removal.
- **SC-002**: The `anvil task pipeline` command is no longer available in the CLI.
- **SC-003**: All existing tests pass after the removal (excluding removed dependency-specific tests).
- **SC-004**: Task files containing stale `depends_on` fields load and execute without errors.
- **SC-005**: Documentation contains zero references to task dependency pipelines or `depends_on` configuration.

## Assumptions

- Historical run records created by dependency-triggered runs are left intact and not modified.
- The `depends_on` field in frontmatter is silently ignored rather than producing a warning, since warnings would create noise for users who haven't yet cleaned up their task files.
- No migration tool is needed — users simply remove `depends_on` from their task files at their convenience.
