# Tasks: Task Subscriptions for External Event Triggers

**Input**: Design documents from `/specs/016-task-subscriptions/`
**Prerequisites**: PLAN-334.md (required), SPEC-334.md (required for user stories)

**Tests**: Unit tests and integration tests required for each handler

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Data Model & Core Infrastructure

**Purpose**: Add subscription support to the data model and create the subscription package foundation

- [ ] T001 [P] Add `Subscription` struct to `internal/project/project.go` with fields for Type, Path, Method, Secret (webhook), Queue, URL (amqp), Events (fs)
- [ ] T002 [P] Add `Subscription` field to `Todo` struct in `internal/project/project.go`
- [ ] T003 Create `internal/subscription/state.go` for subscription state persistence in `.anvil/subscriptions/state.json`

---

## Phase 2: Webhook Subscription (User Story 1)

**Purpose**: Implement HTTP webhook trigger support

- [ ] T004 Create `internal/subscription/webhook.go` with HTTP server implementation
- [ ] T005 Implement path routing from task config to webhook handler
- [ ] T006 Implement secret validation for webhooks
- [ ] T007 Pass webhook payload via `ANVIL_WEBHOOK_PAYLOAD` environment variable
- [ ] T008 Add unit tests for webhook handler

---

## Phase 3: AMQP Subscription (User Story 2)

**Purpose**: Implement message queue trigger support

- [ ] T009 Create `internal/subscription/amqp.go` with AMQP client implementation
- [ ] T010 Implement queue subscription from task config
- [ ] T011 Handle reconnection on AMQP disconnect
- [ ] T012 Pass message body via environment variable
- [ ] T013 Add unit tests for AMQP handler

---

## Phase 4: File System Subscription (User Story 3)

**Purpose**: Implement file system event trigger support

- [ ] T014 Create `internal/subscription/fs.go` with fsnotify watcher
- [ ] T015 Implement glob pattern matching for file paths
- [ ] T016 Implement event filtering (create, modify)
- [ ] T017 Debounce rapid file events
- [ ] T018 Pass file path via environment variable
- [ ] T019 Add unit tests for FS handler

---

## Phase 5: Subscription Manager (User Story 4)

**Purpose**: Coordinate all subscription types and integrate with daemon

- [ ] T020 Create `internal/subscription/manager.go` - SubscriptionManager to coordinate all handlers
- [ ] T021 Implement Start/Stop methods for all subscription types
- [ ] T022 Implement pause/resume functionality
- [ ] T023 Integrate with daemon in `internal/daemon/daemon.go` - start subscription manager alongside scheduler

---

## Phase 6: CLI Commands (User Story 4)

**Purpose**: Add subscription management commands

- [ ] T024 Add `anvil subscription ls` command in `cmd/anvil/main.go`
- [ ] T025 Add `anvil subscription pause <task>` command
- [ ] T026 Add `anvil subscription resume <task>` command
- [ ] T027 Test all CLI commands manually

---

## Phase 7: State Persistence & Integration

**Purpose**: Ensure subscriptions persist across daemon restarts

- [ ] T028 Persist paused state to `.anvil/subscriptions/state.json`
- [ ] T029 Load and restore subscription state on daemon startup
- [ ] T030 End-to-end test: trigger webhook → task queued → task executes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Data Model)**: No dependencies — foundation for all subsequent phases
- **Phase 2 (Webhook)**: Depends on Phase 1 — needs Subscription struct
- **Phase 3 (AMQP)**: Depends on Phase 1 — needs Subscription struct
- **Phase 4 (FS)**: Depends on Phase 1 — needs Subscription struct
- **Phase 5 (Manager)**: Depends on Phases 2, 3, 4 — coordinates all handlers
- **Phase 6 (CLI)**: Depends on Phase 1, 5 — needs manager and state
- **Phase 7 (Persistence)**: Depends on Phase 5 — state management

### Within Each Phase

- T001, T002, T003 can run in parallel
- T004-T008 (webhook) are sequential
- T009-T013 (AMQP) are sequential
- T014-T019 (FS) are sequential

### Parallel Opportunities

- Phases 2, 3, 4 can run in parallel (different subscription types, no dependencies between them)
- T024, T025, T026 can run in parallel (CLI commands, same file)

---

## Notes

- [P] tasks = different subscription types, no dependencies between them
- Each subscription type (webhook, AMQP, FS) should be independently testable
- Commit after each phase completion
- Consider adding integration tests that run against real servers for webhook/AMQP
