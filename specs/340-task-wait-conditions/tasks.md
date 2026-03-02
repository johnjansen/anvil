# Implementation Tasks: Task Wait Conditions for Multi-Criteria Triggering

## Phase 1: Setup

- [ ] T001 Create project structure per implementation plan

## Phase 2: Foundational

- [ ] T002 [P] Extend task configuration to support trigger conditions in internal/project/task_config.go
- [ ] T003 [P] Define Condition struct in internal/project/trigger.go
- [ ] T004 [P] Define TaskTrigger struct in internal/project/trigger.go
- [ ] T005 [P] Define PollingConfig struct in internal/project/trigger.go
- [ ] T006 Implement condition evaluation interface in internal/project/condition_eval.go

## Phase 3: User Story 1 - Complex Task Triggering (P1)

**Goal**: Enable tasks to be triggered based on multiple conditions with AND/OR logic

**Independent Test**: Can be fully tested by creating a task with multiple trigger conditions and verifying it executes when all conditions are met.

### Implementation Tasks

- [ ] T007 [P] [US1] Implement file existence condition evaluation in internal/project/condition_eval.go
- [ ] T008 [P] [US1] Implement environment variable condition evaluation in internal/project/condition_eval.go
- [ ] T009 [US1] Implement AND logic for condition evaluation in internal/project/condition_eval.go
- [ ] T010 [US1] Implement OR logic for condition evaluation in internal/project/condition_eval.go
- [ ] T011 [P] [US1] Create unit tests for condition evaluation logic in tests/unit/condition_eval_test.go
- [ ] T012 [US1] Update task scheduler to check trigger conditions in internal/daemon/task_scheduler.go
- [ ] T013 [US1] Create integration tests for complex triggering in tests/integration/multi_criteria_trigger_test.go

## Phase 4: User Story 2 - Polling-Based Triggers (P2)

**Goal**: Enable tasks to be triggered based on polling conditions

**Independent Test**: Can be fully tested by creating a task with polling conditions and verifying it executes when the polled condition becomes true.

### Implementation Tasks

- [ ] T014 [P] [US2] Implement polling manager in internal/daemon/polling_manager.go
- [ ] T015 [P] [US2] Implement file polling condition in internal/project/condition_eval.go
- [ ] T016 [US2] Implement timeout handling for polling conditions
- [ ] T017 [US2] Implement run-once logic for polling conditions
- [ ] T018 [P] [US2] Create unit tests for polling manager in tests/unit/polling_manager_test.go
- [ ] T019 [US2] Integrate polling manager with task scheduler
- [ ] T020 [US2] Create integration tests for polling triggers in tests/integration/multi_criteria_trigger_test.go

## Phase 5: User Story 3 - Manual Trigger Evaluation (P3)

**Goal**: Enable manual evaluation of trigger conditions for testing and debugging

**Independent Test**: Can be fully tested by running the manual trigger check command and verifying it evaluates conditions correctly.

### Implementation Tasks

- [ ] T021 [P] [US3] Create CLI command for trigger evaluation in cmd/anvil/task_trigger_check.go
- [ ] T022 [US3] Implement trigger evaluation reporting
- [ ] T023 [P] [US3] Create unit tests for CLI command in tests/unit/task_trigger_check_test.go
- [ ] T024 [US3] Add documentation for manual trigger evaluation

## Final Phase: Polish & Cross-Cutting Concerns

- [ ] T025 Add logging for trigger evaluations
- [ ] T026 Ensure backward compatibility with existing task configurations
- [ ] T027 Update documentation with new trigger configuration options
- [ ] T028 Run full integration test suite to ensure no regressions
- [ ] T029 Performance testing of condition evaluation
- [ ] T030 Final code review and cleanup

## Dependencies

User Story 1 (P1) → User Story 2 (P2) → User Story 3 (P3)

## Parallel Execution Opportunities

Within each user story, tasks marked with [P] can be executed in parallel:
- File existence condition evaluation (T007)
- Environment variable condition evaluation (T008)
- Unit tests for condition evaluation logic (T011)
- File polling condition implementation (T15)
- Unit tests for polling manager (T018)
- CLI command for trigger evaluation (T021)
- Unit tests for CLI command (T023)

## Implementation Strategy

1. **MVP Focus**: Implement User Story 1 first to establish the core functionality
2. **Incremental Delivery**: Add polling and manual evaluation as enhancements
3. **Test-First Approach**: Write unit tests before implementation where possible
4. **Backward Compatibility**: Ensure existing tasks continue to work unchanged