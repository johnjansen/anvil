# Research for Task Wait Conditions

## Decision: Trigger Condition Architecture

**Decision**: Implement a flexible condition evaluation system that supports both AND/OR logic combinations with various condition types.

**Rationale**: This approach provides maximum flexibility for users while maintaining a clean separation between condition definition and evaluation logic. The system will evaluate conditions in a lazy manner, only checking conditions that are necessary to determine the final result.

**Alternatives considered**:
- Simple boolean expressions: Would be limiting for complex scenarios
- External DSL for conditions: Would add unnecessary complexity
- Plugin-based condition system: Would be overkill for the current scope

## Decision: Polling Mechanism

**Decision**: Implement a polling manager that runs as a background goroutine, checking conditions at specified intervals.

**Rationale**: This approach integrates well with the existing daemon architecture and allows for efficient resource usage by sharing polling timers across multiple tasks.

**Alternatives considered**:
- Individual polling goroutines per task: Would consume more resources
- System-level file watching: Not portable across platforms and limited to file conditions
- External queue-based polling: Would add infrastructure dependencies

## Decision: Condition Types

**Decision**: Start with file existence, environment variable, and custom command conditions as the core set.

**Rationale**: These cover the most common use cases mentioned in the feature request while keeping the initial implementation focused.

**Additional condition types planned**:
- Queue/item presence (for integration with message queues)
- HTTP endpoint availability
- Database query results
- Time-based conditions beyond cron schedules

## Decision: CLI Command Structure

**Decision**: Add `anvil task trigger-check <task-name>` command for manual evaluation.

**Rationale**: This follows the existing CLI pattern and provides users with debugging capabilities.

**Alternatives considered**:
- `anvil trigger-check`: Would conflict with potential global trigger commands
- `anvil task eval-trigger`: Less clear than "trigger-check"