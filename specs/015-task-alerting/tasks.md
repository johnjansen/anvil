# Tasks: Task Alerting Rules

**Feature Branch**: `015-task-alerting`

## Implementation Tasks

### Phase 1: Core Data Structures

- [ ] **T001** Add AlertRule struct to `internal/project/project.go` with fields: Name, Condition, Message, Severity, Action
- [ ] **T002** Add Alerts field to Todo struct in `internal/project/project.go`
- [ ] **T003** Create `internal/alerts/alerts.go` with Alert and AlertStatus types
- [ ] **T004** Create `internal/alerts/storage.go` with AlertStore for JSON file persistence

### Phase 2: Alert Evaluation

- [ ] **T005** Implement condition parser in `internal/alerts/alerts.go` - cost, duration, output regex
- [ ] **T006** Implement `EvaluateAlerts()` function that takes Todo and RunResult
- [ ] **T007** Add unit tests for condition parsing

### Phase 3: CLI Commands

- [ ] **T008** Add `anvil alerts` command to list active alerts
- [ ] **T009** Add `anvil alerts ack <alert-id>` command to acknowledge alerts
- [ ] **T010** Add `anvil alerts history` command to show past alerts
- [ ] **T011** Add `anvil alerts get <alert-id>` command to show alert details

### Phase 4: Daemon Integration

- [ ] **T012** Modify daemon to call alert evaluation after task completion
- [ ] **T013** Store triggered alerts to alert storage
- [ ] **T014** Implement webhook delivery in alert actions with retry logic

### Phase 5: Testing

- [ ] **T015** Integration test: Create task with alert rule, run task, verify alert fires
- [ ] **T016** Integration test: Test alert acknowledgment workflow
- [ ] **T017** Integration test: Test webhook delivery

## Task Dependencies

```
T001 ─┬─ T002 ── T005 ── T006 ── T012 ─┬─ T015
      │                               │
T003 ─┤                               │
      │                               │
T004 ─┘                               │
                                        │
T008 ──┬─ T009 ──┬─ T010 ── T011 ─────┘
       │         │
       └─────────┘
                │
              T013
                │
              T014
                │
              T016, T017
```

## Notes

- T001 and T003 can be done in parallel
- T005 depends on T001/T003 (need types defined)
- T006 depends on T005 (need condition parser)
- CLI tasks (T008-T011) can be done in parallel after T003/T004
- T015 requires T012-T014 to be complete
