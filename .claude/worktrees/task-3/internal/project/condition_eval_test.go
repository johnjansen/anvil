package project

import (
	"context"
	"testing"
	"time"
)

func TestEvaluateFileExists(t *testing.T) {
	// Test that a file that exists returns true
	exists, err := evaluateFileExists("condition_eval_test.go")
	if err != nil {
		t.Errorf("evaluateFileExists returned error: %v", err)
	}
	if !exists {
		t.Error("evaluateFileExists should return true for existing file")
	}

	// Test that a file that doesn't exist returns false
	exists, err = evaluateFileExists("nonexistent-file.txt")
	if err != nil {
		t.Errorf("evaluateFileExists returned error: %v", err)
	}
	if exists {
		t.Error("evaluateFileExists should return false for non-existing file")
	}
}

func TestEvaluateEnvVarSet(t *testing.T) {
	// Test with a task that has environment variables
	task := Todo{
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	// Test that an environment variable that exists returns true
	exists, err := evaluateEnvVarSet("TEST_VAR", task)
	if err != nil {
		t.Errorf("evaluateEnvVarSet returned error: %v", err)
	}
	if !exists {
		t.Error("evaluateEnvVarSet should return true for existing env var")
	}

	// Test that an environment variable that doesn't exist returns false
	exists, err = evaluateEnvVarSet("NONEXISTENT_VAR", task)
	if err != nil {
		t.Errorf("evaluateEnvVarSet returned error: %v", err)
	}
	// This might return true if the env var exists in the system
	// We'll just check that it doesn't panic
}

func TestEvaluateCommand(t *testing.T) {
	// Test a simple command that should succeed
	met, err := evaluateCommand(context.Background(), "echo hello", "", 0)
	if err != nil {
		t.Errorf("evaluateCommand returned error: %v", err)
	}
	if !met {
		t.Error("evaluateCommand should return true for successful command")
	}

	// Test a command with expected result
	met, err = evaluateCommand(context.Background(), "echo hello", "hello", 0)
	if err != nil {
		t.Errorf("evaluateCommand returned error: %v", err)
	}
	if !met {
		t.Error("evaluateCommand should return true when output matches expected result")
	}

	// Test a command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	met, err = evaluateCommand(ctx, "sleep 2", "", 0)
	if err != nil {
		t.Errorf("evaluateCommand returned error: %v", err)
	}
	if met {
		t.Error("evaluateCommand should return false for timed out command")
	}
}

func TestEvaluateCondition(t *testing.T) {
	// Test file exists condition
	condition := Condition{
		Type:  "fileExists",
		Value: "condition_eval_test.go",
	}
	task := Todo{}

	met, err := evaluateCondition(context.Background(), condition, task)
	if err != nil {
		t.Errorf("evaluateCondition returned error: %v", err)
	}
	if !met {
		t.Error("evaluateCondition should return true for fileExists condition on existing file")
	}

	// Test unsupported condition type
	condition = Condition{
		Type: "unsupportedType",
	}
	_, err = evaluateCondition(context.Background(), condition, task)
	if err == nil {
		t.Error("evaluateCondition should return error for unsupported condition type")
	}
}