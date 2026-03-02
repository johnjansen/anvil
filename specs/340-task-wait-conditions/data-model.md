# Data Model for Task Wait Conditions

## TaskTrigger Entity

Represents the trigger configuration for a task with multiple conditions.

### Fields

- **schedule**: String - Cron expression for scheduled triggering
- **conditions**: []Condition - List of additional conditions that must be evaluated
- **conditionLogic**: String - "AND" or "OR" to specify how conditions should be combined
- **pollingConfig**: PollingConfig - Configuration for polling-based triggers
- **manualTriggerOnly**: Boolean - If true, task can only be triggered manually

## Condition Entity

Represents a single condition that must be evaluated.

### Fields

- **type**: String - Type of condition ("fileExists", "envVarSet", "command", etc.)
- **value**: String - Value to check against (filepath, environment variable name, command to run)
- **expectedResult**: String - Expected result for comparison (optional)
- **timeout**: Duration - Timeout for condition evaluation (especially for commands)

## PollingConfig Entity

Represents the configuration for polling-based triggers.

### Fields

- **enabled**: Boolean - Whether polling is enabled
- **interval**: Duration - How often to check conditions
- **timeout**: Duration - Maximum time to wait for conditions to be met
- **runOnce**: Boolean - If true, task runs only once when conditions are met

## TaskTriggerEvaluation Entity

Represents the result of evaluating trigger conditions for a task.

### Fields

- **taskId**: String - ID of the task being evaluated
- **timestamp**: Time - When the evaluation occurred
- **conditionsMet**: Boolean - Whether all conditions were met
- **conditionResults**: []ConditionResult - Detailed results for each condition
- **triggered**: Boolean - Whether the task was triggered as a result

## ConditionResult Entity

Represents the result of evaluating a single condition.

### Fields

- **conditionId**: String - Reference to the condition being evaluated
- **met**: Boolean - Whether the condition was met
- **message**: String - Explanation of the result
- **duration**: Duration - How long the evaluation took