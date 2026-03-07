# Tasks: File Watcher Trigger for Tasks

**Input**: Design documents from `/specs/364-file-watch-trigger/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new project setup needed — this feature extends existing code. Skip to foundational phase.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extend data model and create debounce infrastructure that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T001 Add `FsEvents`, `FsDebounce`, `FsGlob`, `FsRecursive` fields to `SubscriptionConfig` struct in `internal/project/project.go`
- [ ] T002 Add `FileEvent` struct (Path, Event, Timestamp) to `internal/daemon/fs.go`
- [ ] T003 Add `debouncer` struct with timer-reset logic, event accumulation, and flush callback in `internal/daemon/fs.go`
- [ ] T004 Add helper function to map user-facing event names (`create`, `modify`, `delete`, `rename`) to `fsnotify.Op` constants in `internal/daemon/fs.go`
- [ ] T005 [P] Add helper function to match file paths against glob patterns using `filepath.Match` in `internal/daemon/fs.go`

**Checkpoint**: Foundation ready — data model extended, debounce and filtering utilities available

---

## Phase 3: User Story 1 - Watch a Directory for New Data Files (Priority: P1) MVP

**Goal**: Tasks with `file_watch` (fs) triggers fire when new files matching a glob pattern appear in a watched directory

**Independent Test**: Create a task with `subscription: { type: fs, fs_path: ./data, fs_glob: "*.json", fs_events: [create] }`, create a matching file, verify task executes. Create a non-matching file, verify task does NOT execute.

### Implementation for User Story 1

- [ ] T006 [US1] Update `processEvents` in `internal/daemon/fs.go` to filter events by `fs_events` list (default: all events if empty)
- [ ] T007 [US1] Update `processEvents` in `internal/daemon/fs.go` to filter events by `fs_glob` pattern match (default: `*` if empty)
- [ ] T008 [US1] Update `StartSubscription` in `internal/daemon/fs.go` to read and validate `FsGlob` and `FsEvents` from `SubscriptionConfig`
- [ ] T009 [US1] Add validation for `fs_events` values (must be `create`, `modify`, `delete`, or `rename`) in `StartSubscription` in `internal/daemon/fs.go`

**Checkpoint**: Tasks trigger on matching file creation events with glob filtering

---

## Phase 4: User Story 2 - Debounce Rapid File Changes (Priority: P1) MVP

**Goal**: Rapid file changes are collapsed into a single task execution after a configurable debounce period

**Independent Test**: Create a task with `fs_debounce: 2s`, rapidly create multiple files, verify the task runs exactly once after the debounce window

### Implementation for User Story 2

- [ ] T010 [US2] Integrate `debouncer` into `watcher` struct — create debouncer on `StartSubscription` using `FsDebounce` duration (default 1s) in `internal/daemon/fs.go`
- [ ] T011 [US2] Update `processEvents` to route matching events through debouncer instead of calling `handleEvent` directly in `internal/daemon/fs.go`
- [ ] T012 [US2] Implement debouncer flush callback that calls `handleEvent` with the accumulated batch of `FileEvent` objects in `internal/daemon/fs.go`
- [ ] T013 [US2] Add debounce duration validation (minimum 100ms) in `StartSubscription` in `internal/daemon/fs.go`
- [ ] T014 [US2] Clean up debouncer timer on `StopSubscription` and `StopAll` in `internal/daemon/fs.go`

**Checkpoint**: Debounce collapses rapid file changes into single task execution

---

## Phase 5: User Story 3 - React to File Modifications and Deletions (Priority: P2)

**Goal**: Tasks can be configured to trigger on modify and delete events, not just create

**Independent Test**: Configure task with `fs_events: [modify]`, modify a file, verify task triggers. Configure with `fs_events: [create]`, modify a file, verify task does NOT trigger.

### Implementation for User Story 3

- [ ] T015 [US3] Ensure event type mapping handles `fsnotify.Remove` for `delete` events and `fsnotify.Write` for `modify` events in `internal/daemon/fs.go`
- [ ] T016 [US3] Add `rename` event support mapping to `fsnotify.Rename` in `internal/daemon/fs.go`

**Checkpoint**: All event types (create, modify, delete, rename) filter correctly

---

## Phase 6: User Story 4 - Pass Changed File Information to Task (Priority: P2)

**Goal**: Triggered tasks receive file path, event type, and batch information via environment variables

**Independent Test**: Trigger a task via file change, verify `ANVIL_FS_EVENT`, `ANVIL_FS_PATH`, `ANVIL_FS_EVENT_COUNT`, and `ANVIL_FS_EVENTS` environment variables are set correctly

### Implementation for User Story 4

- [ ] T017 [US4] Update `handleEvent` in `internal/daemon/fs.go` to accept `[]FileEvent` batch instead of single `fsnotify.Event`
- [ ] T018 [US4] Set `ANVIL_FS_EVENT_COUNT` environment variable to the batch size in `handleEvent` in `internal/daemon/fs.go`
- [ ] T019 [US4] Set `ANVIL_FS_EVENTS` environment variable to JSON-serialized array of `{"path":"...","event":"..."}` objects in `handleEvent` in `internal/daemon/fs.go`
- [ ] T020 [US4] Maintain backward compatibility: set `ANVIL_FS_EVENT` and `ANVIL_FS_PATH` to the last event in the batch in `handleEvent` in `internal/daemon/fs.go`

**Checkpoint**: Task scripts can access all file change details from environment variables

---

## Phase 7: User Story 5 - Watcher Lifecycle Management (Priority: P2)

**Goal**: File watchers start/stop cleanly with daemon lifecycle and task additions/removals

**Independent Test**: Start daemon with fs task, verify watcher active. Stop daemon, verify clean shutdown. Add new fs task at runtime, verify watcher starts.

### Implementation for User Story 5

- [ ] T021 [US5] Add logging for watcher start, stop, event detection, and error conditions in `internal/daemon/fs.go`
- [ ] T022 [US5] Handle non-existent `fs_path` by logging warning and polling for directory creation (30s interval) in `StartSubscription` in `internal/daemon/fs.go`
- [ ] T023 [US5] Add recursive directory watching support: walk subdirectories and add watchers when `fs_recursive: true` in `StartSubscription` in `internal/daemon/fs.go`
- [ ] T024 [US5] Watch for new subdirectory creation events and auto-add watchers when `fs_recursive: true` in `processEvents` in `internal/daemon/fs.go`

**Checkpoint**: Watcher lifecycle is fully managed with no resource leaks

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T025 Validate frontmatter parsing handles all new `SubscriptionConfig` fields correctly in `internal/project/project.go`
- [ ] T026 Run `go vet ./...` and `go build ./...` to verify no compilation errors
- [ ] T027 Validate quickstart.md scenarios work end-to-end

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — extends existing code
- **US1 (Phase 3)**: Depends on Phase 2 (T001-T005)
- **US2 (Phase 4)**: Depends on Phase 2 (T003 debouncer) and Phase 3 (T006-T007 event filtering)
- **US3 (Phase 5)**: Depends on Phase 2 (T004 event mapping) — can run in parallel with US2
- **US4 (Phase 6)**: Depends on Phase 4 (T012 batch handling)
- **US5 (Phase 7)**: Depends on Phase 2 — can run in parallel with US1-US4
- **Polish (Phase 8)**: Depends on all user stories

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 — no story dependencies
- **US2 (P1)**: Depends on US1 (needs event filtering before debounce makes sense)
- **US3 (P2)**: Can start after Phase 2 — independent of US1/US2
- **US4 (P2)**: Depends on US2 (needs batch handling from debouncer)
- **US5 (P2)**: Can start after Phase 2 — independent of other stories

### Parallel Opportunities

- T004 and T005 can run in parallel (different helper functions)
- US3 and US5 can run in parallel with US1/US2 (independent concerns)
- T017-T020 (US4) can proceed once T012 (US2) is done

---

## Parallel Example: Foundational Phase

```bash
# These can run in parallel (different concerns):
Task: "T004 Add event name mapping helper in internal/daemon/fs.go"
Task: "T005 Add glob pattern matching helper in internal/daemon/fs.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 2: Foundational (data model + utilities)
2. Complete Phase 3: US1 — glob filtering and event type filtering
3. Complete Phase 4: US2 — debounce
4. **STOP and VALIDATE**: Test with a real task that watches for file changes with debounce
5. This MVP delivers the core file-watching functionality

### Incremental Delivery

1. Foundation → US1 + US2 → Core file watching with debounce (MVP)
2. Add US3 → Full event type support
3. Add US4 → Batch file info in environment variables
4. Add US5 → Robust lifecycle management
5. Polish → Validation and cleanup

---

## Notes

- All implementation is in existing files — no new packages needed
- `internal/daemon/fs.go` is the primary file (most tasks touch it)
- `internal/project/project.go` only needs T001 (struct extension) and T025 (validation)
- Backward compatibility with existing `fs` subscription type must be maintained
- The debouncer is the most complex new component — get it right in Phase 4
