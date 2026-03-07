package project

import (
	"time"
)

// Condition represents a single condition that must be evaluated for task triggering.
type Condition struct {
	// Type specifies the type of condition to evaluate.
	// Supported types: "fileExists", "envVarSet", "command"
	Type string `yaml:"type"`

	// Value contains the value to check against, depending on the condition type:
	// - For "fileExists": the file path to check
	// - For "envVarSet": the environment variable name to check
	// - For "command": the shell command to execute
	Value string `yaml:"value"`

	// ExpectedResult is the expected result for comparison (optional).
	// For command conditions, this is compared against the command output.
	ExpectedResult string `yaml:"expected_result,omitempty"`

	// Timeout is the maximum time allowed for condition evaluation.
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// TaskTrigger represents the trigger configuration for a task with multiple conditions.
type TaskTrigger struct {
	// Schedule is the cron expression for scheduled triggering (maintained for backward compatibility)
	Schedule string `yaml:"schedule,omitempty"`

	// Conditions is a list of additional conditions that must be evaluated.
	Conditions []Condition `yaml:"conditions,omitempty"`

	// ConditionLogic specifies how conditions should be combined ("AND" or "OR").
	// Default is "AND" if not specified.
	ConditionLogic string `yaml:"condition_logic,omitempty"`

	// PollingConfig contains configuration for polling-based triggers.
	PollingConfig *PollingConfig `yaml:"polling_config,omitempty"`

	// ManualTriggerOnly indicates if the task can only be triggered manually.
	ManualTriggerOnly bool `yaml:"manual_trigger_only,omitempty"`
}

// PollingConfig represents the configuration for polling-based triggers.
type PollingConfig struct {
	// Enabled indicates whether polling is enabled.
	Enabled bool `yaml:"enabled"`

	// Interval is how often to check conditions.
	Interval time.Duration `yaml:"interval"`

	// Timeout is the maximum time to wait for conditions to be met.
	Timeout time.Duration `yaml:"timeout"`

	// RunOnce indicates if the task should run only once when conditions are met.
	RunOnce bool `yaml:"run_once"`
}

// TaskTriggerEvaluation represents the result of evaluating trigger conditions for a task.
type TaskTriggerEvaluation struct {
	// TaskID is the ID of the task being evaluated.
	TaskID string `json:"task_id"`

	// Timestamp is when the evaluation occurred.
	Timestamp time.Time `json:"timestamp"`

	// ConditionsMet indicates whether all conditions were met.
	ConditionsMet bool `json:"conditions_met"`

	// ConditionResults contains detailed results for each condition.
	ConditionResults []ConditionResult `json:"condition_results"`

	// Triggered indicates whether the task was triggered as a result.
	Triggered bool `json:"triggered"`
}

// ConditionResult represents the result of evaluating a single condition.
type ConditionResult struct {
	// ConditionID is a reference to the condition being evaluated.
	ConditionID string `json:"condition_id"`

	// Met indicates whether the condition was met.
	Met bool `json:"met"`

	// Message is an explanation of the result.
	Message string `json:"message"`

	// Duration is how long the evaluation took.
	Duration time.Duration `json:"duration"`
}