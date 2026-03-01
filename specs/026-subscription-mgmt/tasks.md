# Tasks: Subscription Management CLI Commands

**Feature**: Subscription Management CLI Commands
**Feature Branch**: 026-subscription-mgmt
**Date**: 2026-03-01

## Summary

Add CLI commands to manage task subscriptions: list all subscriptions, pause/resume individual subscriptions, and view subscription details.

- **Total Tasks**: 12
- **User Stories**: 4
- **Parallel Opportunities**: 2

## User Story Dependencies

```
User Story 1 (List)    ──┬── Independent
User Story 2 (Pause)  ──┤
User Story 3 (Resume) ──┼── Depends on US2 (state management shared)
User Story 4 (Info)   ──┘
```

## Implementation Strategy

**MVP Scope**: User Story 1 (List subscriptions) - Core functionality
**Incremental Delivery**: Each user story is independently testable

## Phase 1: Setup

- [ ] T001 Create subscription state type in internal/models/subscription.go

## Phase 2: Foundational

- [ ] T002 Create subscription state persistence in internal/subscription/state.go
- [ ] T003 Create daemon subscription handlers in internal/daemon/subscription.go

## Phase 3: User Story 1 - List Active Subscriptions (P1)

**Goal**: Users can view all active subscriptions with their status

**Independent Test**: Run `anvil subscription ls` and verify it displays all subscriptions

- [ ] T004 [P] [US1] Implement subscription list command in cmd/anvil/subscription.go
- [ ] T005 [US1] Add daemon handler for listing subscriptions in internal/daemon/subscription.go
- [ ] T006 [US1] Implement subscription state loading from .anvil/subscriptions/state/ in internal/subscription/state.go

## Phase 4: User Story 2 - Pause a Subscription (P1)

**Goal**: Users can pause a subscription to stop it from triggering tasks

**Independent Test**: Pause a subscription and verify it no longer triggers tasks

- [ ] T007 [P] [US2] Implement subscription pause command in cmd/anvil/subscription.go
- [ ] T008 [US2] Add daemon handler for pausing subscriptions in internal/daemon/subscription.go

## Phase 5: User Story 3 - Resume a Paused Subscription (P1)

**Goal**: Users can resume a paused subscription to re-enable triggers

**Independent Test**: Resume a paused subscription and verify it starts triggering again

- [ ] T009 [P] [US3] Implement subscription resume command in cmd/anvil/subscription.go
- [ ] T010 [US3] Add daemon handler for resuming subscriptions in internal/daemon/subscription.go

## Phase 6: User Story 4 - View Subscription Details (P2)

**Goal**: Users can view detailed information about a specific subscription

**Independent Test**: Run `anvil subscription info <id>` and verify detailed output

- [ ] T011 [P] [US4] Implement subscription info command in cmd/anvil/subscription.go
- [ ] T012 [US4] Add daemon handler for subscription details in internal/daemon/subscription.go

## Phase 7: Polish & Cross-Cutting

- [ ] T013 Add JSON output support (--json flag) to all subscription commands
- [ ] T014 Write unit tests for subscription state management in tests/subscription_test.go
- [ ] T015 Register subscription command in cmd/anvil/main.go

## Parallel Execution Examples

**Story 1 (List)**:
```bash
# Can be tested immediately after T003
go build -o anvil ./cmd/anvil && ./anvil subscription ls
```

**Stories 2 & 3 (Pause/Resume)**:
```bash
# Both use same state management, can be tested together after T008
go build -o anvil ./cmd/anvil && ./anvil subscription pause test-task && ./anvil subscription resume test-task
```

## File Paths

| Task | File Path |
|------|-----------|
| T001 | internal/models/subscription.go |
| T002 | internal/subscription/state.go |
| T003 | internal/daemon/subscription.go |
| T004 | cmd/anvil/subscription.go |
| T005 | internal/daemon/subscription.go |
| T006 | internal/subscription/state.go |
| T007 | cmd/anvil/subscription.go |
| T008 | internal/daemon/subscription.go |
| T009 | cmd/anvil/subscription.go |
| T010 | internal/daemon/subscription.go |
| T011 | cmd/anvil/subscription.go |
| T012 | internal/daemon/subscription.go |
| T013 | cmd/anvil/subscription.go |
| T014 | tests/subscription_test.go |
| T015 | cmd/anvil/main.go |
