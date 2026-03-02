package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// EvaluateTriggerConditions evaluates all conditions in a TaskTrigger and returns the result.
func EvaluateTriggerConditions(ctx context.Context, trigger TaskTrigger, task Todo) (*TaskTriggerEvaluation, error) {
	eval := &TaskTriggerEvaluation{
		TaskID:        task.ID,
		Timestamp:     time.Now(),
		ConditionResults: make([]ConditionResult, len(trigger.Conditions)),
	}

	// If there are no conditions, return early
	if len(trigger.Conditions) == 0 {
		eval.ConditionsMet = true
		return eval, nil
	}

	allMet := true
	logic := strings.ToUpper(trigger.ConditionLogic)
	if logic == "" {
		logic = "AND" // Default to AND logic
	}

	// Evaluate each condition
	for i, condition := range trigger.Conditions {
		result := &eval.ConditionResults[i]
		result.ConditionID = fmt.Sprintf("%s-%d", condition.Type, i)

		start := time.Now()
		met, err := evaluateCondition(ctx, condition, task)
		result.Duration = time.Since(start)

		if err != nil {
			result.Met = false
			result.Message = fmt.Sprintf("Error evaluating condition: %v", err)
			allMet = false
		} else {
			result.Met = met
			if met {
				result.Message = "Condition met"
			} else {
				result.Message = "Condition not met"
				allMet = false
			}
		}
	}

	// Apply logic (AND or OR)
	if logic == "OR" {
		// For OR logic, at least one condition must be met
		eval.ConditionsMet = !allMet // This is wrong, let me fix it
		// Actually, we need to check if any condition was met
		eval.ConditionsMet = false
		for _, result := range eval.ConditionResults {
			if result.Met {
				eval.ConditionsMet = true
				break
			}
		}
	} else {
		// For AND logic (default), all conditions must be met
		eval.ConditionsMet = allMet
	}

	return eval, nil
}

// evaluateCondition evaluates a single condition based on its type.
func evaluateCondition(ctx context.Context, condition Condition, task Todo) (bool, error) {
	switch condition.Type {
	case "fileExists":
		return evaluateFileExists(condition.Value)
	case "envVarSet":
		return evaluateEnvVarSet(condition.Value, task)
	case "command":
		return evaluateCommand(ctx, condition.Value, condition.ExpectedResult, condition.Timeout)
	default:
		return false, fmt.Errorf("unsupported condition type: %s", condition.Type)
	}
}

// evaluateFileExists checks if a file exists at the specified path.
func evaluateFileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// evaluateEnvVarSet checks if an environment variable is set.
func evaluateEnvVarSet(varName string, task Todo) (bool, error) {
	// Check task-specific environment variables first
	if task.Env != nil {
		if _, exists := task.Env[varName]; exists {
			return true, nil
		}
	}

	// Check system environment variables
	_, exists := os.LookupEnv(varName)
	return exists, nil
}

// evaluateCommand executes a command and checks if it succeeds or matches expected output.
func evaluateCommand(ctx context.Context, command, expectedResult string, timeout time.Duration) (bool, error) {
	// Set a timeout if specified
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Split command into executable and arguments
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false, fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.Output()

	// If there's an error executing the command, condition is not met
	if err != nil {
		return false, nil
	}

	// If expected result is specified, check if output contains it
	if expectedResult != "" {
		return strings.Contains(string(output), expectedResult), nil
	}

	// If no expected result, command success is sufficient
	return true, nil
}

// ShouldTriggerTask determines if a task should be triggered based on its trigger configuration.
func ShouldTriggerTask(ctx context.Context, task Todo, trigger TaskTrigger) (bool, error) {
	// If manual trigger only, don't trigger automatically
	if trigger.ManualTriggerOnly {
		return false, nil
	}

	// If polling is enabled, let the polling manager handle it
	if trigger.PollingConfig != nil && trigger.PollingConfig.Enabled {
		return false, nil
	}

	// Evaluate trigger conditions
	eval, err := EvaluateTriggerConditions(ctx, trigger, task)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate trigger conditions: %w", err)
	}

	return eval.ConditionsMet, nil
}