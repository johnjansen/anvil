---

description: "Task list template for feature implementation"
---

# Tasks: Add `anvil dispatch` Command

**Input**: Design documents from `/specs/225-dispatch-command/`
**Prerequisites**: plan.md (required), spec.md (required for user stories)

**Tests**: This feature is a CLI command addition. Integration tests via the existing test infrastructure are appropriate.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: No setup required for this CLI command feature

This is a straightforward CLI command addition using existing infrastructure. No project setup needed.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Modify existing code to expose task UUID for dispatch command

**⚠️ CRITICAL**: These changes must be complete before any user story implementation

- [ ] T001 Modify AddTodo to return task ID in cmd/anvil/main.go (call internal/project function that returns both path and UUID)
- [ ] T002 [P] Add helper function to get task ID from file path in cmd/anvil/main.go

---

## Phase 3: User Story 1 - Synchronous task dispatch with result (Priority: P1) 🎯 MVP

**Goal**: Users can run `anvil dispatch "task prompt"` and receive the output_summary when the task completes

**Independent Test**: Run `anvil dispatch "echo hello"` and verify it returns the output within reasonable time

### Implementation for User Story 1

- [ ] T003 [P] [US1] Add dispatchCmd function in cmd/anvil/main.go (line ~8770)
- [ ] T004 [P] [US1] Register dispatch command in cmd/anvil/main.go init()
- [ ] T005 [US1] Parse positional argument (task prompt) in dispatchCmd
- [ ] T006 [US1] Call AddTodo to create one-shot task and capture UUID
- [ ] T007 [US1] Implement wait loop (reuse logic from taskWaitCmd at main.go:6466-6501)
- [ ] T008 [US1] Read RunRecord via ReadCurrentRunRecord after completion
- [ ] T009 [US1] Print output_summary to stdout on success
- [ ] T010 [US1] Handle exit codes: 0=success, 1=failed, 2=timeout
- [ ] T011 [US1] Add error handling for daemon not running

**Checkpoint**: At this point, User Story 1 should be fully functional

---

## Phase 4: User Story 2 - Fire and forget async dispatch (Priority: P2)

**Goal**: Users can run `anvil dispatch --no-wait "task"` and get the UUID immediately

**Independent Test**: Run `anvil dispatch --no-wait "echo hello"` and verify it prints UUID within 500ms

### Implementation for User Story 2

- [ ] T012 [P] [US2] Add --no-wait flag parsing in dispatchCmd
- [ ] T013 [US2] Implement early return with UUID when --no-wait is set
- [ ] T014 [US2] Print task UUID to stdout (not stderr for scripting)

**Checkpoint**: User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Programmatic JSON output (Priority: P2)

**Goal**: Users can run `anvil dispatch --json "task"` and get full RunRecord as JSON

**Independent Test**: Run `anvil dispatch --json "echo hello"` and verify valid JSON output

### Implementation for User Story 3

- [ ] T015 [P] [US3] Add --json flag parsing in dispatchCmd
- [ ] T016 [US3] Marshal RunRecord to JSON when --json is set
- [ ] T017 [US3] Print JSON to stdout instead of output_summary

**Checkpoint**: User Stories 1, 2, AND 3 should all work independently

---

## Phase 6: User Story 4 - Configurable timeout (Priority: P3)

**Goal**: Users can run `anvil dispatch --timeout 5m "task"` to set max wait time

**Independent Test**: Run `anvil dispatch --timeout 1s "sleep 10"` and verify exit code 2

### Implementation for User Story 4

- [ ] T018 [P] [US4] Add --timeout flag parsing in dispatchCmd (default: 30m)
- [ ] T019 [US4] Pass timeout duration to wait loop
- [ ] T020 [US4] Exit with code 2 on timeout with message to stderr

---

## Phase 7: User Story 5 - Piped input support (Priority: P3)

**Goal**: Users can provide prompt via stdin or file: `echo "prompt" | anvil dispatch -` or `anvil dispatch -f file.md`

**Independent Test**: Run `echo "hello world" | anvil dispatch -` and verify it works

### Implementation for User Story 5

- [ ] T021 [P] [US5] Add -f flag support for file input (reuse from addCmd)
- [ ] T022 [US5] Add - argument support for stdin input
- [ ] T023 [US5] Read file/stdin content as task prompt when provided

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final improvements affecting multiple user stories

- [ ] T024 [P] Add --quiet flag to suppress progress output on stderr
- [ ] T025 [P] Ensure dispatch inherits common add flags (--skip-permissions, priority flags)
- [ ] T026 Add integration test for dispatch command
- [ ] T027 Update help text with examples for dispatch command
- [ ] T028 Verify all exit codes work correctly (0, 1, 2)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No setup required
- **Foundational (Phase 2)**: Must complete before user story implementation
- **User Stories (Phase 3-7)**: All depend on Foundational phase completion
  - Can proceed in parallel (P1 first for MVP)
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - No dependencies on other stories - **THIS IS THE MVP**
- **User Story 2 (P2)**: Can start after Foundational - Independent of US1
- **User Story 3 (P3)**: Can start after Foundational - Independent of US1/US2
- **User Story 4 (P3)**: Can start after Foundational - Independent
- **User Story 5 (P3)**: Can start after Foundational - Independent

### Within Each User Story

- Core implementation before adding flags
- Each flag adds independent functionality
- Story complete before moving to next priority

### Parallel Opportunities

- All Foundational tasks marked [P] can run in parallel
- All user story implementation tasks marked [P] can run in parallel
- Different user stories can be worked on in parallel

---

## Parallel Example: User Story Implementation

```bash
# These tasks can run in parallel:
Task: "Add dispatchCmd function in cmd/anvil/main.go"
Task: "Register dispatch command in cmd/anvil/main.go init()"

# These tasks within US1 can run in parallel:
Task: "Parse positional argument (task prompt) in dispatchCmd"
Task: "Add helper function to get task ID from file path"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational
2. Complete Phase 3: User Story 1
3. **STOP and VALIDATE**: Test dispatch command
4. Deploy/demo if ready

### Incremental Delivery

1. Complete Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Add User Story 4 → Test independently → Deploy/Demo
6. Add User Story 5 → Test independently → Deploy/Demo

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- MVP = User Story 1 only (synchronous dispatch with result)
