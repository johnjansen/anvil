# Feature Specification: Git Event Trigger for Tasks

**Feature Branch**: `365-git-event-trigger`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add git event trigger for tasks"
**Dependency**: Requires trigger framework from #363; sibling to #364 (file watcher trigger)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Trigger Task on Git Push (Priority: P1)

A user has a task that runs tests or deploys code after new commits are pushed. They configure the task with a `git` trigger specifying `push` events. The daemon periodically checks the current branch's HEAD ref; when it detects a new commit, the task executes automatically.

**Why this priority**: Detecting new pushes is the fundamental use case for git-based triggering. Without it, the feature provides no value.

**Independent Test**: Can be fully tested by creating a task with `trigger: { type: git, events: [push] }`, making a commit and pushing, and verifying the task executes after the next poll cycle.

**Acceptance Scenarios**:

1. **Given** a task configured with `trigger: { type: git, events: [push] }`, **When** a new commit is pushed to the watched branch, **Then** the task is triggered within one polling interval.
2. **Given** a task configured with `trigger: { type: git, events: [push] }`, **When** no new commits have been pushed since the last check, **Then** the task is NOT triggered.
3. **Given** a task configured with `trigger: { type: git, events: [push] }`, **When** multiple commits are pushed at once, **Then** the task is triggered exactly once.

---

### User Story 2 - Filter by Branch (Priority: P1)

A user wants to trigger a deploy task only when commits are pushed to `main`, not on feature branches. They configure the trigger with a `branch` filter.

**Why this priority**: Branch filtering is essential for real-world workflows where different branches have different automation needs. Without it, triggers fire for all branches, which is rarely desired.

**Independent Test**: Can be fully tested by configuring a task with `branch: main`, pushing to a different branch and verifying no trigger, then pushing to `main` and verifying the task runs.

**Acceptance Scenarios**:

1. **Given** a task with `branch: main`, **When** a commit is pushed to `main`, **Then** the task is triggered.
2. **Given** a task with `branch: main`, **When** a commit is pushed to `feature/foo`, **Then** the task is NOT triggered.
3. **Given** a task with no branch filter, **When** a commit is pushed to any branch, **Then** the task is triggered (default: watch all branches).

---

### User Story 3 - Filter by Path (Priority: P2)

A user wants to trigger a task only when files in a specific directory change. They configure the trigger with a `path` filter using glob patterns.

**Why this priority**: Path filtering enables targeted automation (e.g., only run frontend tests when frontend code changes) but is an optimization over the core push detection.

**Independent Test**: Can be fully tested by configuring a task with `path: src/frontend/**`, pushing a commit that changes only backend files (no trigger), then pushing a commit that changes frontend files (triggers).

**Acceptance Scenarios**:

1. **Given** a task with `path: src/frontend/**`, **When** a commit is pushed that modifies `src/frontend/app.js`, **Then** the task is triggered.
2. **Given** a task with `path: src/frontend/**`, **When** a commit is pushed that only modifies `src/backend/main.go`, **Then** the task is NOT triggered.
3. **Given** a task with multiple path patterns, **When** a commit matches any pattern, **Then** the task is triggered.

---

### User Story 4 - Configurable Polling Interval (Priority: P2)

A user wants to balance responsiveness with system load by configuring how frequently the daemon checks for git changes. They set a `poll_interval` in the trigger configuration.

**Why this priority**: A reasonable default polling interval works for most users, but configurability is needed for high-frequency or resource-constrained environments.

**Independent Test**: Can be fully tested by configuring a task with `poll_interval: 5s`, pushing a commit, and verifying the task triggers within approximately 5 seconds.

**Acceptance Scenarios**:

1. **Given** a task with `poll_interval: 5s`, **When** a commit is pushed, **Then** the task triggers within approximately 5 seconds.
2. **Given** a task with no poll_interval configured, **When** a commit is pushed, **Then** the task triggers within the default interval (30 seconds).

---

### User Story 5 - Pass Git Context to Task (Priority: P2)

