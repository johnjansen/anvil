# Tasks: Git Event Trigger for Tasks

**Input**: Design documents from `/specs/365-git-event-trigger/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Create the new git watcher file and state persistence directory structure

- [ ] T001 Create GitWatcher struct and constructor in internal/daemon/git.go with daemon back-reference, subscriptions map, mutex, and stop channel
- [ ] T002 Create GitRefState struct and read/write functions for persisting ref state as JSON in .anvil/git-state/<task-id>.json in internal/daemon/git.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Register the git subscription type in the daemon so user story tasks can trigger

- [ ] T003 Add gitWatcher *GitWatcher field to Daemon struct in internal/daemon/daemon.go
- [ ] T004 Add case "git" to startSubscriptions() in internal/daemon/daemon.go that initializes GitWatcher and calls StartSubscription for each git-subscribed task
- [ ] T005 Implement GitWatcher.StartSubscription(todo *project.Todo) method in internal/daemon/git.go that parses git_events, git_branch, git_poll_interval, git_path from todo.SubscriptionConfig and spawns a polling goroutine
- [ ] T006 Implement GitWatcher.Stop() method in internal/daemon/git.go to cancel all subscription goroutines and clean up

**Checkpoint**: Daemon recognizes `subscribe: git` in frontmatter and starts/stops polling goroutines

---

## Phase 3: User Story 1 - Trigger Task on Git Push (Priority: P1) MVP

**Goal**: Detect new commits by polling git refs and trigger the associated task

**Independent Test**: Create a task with `subscribe: git` and `git_events: [push]`, make a commit, and verify the task executes after the next poll cycle

### Implementation for User Story 1

- [ ] T007 [US1] Implement pollOnce() method in internal/daemon/git.go that runs `git rev-parse HEAD` via os/exec to get current ref for the watched branch
- [ ] T008 [US1] Implement ref comparison logic in pollOnce() in internal/daemon/git.go: load GitRefState from .anvil/git-state/<task-id>.json, compare stored SHA to current SHA, return whether a change was detected
- [ ] T009 [US1] Implement baseline initialization in pollOnce() in internal/daemon/git.go: if no state file exists, record current HEAD as baseline and do NOT trigger
- [ ] T010 [US1] Implement task dispatch in the polling goroutine in internal/daemon/git.go: when ref change detected, create workItem with git context env vars (ANVIL_GIT_EVENT, ANVIL_GIT_COMMIT, ANVIL_GIT_PREV_COMMIT, ANVIL_GIT_BRANCH, ANVIL_GIT_REPO) and send to daemon.workQueue
- [ ] T011 [US1] Persist updated GitRefState after successful dispatch in internal/daemon/git.go to prevent duplicate triggers across daemon restarts
- [ ] T012 [US1] Add error handling and logging to polling goroutine in internal/daemon/git.go: log poll start/completion, ref changes detected, git command errors, and retry on next cycle

**Checkpoint**: Tasks with `subscribe: git` trigger on new commits. Force pushes also trigger. Daemon restart does not re-trigger for already-seen commits.

---

## Phase 4: User Story 2 - Filter by Branch (Priority: P1)

**Goal**: Only trigger when commits appear on a specific branch matching the configured filter

**Independent Test**: Configure a task with `git_branch: main`, verify it triggers for main commits but not for other branches

### Implementation for User Story 2

- [ ] T013 [US2] Extend pollOnce() in internal/daemon/git.go to use `git for-each-ref refs/heads/` when no branch filter is set to check all branches for changes
- [ ] T014 [US2] Implement branch filtering in pollOnce() in internal/daemon/git.go: when git_branch is set, only run `git rev-parse refs/heads/<branch>` for the specific branch
- [ ] T015 [US2] Update GitRefState to track multiple branch refs in internal/daemon/git.go: store map of branch name to SHA, detect changes per-branch

**Checkpoint**: Branch filtering works correctly. Tasks with no branch filter watch all branches; tasks with git_branch only trigger for that branch.

---

## Phase 5: User Story 3 - Filter by Path (Priority: P2)

**Goal**: Only trigger when changed files match the configured path glob pattern

**Independent Test**: Configure a task with `git_path: src/frontend/**`, push a commit changing only backend files (no trigger), then push a commit changing frontend files (triggers)

### Implementation for User Story 3

- [ ] T016 [US3] Implement getChangedFiles() helper in internal/daemon/git.go that runs `git diff --name-only <old-sha> <new-sha>` via os/exec and returns list of changed file paths
- [ ] T017 [US3] Implement path matching in pollOnce() in internal/daemon/git.go: when git_path is set, call getChangedFiles() and check if any file matches the glob pattern using path/filepath.Match
- [ ] T018 [US3] Handle edge case in internal/daemon/git.go where old SHA is empty (baseline initialization) by skipping path filtering on first detection

**Checkpoint**: Path filtering correctly prevents triggers when no changed files match the pattern.

---

## Phase 6: User Story 4 - Configurable Polling Interval (Priority: P2)

**Goal**: Allow users to set how often the daemon polls for git changes

**Independent Test**: Set `git_poll_interval: 5s` and verify the task triggers within ~5 seconds of a push

### Implementation for User Story 4

- [ ] T019 [US4] Parse git_poll_interval from SubscriptionConfig in StartSubscription() in internal/daemon/git.go using time.ParseDuration with default of 30s
- [ ] T020 [US4] Use parsed interval as the ticker duration in the polling goroutine in internal/daemon/git.go

**Checkpoint**: Custom polling intervals work. Default 30s applies when not specified.

---

## Phase 7: User Story 5 - Pass Git Context to Task (Priority: P2)

**Goal**: Make commit SHA, branch name, and previous SHA available to the triggered task via environment variables

**Independent Test**: Trigger a task via git push and verify ANVIL_GIT_COMMIT, ANVIL_GIT_BRANCH, ANVIL_GIT_PREV_COMMIT, ANVIL_GIT_REPO are set correctly

### Implementation for User Story 5

- [ ] T021 [US5] Verify env var population in dispatch logic in internal/daemon/git.go: ensure ANVIL_GIT_EVENT, ANVIL_GIT_BRANCH, ANVIL_GIT_COMMIT, ANVIL_GIT_PREV_COMMIT, ANVIL_GIT_REPO are all set on the workItem
- [ ] T022 [US5] Handle edge case in internal/daemon/git.go where ANVIL_GIT_PREV_COMMIT is empty string on first detection after baseline establishment

**Checkpoint**: All git context environment variables are correctly populated and available to task scripts.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T023 Add daemon shutdown cleanup for GitWatcher in internal/daemon/daemon.go Stop() method
- [ ] T024 Ensure .anvil/git-state/ directory is created on first write in internal/daemon/git.go
- [ ] T025 Add gitWatcher status to daemon health/status reporting in internal/daemon/daemon.go if applicable

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion - BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 - core push detection (MVP)
- **US2 (Phase 4)**: Depends on Phase 2 - can run in parallel with US1 but builds on same pollOnce()
- **US3 (Phase 5)**: Depends on US1 (needs ref comparison working to get old/new SHAs for diff)
- **US4 (Phase 6)**: Depends on Phase 2 only - independent of other stories
- **US5 (Phase 7)**: Depends on US1 (env vars set during dispatch, which US1 implements)
- **Polish (Phase 8)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (P1)**: Independent after foundational - MVP
- **US2 (P1)**: Independent after foundational, but shares pollOnce() with US1. Recommend sequential after US1.
- **US3 (P2)**: Requires US1 complete (needs SHA comparison for git diff)
- **US4 (P2)**: Independent after foundational
- **US5 (P2)**: Requires US1 complete (extends dispatch logic)

### Parallel Opportunities

- T001 and T002 can run in parallel (different concerns in same file, but no conflicts)
- T003 and T004 touch daemon.go but different sections
- T019/T020 (US4) can run in parallel with T016/T017/T018 (US3) since they touch different parts of git.go
- T023, T024, T025 (Polish) can all run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T006)
3. Complete Phase 3: User Story 1 (T007-T012)
4. **STOP and VALIDATE**: Test with a simple `subscribe: git` task, make commits, verify trigger
5. This delivers: basic git push detection with commit tracking and daemon restart safety

### Incremental Delivery

1. Setup + Foundational → Git watcher registered in daemon
2. Add US1 → Push detection works → MVP!
3. Add US2 → Branch filtering works → Production-ready for most use cases
4. Add US3 → Path filtering works → Advanced use cases enabled
5. Add US4 → Custom polling intervals → Performance tuning enabled
6. Add US5 → Git context env vars → Full feature parity with spec

---

## Notes

- All new code goes in a single new file: `internal/daemon/git.go`
- Minimal changes to `internal/daemon/daemon.go` (add field, registration case, shutdown)
- Follows existing FSWatcher pattern closely for consistency
- No new Go dependencies required - uses `os/exec` for git CLI and stdlib for everything else
- State persistence uses same JSON pattern as RunRecord system
