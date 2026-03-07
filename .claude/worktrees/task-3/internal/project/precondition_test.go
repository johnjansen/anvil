package project

import (
	"os"
	"testing"
	"time"
)

func TestEvaluatePrecondition(t *testing.T) {
	// Test nil precondition
	t.Run("nil precondition", func(t *testing.T) {
		passed, err := EvaluatePrecondition(nil, time.Now())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !passed {
			t.Error("Expected precondition to pass with nil config")
		}
	})

	// Test day of week condition
	t.Run("day of week condition", func(t *testing.T) {
		// Create a precondition for weekdays (1-5)
		precond := &PreconditionConfig{
			DayOfWeek: "1-5",
		}

		// Test with a Wednesday (3)
		now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
		passed, err := EvaluatePrecondition(precond, now)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !passed {
			t.Error("Expected precondition to pass on Wednesday")
		}

		// Test with a Sunday (0)
		now = time.Date(2024, 1, 7, 12, 0, 0, 0, time.UTC)
		passed, err = EvaluatePrecondition(precond, now)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if passed {
			t.Error("Expected precondition to fail on Sunday")
		}
	})

	// Test time range condition
	t.Run("time range condition", func(t *testing.T) {
		// Create a precondition for business hours (9AM-5PM)
		precond := &PreconditionConfig{
			TimeRange: "09:00-17:00",
		}

		// Test with 2PM
		now := time.Date(2024, 1, 3, 14, 0, 0, 0, time.UTC)
		passed, err := EvaluatePrecondition(precond, now)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !passed {
			t.Error("Expected precondition to pass at 2PM")
		}

		// Test with 7AM
		now = time.Date(2024, 1, 3, 7, 0, 0, 0, time.UTC)
		passed, err = EvaluatePrecondition(precond, now)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if passed {
			t.Error("Expected precondition to fail at 7AM")
		}
	})

	// Test environment variable condition
	t.Run("environment variable condition", func(t *testing.T) {
		// Create a precondition requiring TEST_ENV_VAR to be set
		precond := &PreconditionConfig{
			EnvSet: "TEST_ENV_VAR",
		}

		// Test with variable not set
		os.Unsetenv("TEST_ENV_VAR")
		passed, err := EvaluatePrecondition(precond, time.Now())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if passed {
			t.Error("Expected precondition to fail when env var not set")
		}

		// Test with variable set
		os.Setenv("TEST_ENV_VAR", "test_value")
		passed, err = EvaluatePrecondition(precond, time.Now())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !passed {
			t.Error("Expected precondition to pass when env var is set")
		}

		// Clean up
		os.Unsetenv("TEST_ENV_VAR")
	})

	// Test expression condition
	t.Run("expression condition", func(t *testing.T) {
		// Create a precondition with a simple expression using template functions
		precond := &PreconditionConfig{
			Expr: "{{ and (ge .Hour 9) (lt .Hour 17) }}",
		}

		// Test with 2PM (should pass)
		now := time.Date(2024, 1, 3, 14, 0, 0, 0, time.UTC)
		passed, err := EvaluatePrecondition(precond, now)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !passed {
			t.Error("Expected precondition to pass at 2PM")
		}

		// Test with 7AM (should fail)
		now = time.Date(2024, 1, 3, 7, 0, 0, 0, time.UTC)
		passed, err = EvaluatePrecondition(precond, now)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if passed {
			t.Error("Expected precondition to fail at 7AM")
		}
	})
}

func TestMatchesDayOfWeek(t *testing.T) {
	// Test single day
	if !matchesDayOfWeek("3", time.Wednesday) {
		t.Error("Expected Wednesday to match '3'")
	}
	if matchesDayOfWeek("3", time.Tuesday) {
		t.Error("Expected Tuesday to not match '3'")
	}

	// Test range
	if !matchesDayOfWeek("1-5", time.Wednesday) {
		t.Error("Expected Wednesday to match '1-5'")
	}
	if matchesDayOfWeek("1-5", time.Sunday) {
		t.Error("Expected Sunday to not match '1-5'")
	}

	// Test comma-separated
	if !matchesDayOfWeek("1,3,5", time.Wednesday) {
		t.Error("Expected Wednesday to match '1,3,5'")
	}
	if matchesDayOfWeek("1,3,5", time.Tuesday) {
		t.Error("Expected Tuesday to not match '1,3,5'")
	}

	// Test combined format
	if !matchesDayOfWeek("1-5,0", time.Sunday) {
		t.Error("Expected Sunday to match '1-5,0'")
	}
}

func TestMatchesTimeRange(t *testing.T) {
	// Test normal range
	if !matchesTimeRange("09:00-17:00", time.Date(0, 1, 1, 12, 0, 0, 0, time.UTC)) {
		t.Error("Expected 12:00 to match '09:00-17:00'")
	}
	if matchesTimeRange("09:00-17:00", time.Date(0, 1, 1, 7, 0, 0, 0, time.UTC)) {
		t.Error("Expected 7:00 to not match '09:00-17:00'")
	}

	// Test overnight range
	if !matchesTimeRange("22:00-06:00", time.Date(0, 1, 1, 23, 0, 0, 0, time.UTC)) {
		t.Error("Expected 23:00 to match '22:00-06:00'")
	}
	if matchesTimeRange("22:00-06:00", time.Date(0, 1, 1, 12, 0, 0, 0, time.UTC)) {
		t.Error("Expected 12:00 to not match '22:00-06:00'")
	}
}

func TestEvaluateExpression(t *testing.T) {
	// Test simple true expression
	result, err := evaluateExpression("true", time.Now())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !result {
		t.Error("Expected 'true' expression to evaluate to true")
	}

	// Test simple false expression
	result, err = evaluateExpression("false", time.Now())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result {
		t.Error("Expected 'false' expression to evaluate to false")
	}

	// Test numeric true expression
	result, err = evaluateExpression("1", time.Now())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !result {
		t.Error("Expected '1' expression to evaluate to true")
	}

	// Test numeric false expression
	result, err = evaluateExpression("0", time.Now())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result {
		t.Error("Expected '0' expression to evaluate to false")
	}
}

func TestTodoEvaluatePrecondition(t *testing.T) {
	// Test with nil precondition
	todo := Todo{}
	shouldProceed, reason := todo.EvaluatePrecondition("/tmp")
	if !shouldProceed {
		t.Error("Expected precondition to pass with nil config")
	}
	if reason != "" {
		t.Errorf("Expected empty reason, got %s", reason)
	}
}