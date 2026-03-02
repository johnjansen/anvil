# Implementation Tasks: Task Preconditions for Conditional Execution

**Feature**: 035-task-preconditions | **Branch**: 035-task-preconditions  
**Generated**: 2026-03-02 | **Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

## Implementation Strategy

This feature will be implemented in phases corresponding to user stories, with each story being independently testable. The MVP will focus on User Story 1 (time-based conditions) to establish the core functionality before expanding to more complex features.

## Dependencies

User stories can be implemented independently:
- US1 (Time-based conditions): Foundational for all other precondition types
- US2 (Environment conditions): Independent, can be implemented in parallel
- US3 (Complex expressions): Independent, can be implemented in parallel
- US4 (Combined logic): Requires both precondition and pre_check, can be implemented last

## Phase 1: Setup

Initialize project structure and dependencies for the feature.

- [ ] T001 Create preconditions package directory in internal/project/preconditions
- [ ] T002 Add precondition fields to task frontmatter parsing in internal/project/project.go
- [ ] T003 Create template variable context provider

## Phase 2: Foundational

Core infrastructure needed for all precondition types.

- [ ] T004 [P] Implement day_of_week precondition evaluator in internal/project/preconditions/day_of_week.go
- [ ] T005 [P] Implement time_range precondition evaluator in internal/project/preconditions/time_range.go
- [ ] T006 [P] Implement env_set precondition evaluator in internal/project/preconditions/env_set.go
- [ ] T007 [P] Implement expression precondition evaluator in internal/project/preconditions/expr.go
- [ ] T008 Create precondition evaluation orchestrator in internal/project/preconditions/evaluator.go
- [ ] T009 Add template variables provider in internal/project/preconditions/variables.go

## Phase 3: User Story 1 - Time-Based Conditions (P1)

Implement time-based preconditions for scheduling control.

Independent Test: Can be fully tested by creating a task with time-based preconditions and verifying it only executes during the specified time windows.

- [ ] T010 [P] [US1] Add day_of_week validation to precondition parser in internal/project/project.go
- [ ] T011 [P] [US1] Add time_range validation to precondition parser in internal/project/project.go
- [ ] T012 [US1] Integrate precondition evaluation into task dispatch flow in internal/runner/runner.go
- [ ] T013 [P] [US1] Create unit tests for day_of_week evaluation in internal/project/preconditions/day_of_week_test.go
- [ ] T014 [P] [US1] Create unit tests for time_range evaluation in internal/project/preconditions/time_range_test.go
- [ ] T015 [US1] Create integration test for time-based preconditions in tests/integration/preconditions_time_test.go

## Phase 4: User Story 2 - Environment Conditions (P1)

Implement environment variable-based preconditions.

Independent Test: Can be fully tested by creating a task with environment conditions and verifying it only executes when the specified environment variables are present.

- [ ] T016 [P] [US2] Add env_set validation to precondition parser in internal/project/project.go
- [ ] T017 [US2] Extend precondition evaluation to include environment checks in internal/project/preconditions/evaluator.go
- [ ] T018 [P] [US2] Create unit tests for env_set evaluation in internal/project/preconditions/env_set_test.go
- [ ] T019 [US2] Create integration test for environment-based preconditions in tests/integration/preconditions_env_test.go

## Phase 5: User Story 3 - Complex Expressions (P2)

Implement expression-based preconditions for advanced logic.

Independent Test: Can be fully tested by creating a task with complex expressions and verifying it evaluates the expressions correctly.

- [ ] T020 [P] [US3] Add expr validation to precondition parser in internal/project/project.go
- [ ] T021 [US3] Implement Go template expression evaluation in internal/project/preconditions/expr.go
- [ ] T022 [P] [US3] Create unit tests for expression evaluation in internal/project/preconditions/expr_test.go
- [ ] T023 [US3] Create integration test for expression-based preconditions in tests/integration/preconditions_expr_test.go

## Phase 6: User Story 4 - Combined Logic (P1)

Implement combined precondition and pre_check evaluation.

Independent Test: Can be fully tested by creating a task with both precondition and pre_check, verifying both must pass for execution.

- [ ] T024 [US4] Modify task dispatch to evaluate both precondition and pre_check in internal/runner/runner.go
- [ ] T025 [P] [US4] Create unit tests for combined evaluation logic in internal/project/preconditions/evaluator_test.go
- [ ] T026 [US4] Create integration test for combined precondition/pre_check logic in tests/integration/preconditions_combined_test.go

## Phase 7: Polish & Cross-Cutting Concerns

Final enhancements and quality improvements.

- [ ] T027 [P] Add skip reason reporting for precondition failures in internal/project/preconditions/reporting.go
- [ ] T028 [P] Add precondition validation to anvil CLI in cmd/anvil/validate.go
- [ ] T029 Update documentation with precondition examples in docs/preconditions.md
- [ ] T030 Add precondition debugging command to CLI in cmd/anvil/debug_preconditions.go
- [ ] T031 Create comprehensive end-to-end test suite in tests/e2e/preconditions_test.go
- [ ] T032 Update README with precondition documentation
- [ ] T033 Add example task files with preconditions to example_proj/

## Parallel Execution Opportunities

Within each user story phase, these tasks can be executed in parallel:
- Model/structure creation tasks
- Unit test implementation tasks
- Validation logic tasks

Cross-story parallelization:
- US1, US2, US3 can be implemented simultaneously after foundational tasks
- US4 depends on completion of US1-US3
