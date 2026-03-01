# Feature Specification: Task Output Validation with Assertions

**Feature Branch**: `025-task-output-validation`
**Created**: 2026-03-02
**Status**: Draft
**Input**: User description: "Add task output validation with assertions. Currently, there's no way to validate task output before considering it successful. Tasks may complete without errors but produce incorrect or unexpected output. Users only discover issues later when downstream tasks fail or reports are wrong. Add output assertions: 1. Assert on stdout/stderr with contains, matches, json_valid, empty checks. 2. Assert on files with exists, contains, size_min, size_max checks. 3. Assertion failure triggers retry and failure hooks. 4. Optional soft assertions that log warning but don't fail. 5. Clear error messaging showing which assertion failed."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Stdout/Stderr Assertion Validation (Priority: P1)

A user wants to ensure their task produces expected output content. They add an `assert.stdout.contains` condition to their task's frontmatter. When the task runs, the system checks the stdout against the assertion. If the stdout doesn't contain the expected string, the task is marked as failed and retry/failure hooks are triggered.

**Why this priority**: This is the core value proposition — validating task output content. Without this, the feature has no purpose.

**Independent Test**: Can be tested by creating a task with `assert.stdout.contains: "Success"` and verifying that when the task's stdout contains "Success", it passes, but when it doesn't, it fails with appropriate error messaging.

**Acceptance Scenarios**:

1. **Given** a task with `assert.stdout.contains: "Success"`, **When** the task outputs "Operation completed. Success.", **Then** the task succeeds with no assertion errors.
2. **Given** the same task, **When** the task outputs "Operation failed.", **Then** the task fails with error "Assertion failed: stdout does not contain 'Success'" and triggers retry/failure hooks.
3. **Given** a task with `assert.stderr.empty: true`, **When** the task produces no stderr output, **Then** the task succeeds.
4. **Given** the same task, **When** the task produces stderr output, **Then** the task fails with error "Assertion failed: stderr is not empty" and triggers retry/failure hooks.
5. **Given** a task with `assert.stdout.matches: "\\d+ records"`, **When** the task outputs "Processed 42 records successfully", **Then** the task succeeds.
6. **Given** the same task, **When** the task outputs "Processing failed", **Then** the task fails with error "Assertion failed: stdout does not match '\\d+ records'" and triggers retry/failure hooks.

---

### User Story 2 - File Content Assertion Validation (Priority: P1)

A user wants to ensure their task creates files with expected content and properties. They add `assert.files` conditions to their task's frontmatter. When the task runs, the system checks the specified files against the assertions. If any file assertion fails, the task is marked as failed and retry/failure hooks are triggered.

**Why this priority**: File validation is equally important as stdout/stderr validation for many tasks that produce output files.

**Independent Test**: Can be tested by creating a task with file assertions and verifying that when files meet the criteria, the task passes, but when they don't, it fails with appropriate error messaging.

**Acceptance Scenarios**:

1. **Given** a task with `assert.files[0].path: "output.json"` and `assert.files[0].exists: true`, **When** the task creates output.json, **Then** the task succeeds.
2. **Given** the same task, **When** the task does not create output.json, **Then** the task fails with error "Assertion failed: file 'output.json' does not exist" and triggers retry/failure hooks.
3. **Given** a task with `assert.files[0].path: "report.csv"` and `assert.files[0].contains: "status: ok"`, **When** report.csv contains "status: ok", **Then** the task succeeds.
4. **Given** the same task, **When** report.csv does not contain "status: ok", **Then** the task fails with error "Assertion failed: file 'report.csv' does not contain 'status: ok'" and triggers retry/failure hooks.
5. **Given** a task with `assert.files[0].path: "data.txt"` and `assert.files[0].size_min: 1000`, **When** data.txt is 1500 bytes, **Then** the task succeeds.
6. **Given** the same task, **When** data.txt is 500 bytes, **Then** the task fails with error "Assertion failed: file 'data.txt' size 500 bytes is less than minimum 1000 bytes" and triggers retry/failure hooks.

---

### User Story 3 - Soft Assertions (Priority: P2)

A user wants to be warned about potential issues without failing the task. They add `assert.soft: true` to their task's frontmatter along with assertions. When the task runs, if soft assertions fail, they are logged as warnings but the task still succeeds.

**Why this priority**: Provides flexibility for non-critical validations that shouldn't block task completion.

**Independent Test**: Can be tested by creating a task with soft assertions and verifying that when assertions fail, warnings are logged but the task still succeeds.

**Acceptance Scenarios**:

