# CLI Contract: Task Trigger Commands

## anvil task trigger-check

Evaluate trigger conditions for a task without executing it.

### Usage

```bash
anvil task trigger-check <task-name>
```

### Arguments

- `<task-name>`: Name of the task to evaluate

### Output

JSON output showing:
- Overall condition status (met/not met)
- Individual condition results
- Reasons for any failed conditions
- Time taken for evaluation

### Exit Codes

- 0: Evaluation completed successfully
- 1: Task not found
- 2: Evaluation error occurred

### Examples

```bash
# Check if a task's conditions are currently met
anvil task trigger-check daily-report

# Output detailed results
anvil task trigger-check data-import --verbose
```