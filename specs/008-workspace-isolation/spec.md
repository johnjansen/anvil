# Feature Specification: Task Workspace Isolation

**Feature Branch**: `008-workspace-isolation`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task workspace isolation for secure file access"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Restrict Task File Access to Specific Directories (Priority: P1)

As a project owner running multiple tasks in a shared project, I want to restrict which directories a task can read and write so that tasks cannot accidentally corrupt each other's data or access sensitive files.

**Why this priority**: This is the core value proposition — path-based access control prevents the most common damage scenario (a runaway or untrusted task writing to files it shouldn't). Without this, all other isolation features have limited value.

**Independent Test**: Can be fully tested by creating a task with `workspace.allowed_paths` configured, running it, and verifying it can only access the specified directories. Delivers immediate security value for multi-task projects.

**Acceptance Scenarios**:

1. **Given** a task with `workspace.allowed_paths: [./data/, ./output/]`, **When** the task attempts to write to `./data/result.txt`, **Then** the write succeeds normally.
2. **Given** a task with `workspace.allowed_paths: [./data/]`, **When** the task attempts to write to `./src/main.go`, **Then** the write is blocked and the task receives an error.
3. **Given** a task with `workspace.read_only: [./config/]`, **When** the task attempts to read `./config/settings.yaml`, **Then** the read succeeds, but writing to that path is blocked.
4. **Given** a task with `workspace.blocked_paths: [~/.ssh/]`, **When** the task attempts to read `~/.ssh/id_rsa`, **Then** the read is blocked regardless of other path settings.

---

### User Story 2 - Run Tasks in Temporary Isolated Workspace (Priority: P2)

As a user running untrusted or experimental prompts, I want tasks to execute in an isolated temporary directory so that they cannot affect the project directory at all, and any files they create are cleaned up after execution.

**Why this priority**: Temporary workspaces provide the strongest isolation for untrusted tasks. This builds on P1 by offering a "total isolation" option rather than fine-grained path control.

**Independent Test**: Can be tested by creating a task with `workspace.type: temp`, running it, and verifying it executes in a temporary directory that is removed after completion.

**Acceptance Scenarios**:

1. **Given** a task with `workspace.type: temp`, **When** the task executes, **Then** it runs in a fresh temporary directory outside the project tree.
2. **Given** a task with `workspace.type: temp`, **When** the task completes, **Then** the temporary directory and its contents are removed.
3. **Given** a task with `workspace.type: temp` and `workspace.size: 100mb`, **When** the task writes more than 100MB of data, **Then** the task receives a disk space error.

---

### User Story 3 - View Workspace Configuration (Priority: P2)

As a project administrator, I want to see the workspace configuration for any task so that I can audit and verify security settings before running tasks.

**Why this priority**: Visibility into workspace settings is essential for auditing and debugging. Users need to confirm their restrictions are in effect.

**Independent Test**: Can be tested by running `anvil task get <name>` on a task with workspace config and verifying the output includes workspace details.

**Acceptance Scenarios**:

1. **Given** a task with workspace configuration, **When** the user runs `anvil task get <name>`, **Then** the output includes the workspace type and path restrictions.
2. **Given** a task with no workspace configuration, **When** the user runs `anvil task get <name>`, **Then** the output shows the default workspace type (project-only).

---

### User Story 4 - Default Project-Only Access (Priority: P1)

As a user, I want tasks to be restricted to the project directory by default (without any explicit workspace config) so that tasks cannot access files outside the project unless explicitly permitted.

**Why this priority**: A secure default protects users who haven't explicitly configured workspace settings. This is critical for security — tasks should not be able to access `~/.ssh`, `~/.aws`, or other sensitive home-directory files unless the user opts in.

**Independent Test**: Can be tested by creating a task with no workspace config and verifying it cannot access files outside the project directory.

**Acceptance Scenarios**:

1. **Given** a task with no workspace configuration, **When** the task attempts to access a file within the project directory, **Then** the access succeeds.
2. **Given** a task with no workspace configuration, **When** the task attempts to read `~/.ssh/id_rsa`, **Then** the access is blocked.
3. **Given** a task with no workspace configuration, **When** the task attempts to write to `/tmp/output.txt`, **Then** the access is blocked.

---

### Edge Cases

- What happens when `allowed_paths` and `blocked_paths` overlap (e.g., `allowed_paths: [./data/]` and `blocked_paths: [./data/secrets/]`)? Blocked paths take precedence.
- What happens when a task uses symbolic links to escape the workspace? Symlinks that resolve outside the allowed workspace are blocked.
- What happens when a relative path in `allowed_paths` resolves outside the project? It is treated relative to the project root; paths that escape the project root are rejected at parse time.
- What happens when a task with `workspace.type: temp` uses checkpoint? Checkpoint data is stored separately from the temp workspace and persists as normal.
- What happens when the temp workspace size limit is not specified? No size limit is enforced (system disk limits apply).
- What happens when workspace config has invalid paths? A parse warning is emitted (similar to frontmatter errors) and the task is preserved but not executed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a `workspace` block in task frontmatter YAML to configure file access restrictions.
- **FR-002**: System MUST support `workspace.type` with values: `restricted`, `temp`, and `project` (default).
- **FR-003**: System MUST support `workspace.allowed_paths` as a list of directory paths the task can read and write.
- **FR-004**: System MUST support `workspace.read_only` as a list of directory paths the task can read but not write.
- **FR-005**: System MUST support `workspace.blocked_paths` as a list of directory paths the task cannot access at all.
- **FR-006**: System MUST enforce that `blocked_paths` take precedence over `allowed_paths` and `read_only` when paths overlap.
- **FR-007**: System MUST resolve all workspace paths relative to the project root directory.
- **FR-008**: System MUST block symbolic links that resolve to paths outside the allowed workspace.
- **FR-009**: System MUST create and manage a temporary directory for tasks with `workspace.type: temp`.
- **FR-010**: System MUST clean up temporary workspace directories after task completion (success or failure).
- **FR-011**: System MUST support an optional `workspace.size` limit for temporary workspaces.
- **FR-012**: System MUST display workspace configuration in `anvil task get` output.
- **FR-013**: System MUST default to `workspace.type: project` which restricts access to the project directory only.
- **FR-014**: System MUST reject workspace path configurations that reference paths outside the project root (for `restricted` type).
- **FR-015**: System MUST log a warning when a task's file access is blocked by workspace restrictions.

### Key Entities

- **WorkspaceConfig**: Represents the workspace isolation settings for a task. Contains type (restricted/temp/project), allowed paths, read-only paths, blocked paths, and optional size limit.
- **Todo (extended)**: Existing task entity, extended with an optional WorkspaceConfig that defines its file access boundaries.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tasks with `workspace.type: restricted` can only access files within their `allowed_paths` and `read_only` paths — 100% of unauthorized access attempts are blocked.
- **SC-002**: Tasks with `workspace.type: temp` execute in an isolated directory that is fully cleaned up after completion in 100% of cases (success or failure).
- **SC-003**: Tasks with no explicit workspace config default to project-only access, blocking 100% of access attempts outside the project directory.
- **SC-004**: Users can view workspace configuration for any task via `anvil task get` within normal command response time.
- **SC-005**: Workspace configuration adds less than 1 second of overhead to task startup time.

## Assumptions

- The `jail` workspace type (OS-level isolation via Capsicum/namespaces) is deferred to a future iteration as it requires platform-specific kernel features and significantly more implementation complexity. The `restricted`, `temp`, and `project` types cover the majority of use cases.
- Workspace path enforcement is implemented at the application level (pre-execution validation and environment setup) rather than OS-level sandboxing. This provides practical protection against accidental access but is not a security boundary against deliberately malicious tasks.
- Temporary workspace size enforcement uses standard filesystem quota mechanisms or pre-execution checks rather than real-time monitoring.
- The existing task execution model (shell command or prompt execution) is preserved; workspace restrictions are applied as environment setup before execution.
