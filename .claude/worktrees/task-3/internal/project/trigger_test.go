package project

import (
	"testing"
	"time"
)

func TestTaskTriggerStruct(t *testing.T) {
	// Test that we can create a TaskTrigger struct
	trigger := TaskTrigger{
		Schedule:       "0 9 * * *",
		Conditions:     []Condition{{Type: "fileExists", Value: "test.txt"}},
		ConditionLogic: "AND",
		PollingConfig: &PollingConfig{
			Enabled:   true,
			Interval:  30 * time.Second,
			Timeout:   1 * time.Hour,
			RunOnce:   true,
		},
		ManualTriggerOnly: false,
	}

	if trigger.Schedule != "0 9 * * *" {
		t.Error("Schedule field not set correctly")
	}

	if len(trigger.Conditions) != 1 {
		t.Error("Conditions slice not set correctly")
	}

	if trigger.ConditionLogic != "AND" {
		t.Error("ConditionLogic field not set correctly")
	}

	if trigger.PollingConfig == nil {
		t.Error("PollingConfig not set correctly")
	}

	if !trigger.PollingConfig.Enabled {
		t.Error("PollingConfig.Enabled not set correctly")
	}
}

func TestPollingConfigStruct(t *testing.T) {
	// Test that we can create a PollingConfig struct
	config := PollingConfig{
		Enabled:   true,
		Interval:  30 * time.Second,
		Timeout:   1 * time.Hour,
		RunOnce:   false,
	}

	if !config.Enabled {
		t.Error("Enabled field not set correctly")
	}

	if config.Interval != 30*time.Second {
		t.Error("Interval field not set correctly")
	}

	if config.Timeout != 1*time.Hour {
		t.Error("Timeout field not set correctly")
	}

	if config.RunOnce {
		t.Error("RunOnce field not set correctly")
	}
}

func TestConditionStruct(t *testing.T) {
	// Test that we can create a Condition struct
	condition := Condition{
		Type:           "fileExists",
		Value:          "test.txt",
		ExpectedResult: "expected_content",
		Timeout:        30 * time.Second,
	}

	if condition.Type != "fileExists" {
		t.Error("Type field not set correctly")
	}

	if condition.Value != "test.txt" {
		t.Error("Value field not set correctly")
	}

	if condition.ExpectedResult != "expected_content" {
		t.Error("ExpectedResult field not set correctly")
	}

	if condition.Timeout != 30*time.Second {
		t.Error("Timeout field not set correctly")
	}
}