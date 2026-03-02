# Quick Start: Task Preconditions

## Overview

Task preconditions allow you to define conditional logic that determines when tasks should execute. This guide shows you how to use preconditions in your tasks.

## Basic Example

Add a precondition to your task's frontmatter:

```yaml
---
schedule: "0 9 * * *"
precondition:
  day_of_week: "1-5"  # weekdays only
---
# Your task content here
```

This task will only run on weekdays at 9 AM.

## Time-Based Preconditions

### Business Hours Only

```yaml
---
schedule: "*/30 8-17 * * 1-5"
precondition:
  time_range: "08:00-17:00"
  day_of_week: "1-5"
---
# Task runs every 30 minutes, but only during business hours on weekdays
```

### Weekend Maintenance

```yaml
---
schedule: "0 2 * * *"
precondition:
  expr: "{{ .IsWeekend }}"
---
# Task runs at 2 AM only on weekends
```

## Environment-Based Preconditions

### CI/CD Only

```yaml
---
schedule: "0 9 * * *"
precondition:
  env_set: "CI"
---
# Task only runs when CI environment variable is set
```

## Complex Expressions

### Custom Business Logic

```yaml
---
schedule: "0 9 * * *"
precondition:
  expr: "{{ .Hour >= 9 && .Hour < 17 && .DayOfWeek < 6 && .DayOfMonth != 1 }}"
---
# Task runs at 9 AM on weekdays, excluding the 1st of each month
```

## Combining With Pre-check

```yaml
---
schedule: "0 9 * * *"
precondition:
  day_of_week: "1-5"
pre_check: "test -f /tmp/system-ready"
---
# Task runs on weekdays, but only if /tmp/system-ready file exists
```

## Viewing Skipped Tasks

Check why tasks were skipped:

```bash
anvil queue --skipped
```

This will show detailed reasons for precondition failures.

## Common Patterns

### Development vs Production

```yaml
---
schedule: "*/5 * * * *"
precondition:
  env_set: "ANVIL_ENV"
  expr: "{{ .Env.ANVIL_ENV == \"production\" }}"
---
# Task only runs in production environment
```

### Monthly First Business Day

```yaml
---
schedule: "0 9 1-7 * *"
precondition:
  expr: "{{ .DayOfWeek >= 1 && .DayOfWeek <= 5 }}"
---
# Task runs at 9 AM on first weekday of month
```

## Troubleshooting

### Debugging Preconditions

1. Use `anvil queue --skipped` to see skip reasons
2. Test expressions locally with `anvil debug precondition`
3. Check environment variables with `echo $VARNAME`

### Common Issues

1. **Invalid syntax**: Check YAML formatting and expression syntax
2. **Wrong timezone**: Preconditions use system timezone
3. **Environment variables**: Ensure variables are exported in task context
