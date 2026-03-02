package runner

import (
	"strings"
	"testing"

	"github.com/johnjansen/anvil/internal/project"
)

func TestEvaluateStdoutContainsAssertion(t *testing.T) {
	tests := []struct {
		name        string
		config      *project.AssertConfig
		stdout      string
		stderr      string
		expectFail  bool
		expectError bool
		expectMessageContains string
	}{
		{
			name: "stdout contains match",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					Contains: "Success",
				},
			},
			stdout:     "Operation completed. Success.",
			stderr:     "",
			expectFail: false,
		},
		{
			name: "stdout contains no match",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					Contains: "Success:",
				},
			},
			stdout:     "Operation failed.",
			stderr:     "",
			expectFail: true,
			expectMessageContains: "Assertion failed: stdout does not contain 'Success:'",
		},
		{
			name: "stdout contains multiple matches",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					Contains: "test",
				},
			},
			stdout:     "This is a test string with test content",
			stderr:     "",
			expectFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := &project.Todo{
				Assert: tt.config,
			}

			results, err := EvaluateAssertions(todo, tt.stdout, tt.stderr)
			if err != nil && !tt.expectError {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if err == nil && tt.expectError {
				t.Error("expected error but got none")
				return
			}

			if results.Failed != tt.expectFail {
				t.Errorf("expected failed=%v, got failed=%v", tt.expectFail, results.Failed)
				for _, result := range results.Results {
					t.Logf("Result: passed=%v, message=%s, error=%v", result.Passed, result.Message, result.Error)
				}
			}

			// Check error message format if specified
			if tt.expectMessageContains != "" {
				foundExpectedMessage := false
				for _, result := range results.Results {
					if !result.Passed && result.Message != "" && strings.Contains(result.Message, tt.expectMessageContains) {
						foundExpectedMessage = true
						break
					}
				}
				if !foundExpectedMessage {
					t.Errorf("expected message containing '%s', but didn't find it in results", tt.expectMessageContains)
					for _, result := range results.Results {
						t.Logf("Result: passed=%v, message=%s, error=%v", result.Passed, result.Message, result.Error)
					}
				}
			}
		})
	}
}

func TestEvaluateStderrEmptyAssertion(t *testing.T) {
	tests := []struct {
		name        string
		config      *project.AssertConfig
		stdout      string
		stderr      string
		expectFail  bool
		expectError bool
	}{
		{
			name: "stderr empty success",
			config: &project.AssertConfig{
				Stderr: &project.StderrAssertion{
					Empty: true,
				},
			},
			stdout:     "output",
			stderr:     "",
			expectFail: false,
		},
		{
			name: "stderr empty failure",
			config: &project.AssertConfig{
				Stderr: &project.StderrAssertion{
					Empty: true,
				},
			},
			stdout:     "output",
			stderr:     "error message",
			expectFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := &project.Todo{
				Assert: tt.config,
			}

			results, err := EvaluateAssertions(todo, tt.stdout, tt.stderr)
			if err != nil && !tt.expectError {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if err == nil && tt.expectError {
				t.Error("expected error but got none")
				return
			}

			if results.Failed != tt.expectFail {
				t.Errorf("expected failed=%v, got failed=%v", tt.expectFail, results.Failed)
				for _, result := range results.Results {
					t.Logf("Result: passed=%v, message=%s, error=%v", result.Passed, result.Message, result.Error)
				}
			}
		})
	}
}

func TestEvaluateStdoutMatchesAssertion(t *testing.T) {
	tests := []struct {
		name        string
		config      *project.AssertConfig
		stdout      string
		stderr      string
		expectFail  bool
		expectError bool
		expectMessageContains string
	}{
		{
			name: "stdout matches regex success",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					Matches: `Processed \d+ records`,
				},
			},
			stdout:     "Processed 42 records successfully",
			stderr:     "",
			expectFail: false,
		},
		{
			name: "stdout matches regex failure",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					Matches: `Processed \d+ records`,
				},
			},
			stdout:     "Processing failed",
			stderr:     "",
			expectFail: true,
			expectMessageContains: "Assertion failed: stdout does not match pattern",
		},
		{
			name: "stdout matches invalid regex error",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					Matches: `[invalid regex`,
				},
			},
			stdout:     "any content",
			stderr:     "",
			expectFail: true,
			expectError: false, // The evaluation itself doesn't return an error, but the result has an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := &project.Todo{
				Assert: tt.config,
			}

			results, err := EvaluateAssertions(todo, tt.stdout, tt.stderr)
			if err != nil && !tt.expectError {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if err == nil && tt.expectError {
				t.Error("expected error but got none")
				return
			}

			if results.Failed != tt.expectFail {
				t.Errorf("expected failed=%v, got failed=%v", tt.expectFail, results.Failed)
				for _, result := range results.Results {
					t.Logf("Result: passed=%v, message=%s, error=%v", result.Passed, result.Message, result.Error)
				}
			}

			// Check error message format if specified
			if tt.expectMessageContains != "" {
				foundExpectedMessage := false
				for _, result := range results.Results {
					if (!result.Passed || result.Error != nil) && result.Message != "" && strings.Contains(result.Message, tt.expectMessageContains) {
						foundExpectedMessage = true
						break
					}
				}
				if !foundExpectedMessage {
					t.Errorf("expected message containing '%s', but didn't find it in results", tt.expectMessageContains)
					for _, result := range results.Results {
						t.Logf("Result: passed=%v, message=%s, error=%v", result.Passed, result.Message, result.Error)
					}
				}
			}
		})
	}
}

