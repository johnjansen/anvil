# Task Breakdown: Task Output Validation with Assertions

**Feature**: Task Output Validation with Assertions
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

## Implementation Strategy

This feature can be implemented incrementally, starting with the core assertion evaluation logic and expanding to support all assertion types. The MVP should include:

1. Basic stdout/stderr assertion support (contains, empty)
2. File existence assertions
3. Integration with task execution flow
4. Error messaging

Additional assertion types (regex matching, JSON validation, file content/size checks, soft assertions) can be added incrementally.

## Dependencies

- Implementation follows the pattern established by existing features like task SLA tracking
- Requires changes to task execution flow in the runner module
- Uses existing configuration and run record patterns

## Phases

### Phase 1: Setup

Foundational tasks that must be completed before implementing user stories.

- [ ] T001 Create project structure per implementation plan

### Phase 2: Foundational

Core infrastructure and shared components needed by all user stories.

- [ ] T002 [P] Add AssertionConfig struct to internal/project/project.go
- [ ] T003 [P] Create assertion evaluation module in internal/runner/assertion.go
- [ ] T004 [P] Add assertion evaluation to task execution flow in internal/runner/runner.go

### Phase 3: User Story 1 - Stdout/Stderr Assertion Validation

Support for validating stdout and stderr content with basic assertions.

#### Independent Test Criteria

Can be tested by creating a task with stdout/stderr assertions and verifying that:
1. Tasks with passing assertions succeed
2. Tasks with failing assertions fail with appropriate error messages
3. Tasks without assertions behave identically to current behavior

#### Implementation Tasks

- [ ] T005 [P] [US1] Implement stdout/stderr contains assertion evaluation in internal/runner/assertion.go
- [ ] T006 [P] [US1] Implement stdout/stderr empty assertion evaluation in internal/runner/assertion.go
- [ ] T007 [US1] Add error messaging for stdout/stderr assertion failures
- [ ] T008 [P] [US1] Create tests for stdout/stderr assertion evaluation in internal/runner/assertion_test.go

### Phase 4: User Story 2 - File Content Assertion Validation

Support for validating file properties with existence, content, and size assertions.

#### Independent Test Criteria

Can be tested by creating a task with file assertions and verifying that:
1. Tasks with passing file assertions succeed
2. Tasks with failing file assertions fail with appropriate error messages
3. File assertions work with both relative and absolute paths

#### Implementation Tasks

- [ ] T009 [P] [US2] Implement file existence assertion evaluation in internal/runner/assertion.go
- [ ] T010 [P] [US2] Implement file content assertion evaluation in internal/runner/assertion.go
- [ ] T011 [P] [US2] Implement file size assertion evaluation in internal/runner/assertion.go
- [ ] T012 [US2] Add error messaging for file assertion failures
- [ ] T013 [P] [US2] Create tests for file assertion evaluation in internal/runner/assertion_test.go

### Phase 5: User Story 3 - Soft Assertions

Support for assertions that log warnings instead of failing tasks.

#### Independent Test Criteria

Can be tested by creating a task with soft assertions and verifying that:
1. Tasks with failing soft assertions succeed but log warnings
2. Tasks with passing soft assertions succeed with no warnings
3. Hard and soft assertions can coexist correctly

#### Implementation Tasks

- [ ] T014 [P] [US3] Implement soft assertion evaluation in internal/runner/assertion.go
- [ ] T015 [US3] Add warning logging for soft assertion failures
- [ ] T016 [P] [US3] Create tests for soft assertion evaluation in internal/runner/assertion_test.go

### Phase 6: User Story 4 - Clear Error Messaging

Enhanced error messages that clearly indicate which assertion failed and why.

#### Independent Test Criteria

Can be tested by creating tasks with various failing assertions and verifying that:
1. Error messages clearly identify the failing assertion
2. Error messages include relevant details (expected values, actual values)
3. Error messages are consistent across different assertion types

#### Implementation Tasks

- [ ] T017 [P] [US4] Enhance error messages for stdout/stderr assertion failures
- [ ] T018 [P] [US4] Enhance error messages for file assertion failures
- [ ] T019 [US4] Create comprehensive error message tests in internal/runner/assertion_test.go

### Phase 7: Polish & Cross-Cutting Concerns

Final improvements and integration with existing features.

- [ ] T020 [P] Add assertion information to task get command output
- [ ] T021 [P] Update documentation with assertion examples
- [ ] T022 Run integration tests with all assertion types
- [ ] T023 Verify backward compatibility with existing tasks
- [ ] T024 Add examples to quickstart guide

## Parallel Execution Opportunities

Several tasks can be implemented in parallel:
- All [P] marked tasks within each phase
- Implementation of different assertion types (stdout/stderr, file, soft)
- Test creation can happen in parallel with implementation
- Documentation updates can happen in parallel with code changes

## User Story Dependencies

- US1 (Stdout/Stderr): Independent - can be implemented first
- US2 (File Content): Independent - can be implemented in parallel with US1
- US3 (Soft Assertions): Independent - can be implemented anytime after foundational tasks
- US4 (Error Messaging): Independent - enhancements can be added incrementally

Each user story can be implemented and tested independently, allowing for flexible development and deployment.