When a task is triggered by a git event, the user wants access to information about what changed (commit SHA, branch name) so the task can act accordingly.

**Why this priority**: Enables smarter task behavior but the feature is still useful without it — the task can always query git itself.

**Independent Test**: Can be fully tested by triggering a task via git push and verifying that commit SHA and branch information are available to the task.

**Acceptance Scenarios**:

1. **Given** a task triggered by a git push event, **When** the task executes, **Then** the new commit SHA and branch name are available to the task.
2. **Given** a task triggered by a push with multiple commits, **When** the task executes, **Then** both the previous and current HEAD SHAs are available so the task can determine the full range of changes.

---

### Edge Cases

- What happens when the git repository is in a detached HEAD state? The trigger should still detect ref changes but branch filtering should not match any branch name.
- What happens when the daemon starts and has no record of the last-seen commit? It should record the current HEAD and not trigger on startup.
- What happens when a force push resets the branch to an earlier commit? The trigger should detect the ref change and fire.
- What happens when the `.git` directory is not accessible (e.g., permissions issue)? The daemon should log a warning and retry on the next poll cycle.
- What happens when the remote is unreachable during a poll? The daemon should log the error and retry on the next cycle without triggering the task.
- What happens when multiple tasks watch the same branch? Each task should independently evaluate its trigger conditions and fire independently.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a `git` trigger type in task frontmatter configuration.
- **FR-002**: System MUST detect new commits on watched branches by periodically polling git refs.
- **FR-003**: System MUST support `push` as a trigger event type.
- **FR-004**: System MUST support filtering by branch name, with a default of watching all branches.
- **FR-005**: System MUST support filtering by file path using glob patterns, checking which files changed between the last-seen and current commit.
- **FR-006**: System MUST support a configurable polling interval with a default of 30 seconds.
- **FR-007**: System MUST track the last-seen commit SHA per watched branch to avoid duplicate triggers.
- **FR-008**: System MUST pass git context information (commit SHA, branch name, previous SHA) to triggered tasks.
- **FR-009**: System MUST NOT trigger on daemon startup when no previous state exists; it should record the current HEAD as the baseline.
- **FR-010**: System MUST handle force pushes by detecting ref changes regardless of commit ancestry.
- **FR-011**: System MUST integrate with the trigger framework established by #363.
- **FR-012**: System MUST log polling activity, trigger events, and errors for observability.
- **FR-013**: System MUST handle unreachable remotes and git errors gracefully by logging and retrying on the next poll cycle.

### Key Entities

- **GitTrigger**: A trigger configuration specifying the event types (push), branch filter, path filter, and polling interval. Extends the trigger framework from #363.
- **GitRef**: A tracked reference point consisting of a branch name and commit SHA, used to detect changes between poll cycles.
- **GitPoller**: A runtime component managed by the daemon that periodically checks git refs and emits trigger events when changes are detected.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tasks configured with git triggers execute within one polling interval of a detectable git change.
- **SC-002**: Branch filtering correctly prevents task execution for non-matching branches 100% of the time.
- **SC-003**: Path filtering correctly prevents task execution when changed files do not match the configured patterns.
- **SC-004**: Each poll cycle completes in under 1 second for a typical repository.
- **SC-005**: Users can configure a complete git trigger in under 1 minute using the frontmatter syntax.
- **SC-006**: No duplicate task executions occur for the same commit across daemon restarts.

## Assumptions

- The trigger framework (#363) provides a plugin/registration mechanism for new trigger types that this feature will use.
- Polling is the primary detection mechanism; git hooks are out of scope for the initial implementation.
- The daemon runs in the same filesystem as the git repository it monitors.
- The `git` CLI is available on the system PATH and the daemon can execute git commands.
- The default polling interval of 30 seconds balances responsiveness with system load for most use cases.
- Last-seen commit state is persisted across daemon restarts (e.g., in `.anvil/` state files).
- Path filtering compares changed files between the last-seen commit and current HEAD.