1. **Given** a task with `assert.soft: true` and `assert.stdout.contains: "Warning"`, **When** the task outputs "Warning: Low disk space", **Then** the task succeeds but logs a warning about the assertion.
2. **Given** the same task, **When** the task outputs "All systems normal", **Then** the task succeeds with no warnings.
3. **Given** a task with `assert.soft: true` and multiple failing assertions, **When** the task runs, **Then** all failing assertions are logged as warnings but the task still succeeds.

---

### User Story 4 - Clear Error Messaging (Priority: P2)

A user wants to quickly understand why their task failed due to assertions. When an assertion fails, they see clear, specific error messages indicating which assertion failed and why.

**Why this priority**: Essential for debugging and user experience but depends on the core assertion functionality.

**Independent Test**: Can be tested by creating tasks with various failing assertions and verifying the error messages are clear and specific.

**Acceptance Scenarios**:

1. **Given** a task with `assert.stdout.contains: "Success"` that fails, **When** the task runs, **Then** the error message is "Assertion failed: stdout does not contain 'Success'".
2. **Given** a task with `assert.files[0].path: "missing.txt"` and `assert.files[0].exists: true` that fails, **Then** the error message is "Assertion failed: file 'missing.txt' does not exist".
3. **Given** a task with multiple failing assertions, **When** the task runs, **Then** all failing assertions are reported in the error message.

---

### Edge Cases

- What happens when a file assertion refers to a path that doesn't exist? The assertion fails with a clear error message.
- What happens when a regex pattern in matches assertion is invalid? The assertion fails with a clear error about the invalid regex.
- What happens when a task has both hard and soft assertions, and some of each fail? Hard assertion failures cause task failure; soft assertion failures are logged as warnings.
- What happens when assertion evaluation itself fails (e.g., file permission error)? The task fails with an error about the assertion evaluation failure.
- What happens when a task has no assertions configured? The task behaves exactly as before (backward compatible).
- How are assertion failures persisted for debugging? Assertion failure details are logged and may be included in task run records.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support an `assert` configuration block per task with stdout/stderr assertion fields in task frontmatter.
- **FR-002**: System MUST support `assert.stdout.contains` string matching that passes when stdout contains the specified string.
- **FR-003**: System MUST support `assert.stdout.matches` regex matching that passes when stdout matches the specified regex pattern.
- **FR-004**: System MUST support `assert.stdout.json_valid` boolean that passes when stdout is valid JSON.
- **FR-005**: System MUST support `assert.stderr.empty` boolean that passes when stderr is empty.
- **FR-006**: System MUST support `assert.files` array with file path assertions including `exists`, `contains`, `size_min`, and `size_max`.
- **FR-007**: System MUST fail task execution when any hard assertion fails, triggering retry and failure hooks.
- **FR-008**: System MUST support `assert.soft: true` option that logs assertion failures as warnings without failing the task.
- **FR-009**: System MUST provide clear, specific error messages indicating which assertion failed and why.
- **FR-010**: System MUST be fully backward compatible - tasks without assertions behave identically to current behavior.
- **FR-011**: System MUST handle assertion evaluation errors gracefully, failing the task with clear error messages about the evaluation failure.

### Key Entities

- **Assertion Config**: Per-task configuration with stdout/stderr and file assertion criteria. Controls what constitutes valid task output.
- **Assertion Result**: The outcome of evaluating an assertion against actual task output, either pass or fail with specific details.
- **Soft Assertion Flag**: Configuration option that determines whether assertion failures cause task failure or just log warnings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can configure output assertions per task and receive immediate pass/fail determination with clear error messages when assertions fail.
- **SC-002**: Users can use soft assertions to warn about potential issues without blocking task completion.
- **SC-003**: Users can validate both stdout/stderr content and file properties with a single task configuration.
- **SC-004**: Users receive specific, actionable error messages that clearly indicate which assertion failed and why.
- **SC-005**: Existing tasks without assertions continue to operate identically to current behavior (zero breaking changes).
- **SC-006**: Failed assertions trigger the same retry and failure hooks as other task failures, maintaining consistent error handling.

## Assumptions

- Assertion evaluation happens after task completion but before considering the task successful.
- File assertions check file existence and content at the time of assertion evaluation.
- Regex patterns in `matches` assertions use standard Go regex syntax.
- Soft assertions are evaluated the same as hard assertions, but failures are logged as warnings instead of causing task failure.
- Assertion failures are logged with sufficient detail for debugging but don't expose sensitive content.
- The assertion system integrates with existing task execution and hook mechanisms without requiring changes to those systems.