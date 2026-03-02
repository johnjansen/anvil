# Data Model: Task Preconditions

## Entities

### Task (existing)

The Task entity represents a unit of work that can be scheduled and executed by the Anvil task scheduler.

**Fields**:
- Path (string): Absolute path to the task file
- Name (string): Filename of the task
- Priority (int): Task priority level (0-9)
- Content (string): Task content after frontmatter
- Schedule (string): Cron expression defining when task should run
- ID (string): Unique identifier for the task
- Resume (*bool): Whether task should resume after completion
- MaxConcurrent (int): Maximum concurrent instances
- SkipPermissions (bool): Whether to skip permission checks
- AllowedTools ([]string): List of allowed tools for this task
- PreCheck (string): Shell command to determine if task should run
- OnSuccess (string): Command to run after successful completion
- OnFailure (string): Command to run after failed completion
- IsLocked (bool): Whether task has a stale lock file
- Disabled (bool): Whether task is disabled

### Precondition (new)

The Precondition entity defines conditional logic that determines whether a task should execute.

**Fields**:
- DayOfWeek (string): Days of week when task should run (e.g., "1-5" for weekdays)
- TimeRange (string): Time range when task should run (e.g., "09:00-17:00")
- EnvSet (string): Environment variable that must be set for task to run
- Expr (string): Complex expression using template variables for conditional logic

**Validation Rules**:
- DayOfWeek must be a valid day range specification (0-6, with optional ranges)
- TimeRange must be in HH:MM-HH:MM format
- EnvSet must be a valid environment variable name
- Expr must be a valid Go template expression

### TemplateVariable (new)

The TemplateVariable entity provides contextual information for expression evaluation.

**Fields**:
- Hour (int): Current hour (0-23)
- Minute (int): Current minute (0-59)
- DayOfWeek (int): Current day of week (0-6, Sunday = 0)
- DayOfMonth (int): Current day of month (1-31)
- Month (int): Current month (1-12)
- IsWeekend (bool): Whether current day is a weekend

## Relationships

### Task → Precondition (1:1)

Each Task may have zero or one Precondition. When a Precondition exists, it must evaluate to true for the task to execute.

### Precondition → TemplateVariable (1:1)

Each Precondition evaluation has access to current TemplateVariable values for expression evaluation.

## State Transitions

### Task Evaluation Flow

1. Parse task file and extract frontmatter
2. If precondition exists, evaluate precondition fields
3. If precondition.expr exists, evaluate expression with template variables
4. If precondition evaluation fails, skip task with reason
5. If precondition passes, evaluate existing pre_check command
6. If pre_check fails, skip task with reason
7. If both precondition and pre_check pass, execute task

## Constraints

- Precondition evaluation must not have side effects
- Template variables must reflect current time at evaluation
- Precondition evaluation should complete in <1ms
- All precondition fields are optional but if present, all must pass
