# Precondition Contract

## Frontmatter Extension

Tasks can include precondition fields in their YAML frontmatter:

```yaml
---
schedule: "0 9 * * *"
precondition:
  day_of_week: "1-5"      # only weekdays
  time_range: "09:00-17:00"  # only business hours
  env_set: "CI"            # only when CI env var is set
  expr: "{{ .Hour >= 9 && .Hour < 17 && .DayOfWeek < 6 }}"  # complex expression
pre_check: "test -f /tmp/ready"  # both precondition AND pre_check must pass
---
# Task content here
```

## Field Definitions

### day_of_week

Specifies days of the week when the task should run.

**Format**: String representing day numbers (0-6) where Sunday=0
- Single day: "1" (Monday)
- Range: "1-5" (Monday-Friday)
- List: "1,3,5" (Monday, Wednesday, Friday)
- Mixed: "1-3,5" (Monday-Wednesday and Friday)

### time_range

Specifies time range when the task should run.

**Format**: "HH:MM-HH:MM" in 24-hour format
- Example: "09:00-17:00" (9 AM to 5 PM)
- Start and end times are inclusive

### env_set

Specifies environment variable that must be set for task to run.

**Format**: String representing environment variable name
- Example: "CI" (variable must be set to any value)
- Empty string means no environment variable check

### expr

Specifies complex conditional expression using template variables.

**Format**: Go template expression with available variables:
- .Hour (int): Current hour (0-23)
- .Minute (int): Current minute (0-59)
- .DayOfWeek (int): Current day of week (0-6)
- .DayOfMonth (int): Current day of month (1-31)
- .Month (int): Current month (1-12)
- .IsWeekend (bool): Whether current day is weekend

**Examples**:
- "{{ .Hour >= 9 && .Hour < 17 }}" (business hours)
- "{{ .DayOfWeek >= 1 && .DayOfWeek <= 5 }}" (weekdays)
- "{{ .IsWeekend }}" (weekends only)

## Evaluation Rules

1. All specified precondition fields must evaluate to true
2. If any precondition field evaluates to false, task is skipped
3. Precondition evaluation occurs before pre_check command execution
4. Both precondition AND pre_check must pass for task to execute
5. Skip reasons are logged for failed precondition evaluations

## Template Variables

### .Hour

Current hour in 24-hour format (0-23)

### .Minute

Current minute (0-59)

### .DayOfWeek

Current day of week where Sunday=0, Monday=1, ..., Saturday=6

### .DayOfMonth

Current day of month (1-31)

### .Month

Current month (1-12)

### .IsWeekend

Boolean indicating whether current day is Saturday or Sunday

## Error Handling

### Invalid Field Values

- Invalid day_of_week format: Task is skipped with error message
- Invalid time_range format: Task is skipped with error message
- Invalid expr syntax: Task is skipped with error message

### Missing Environment Variables

- If env_set specifies a variable that doesn't exist: Task is skipped with reason

## Backward Compatibility

- Tasks without precondition fields continue to work unchanged
- Existing pre_check functionality remains unchanged
- New precondition fields are optional extensions