func TestEvaluateStdoutJSONValidAssertion(t *testing.T) {
	tests := []struct {
		name        string
		config      *project.AssertConfig
		stdout      string
		stderr      string
		expectFail  bool
		expectError bool
		expectMessageContains string
	}{
		{
			name: "stdout json valid success",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					JSONValid: true,
				},
			},
			stdout:     `{"result": "success", "count": 42}`,
			stderr:     "",
			expectFail: false,
		},
		{
			name: "stdout json valid failure",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					JSONValid: true,
				},
			},
			stdout:     `{"result": "success", "count": 42`,
			stderr:     "",
			expectFail: true,
			expectMessageContains: "Assertion failed: stdout is not valid JSON",
		},
		{
			name: "stdout json valid empty string",
			config: &project.AssertConfig{
				Stdout: &project.StdoutAssertion{
					JSONValid: true,
				},
			},
			stdout:     "",
			stderr:     "",
			expectFail: true,
			expectMessageContains: "Assertion failed: stdout is not valid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := &project.Todo{
				Assert: tt.config,
			}

			results, err := EvaluateAssertions(todo, tt.stdout, tt.stderr)
			if err != nil && !tt.expectError {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if err == nil && tt.expectError {
				t.Error("expected error but got none")
				return
			}

			if results.Failed != tt.expectFail {
				t.Errorf("expected failed=%v, got failed=%v", tt.expectFail, results.Failed)
				for _, result := range results.Results {
					t.Logf("Result: passed=%v, message=%s, error=%v", result.Passed, result.Message, result.Error)
				}
			}

			// Check error message format if specified
			if tt.expectMessageContains != "" {
				foundExpectedMessage := false
				for _, result := range results.Results {
					if !result.Passed && result.Message != "" && strings.Contains(result.Message, tt.expectMessageContains) {
						foundExpectedMessage = true
						break
					}
				}
				if !foundExpectedMessage {
					t.Errorf("expected message containing '%s', but didn't find it in results", tt.expectMessageContains)
					for _, result := range results.Results {
						t.Logf("Result: passed=%v, message=%s, error=%v", result.Passed, result.Message, result.Error)
					}
				}
			}
		})
	}
}

func TestEvaluateSoftAssertions(t *testing.T) {
	tests := []struct {
		name        string
		config      *project.AssertConfig
		stdout      string
		stderr      string
		expectFail  bool
		expectError bool
	}{
		{
			name: "soft assertion failure doesn't fail task",
			config: &project.AssertConfig{
				Soft: true,
				Stdout: &project.StdoutAssertion{
					Contains: "Success:",
				},
			},
			stdout:     "Operation failed.",
			stderr:     "",
			expectFail: false, // Soft assertions don't cause task failure
		},
		{
			name: "soft assertion success",
			config: &project.AssertConfig{
				Soft: true,
				Stdout: &project.StdoutAssertion{
					Contains: "Success:",
				},
			},
			stdout:     "Operation completed. Success.",
			stderr:     "",
			expectFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := &project.Todo{
				Assert: tt.config,
			}

			results, err := EvaluateAssertions(todo, tt.stdout, tt.stderr)
			if err != nil && !tt.expectError {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if err == nil && tt.expectError {
				t.Error("expected error but got none")
				return
			}

			// For soft assertions, the overall result should not fail even if individual assertions fail
			if results.Failed != tt.expectFail {
				t.Errorf("expected failed=%v, got failed=%v", tt.expectFail, results.Failed)
				for _, result := range results.Results {
					t.Logf("Result: passed=%v, message=%s, error=%v", result.Passed, result.Message, result.Error)
				}
			}
		})
	}
}