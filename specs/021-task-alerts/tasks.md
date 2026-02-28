# Tasks: Task Alerting Rules

**Feature**: 021-task-alerts
**Generated**: 2026-03-01
**Spec**: [spec.md](spec.md)
**Plan**: [plan.md](plan.md)

## Summary

- **Total Tasks**: 18
- **User Stories**: 4 (US1-P1: Alert Conditions, US2-P1: Alert Actions, US3-P2: View/Acknowledge, US4-P3: Multiple Rules)
- **MVP Scope**: US1 + US2 (P1 stories) = Tasks T001-T010
- **Parallel Opportunities**: T004/T005 can run in parallel (different structs)

## Implementation Strategy

**MVP First**: Complete Phase 3 (US1 - Alert Conditions) and Phase 4 (US2 - Alert Actions) for a minimal viable product. This enables basic alert triggering with actions.

**Incremental Delivery**:
- Phase 3-4 (MVP): Basic alerting with conditions and actions
- Phase 5: Add CLI visibility (list, ack, history)
- Phase 6: Support multiple rules per task
- Phase 7: Polish and edge cases

## Dependencies

```
Phase 2 (Foundational)
  └── T003: Add AlertConfig to Todo → Blocks all User Stories

Phase 3 (US1 - Alert Conditions)
  └── Depends on T003

Phase 4 (US2 - Alert Actions)
  └── Depends on T003, T006

Phase 5 (US3 - View/Acknowledge)
  └── Depends on T003

Phase 6 (US4 - Multiple Rules)
  └── Depends on Phase 3-4

Phase 7 (Polish)
  └── Depends on all phases complete
```

## Phase 1: Setup

- [X] T001 Create alerts package in internal/daemon/alerts.go

## Phase 2: Foundational

- [X] T002 Add AlertGlobalConfig to internal/config/config.go
- [X] T003 [P] Add AlertConfig struct and Alerts field to Todo in internal/project/project.go

## Phase 3: User Story 1 - Configure Alert Conditions (P1)

**Goal**: Enable users to define alert conditions that trigger alerts
**Independent Test**: Create task with alert conditions, run task, verify alerts fire when conditions are met

- [X] T004 [P] [US1] Add AlertCondition, AlertRule, AlertConfig structs in internal/project/project.go
- [X] T005 [P] [US1] Add AlertRecord struct in internal/daemon/alerts.go
- [X] T006 [US1] Implement alert condition evaluation (cost, duration, output) in internal/daemon/alerts.go
- [X] T007 [US1] Integrate alert evaluation in run completion handler in internal/daemon/daemon.go
- [X] T008 [US1] Implement alert storage in .anvil/alerts/<task-id>/alerts.json

## Phase 4: User Story 2 - Configure Alert Actions (P1)

**Goal**: Enable users to specify what happens when an alert triggers
**Independent Test**: Configure alert with webhook, trigger alert, verify POST request sent

- [X] T009 [P] [US2] Add AlertAction struct in internal/daemon/alerts.go
- [X] T010 [US2] Implement webhook delivery with retry logic in internal/daemon/alerts.go
- [X] T011 [US2] Implement notify action (placeholder for future notification system)

## Phase 5: User Story 2 - View and Acknowledge Alerts (P2)

**Goal**: Enable users to see active alerts and acknowledge them
**Independent Test**: Run `anvil alerts`, verify output; run `anvil alerts ack`, verify alert marked acknowledged

- [X] T012 [P] [US3] Add alerts command with list subcommand in cmd/anvil/main.go
- [X] T013 [US3] Add alerts ack subcommand in cmd/anvil/main.go
- [X] T014 [US3] Add alerts history subcommand in cmd/anvil/main.go

## Phase 6: User Story 3 - Multiple Alert Rules (P3)

**Goal**: Enable multiple alert rules per task
**Independent Test**: Define multiple alerts on task, verify each fires independently

- [X] T015 [P] [US4] Update alert evaluation to handle multiple rules per task
- [X] T016 [US4] Add severity levels to alert display

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T017 Add tests for alert evaluation logic in internal/daemon/alerts_test.go
- [X] T018 [P] Verify backward compatibility (tasks without alerts work unchanged)

## Parallel Execution Examples

**Example 1**: T004 + T005 can run in parallel (different structs, no dependencies)

**Example 2**: T012 + T015 + T018 can run in parallel (different components, no shared logic)

## Independent Test Criteria

| Story | Test Criteria |
|-------|---------------|
| US1 | Task with cost > $10 alert fires when cost is $15; Task with duration > 30m alert fires when duration is 45m; Task with output pattern "ERROR:" fires when output contains "ERROR:" |
| US2 | Webhook receives POST with alert payload; Retry happens on failure up to configured count |
| US3 | `anvil alerts` shows all active alerts; `anvil alerts ack <id>` marks as acknowledged; `anvil alerts history` shows past alerts |
| US4 | Multiple rules fire independently; Each rule carries its own severity |
