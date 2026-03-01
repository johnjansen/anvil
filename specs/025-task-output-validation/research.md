# Research: Task Output Validation with Assertions

## Decision: Where to implement assertion evaluation

**Chosen**: In the runner module (`internal/runner/assertion.go`)
**Rationale**: The runner is responsible for executing tasks and capturing their output. Placing assertion evaluation here keeps the logic close to where output is captured and makes it available for both manual task runs and daemon-scheduled tasks.

**Alternatives considered**:
- In the daemon module - Would only work for daemon-scheduled tasks
- In a new module - Would add unnecessary complexity
- In the project module - Would mix configuration with execution logic

## Decision: How to integrate with existing task execution flow

**Chosen**: Add assertion evaluation after task completion but before considering the task successful
**Rationale**: This ensures that tasks are only marked as successful if they meet all configured assertions. Failure hooks will be triggered naturally when assertions fail, maintaining consistency with existing error handling.

**Alternatives considered**:
- Separate assertion evaluation step - Would complicate the execution flow
- Pre-execution validation - Not applicable since we're validating output, not inputs

## Decision: File assertion implementation approach

**Chosen**: Check file properties (existence, content, size) using standard Go file operations
**Rationale**: Go's standard library provides robust file operations that are efficient and reliable. This approach leverages existing Go expertise and avoids external dependencies.

**Alternatives considered**:
- External file validation tools - Would add dependencies and complexity
- Shell commands for file checks - Less portable and harder to test

## Decision: Regex engine for pattern matching

**Chosen**: Go's standard `regexp` package
**Rationale**: The Go standard library's regexp package is well-tested, efficient, and familiar to Go developers. It supports the full range of regex features needed for assertion validation.

**Alternatives considered**:
- External regex libraries - Would add dependencies
- Simple string matching only - Would limit functionality

## Decision: JSON validation approach

**Chosen**: Use Go's `encoding/json` package to parse and validate JSON
**Rationale**: Go's standard JSON package is efficient and reliable. Simply attempting to unmarshal the content into an interface{} will validate that it's syntactically correct JSON.

**Alternatives considered**:
- External JSON validation libraries - Would add dependencies
- Schema validation - Beyond the scope of basic JSON validity checking

## Decision: Soft assertion implementation

**Chosen**: Evaluate soft assertions the same as hard assertions, but log failures as warnings instead of returning errors
**Rationale**: This approach minimizes code duplication while providing the flexibility users need. The same validation logic can be used for both types of assertions.

**Alternatives considered**:
- Separate evaluation logic - Would duplicate code
- Different return types - Would complicate the API

## Decision: Error message format

**Chosen**: Clear, specific messages indicating which assertion failed and why
**Rationale**: Good error messages are essential for debugging. Users need to quickly understand why their assertions failed to fix their tasks.

**Alternatives considered**:
- Generic error messages - Would make debugging difficult
- Very verbose error messages - Could overwhelm users with unnecessary details