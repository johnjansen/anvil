package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

// AssertionResult represents the result of evaluating a single assertion.
type AssertionResult struct {
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
	Error   error  `json:"error,omitempty"`
}

// AssertionResults represents the results of evaluating all assertions for a task.
type AssertionResults struct {
	Results []AssertionResult `json:"results"`
	Failed  bool              `json:"failed"` // true if any hard assertion failed
}

// EvaluateAssertions evaluates all configured assertions for a task.
func EvaluateAssertions(todo *project.Todo, stdout, stderr string) (*AssertionResults, error) {
	results := &AssertionResults{
		Results: make([]AssertionResult, 0),
		Failed:  false,
	}

	// If no assertions are configured, return early
	if todo.Assert == nil {
		return results, nil
	}

	// Evaluate stdout assertions
	if todo.Assert.Stdout != nil {
		stdoutResults := evaluateStdoutAssertions(todo.Assert.Stdout, stdout)
		results.Results = append(results.Results, stdoutResults...)
	}

	// Evaluate stderr assertions
	if todo.Assert.Stderr != nil {
		stderrResults := evaluateStderrAssertions(todo.Assert.Stderr, stderr)
		results.Results = append(results.Results, stderrResults...)
	}

	// Evaluate file assertions
	if len(todo.Assert.Files) > 0 {
		fileResults := evaluateFileAssertions(todo.Assert.Files, todo.Path)
		results.Results = append(results.Results, fileResults...)
	}

	// Check if any hard assertions failed (when soft assertions are disabled)
	if !todo.Assert.Soft {
		for _, result := range results.Results {
			if !result.Passed {
				results.Failed = true
				break
			}
		}
	}

	return results, nil
}

// evaluateStdoutAssertions evaluates stdout content assertions.
func evaluateStdoutAssertions(config *project.StdoutAssertion, stdout string) []AssertionResult {
	var results []AssertionResult

	// Check contains assertion
	if config.Contains != "" {
		result := AssertionResult{
			Passed: strings.Contains(stdout, config.Contains),
		}
		if !result.Passed {
			result.Message = fmt.Sprintf("Assertion failed: stdout does not contain '%s'", config.Contains)
		}
		results = append(results, result)
	}

	// Check matches assertion
	if config.Matches != "" {
		re, err := regexp.Compile(config.Matches)
		if err != nil {
			results = append(results, AssertionResult{
				Passed: false,
				Error:  err,
				Message: fmt.Sprintf("Assertion failed: invalid regex pattern '%s' for stdout matches assertion: %v",
					config.Matches, err),
			})
		} else {
			result := AssertionResult{
				Passed: re.MatchString(stdout),
			}
			if !result.Passed {
				result.Message = fmt.Sprintf("Assertion failed: stdout does not match pattern '%s'", config.Matches)
			}
			results = append(results, result)
		}
	}

	// Check JSON validity assertion
	if config.JSONValid {
		result := AssertionResult{
			Passed: json.Valid([]byte(stdout)),
		}
		if !result.Passed {
			result.Message = "Assertion failed: stdout is not valid JSON"
		}
		results = append(results, result)
	}

	return results
}

// evaluateStderrAssertions evaluates stderr content assertions.
func evaluateStderrAssertions(config *project.StderrAssertion, stderr string) []AssertionResult {
	var results []AssertionResult

	// Check empty assertion
	if config.Empty {
		result := AssertionResult{
			Passed: len(stderr) == 0,
		}
		if !result.Passed {
			result.Message = "Assertion failed: stderr is not empty"
		}
		results = append(results, result)
	}

	return results
}

// evaluateFileAssertions evaluates file content assertions.
func evaluateFileAssertions(files []project.FileAssertion, projectPath string) []AssertionResult {
	var results []AssertionResult

	for _, file := range files {
		// Resolve file path relative to project root
		filePath := file.Path
		if !strings.HasPrefix(filePath, "/") {
			filePath = fmt.Sprintf("%s/%s", projectPath, filePath)
		}

		// Check exists assertion
		if file.Exists != nil {
			_, err := os.Stat(filePath)
			exists := err == nil

			result := AssertionResult{
				Passed: exists == *file.Exists,
			}
			if !result.Passed {
				if *file.Exists {
					result.Message = fmt.Sprintf("Assertion failed: file '%s' does not exist", file.Path)
				} else {
					result.Message = fmt.Sprintf("Assertion failed: file '%s' exists but should not", file.Path)
				}
			}
			results = append(results, result)
		}

		// If file doesn't exist and we're checking content/size, that's an automatic failure
		if file.Exists != nil && !*file.Exists {
			continue
		}

		// Read file content for further assertions
		content, err := os.ReadFile(filePath)
		if err != nil && (file.Contains != "" || file.SizeMin > 0 || file.SizeMax > 0) {
			results = append(results, AssertionResult{
				Passed:  false,
				Error:   err,
				Message: fmt.Sprintf("Assertion failed: could not read file '%s': %v", file.Path, err),
			})
			continue
		}

		// Check contains assertion
		if file.Contains != "" {
			result := AssertionResult{
				Passed: strings.Contains(string(content), file.Contains),
			}
			if !result.Passed {
				result.Message = fmt.Sprintf("Assertion failed: file '%s' does not contain '%s'", file.Path, file.Contains)
			}
			results = append(results, result)
		}

		// Check size assertions
		fileInfo, err := os.Stat(filePath)
		if err != nil && (file.SizeMin > 0 || file.SizeMax > 0) {
			results = append(results, AssertionResult{
				Passed:  false,
				Error:   err,
				Message: fmt.Sprintf("Assertion failed: could not stat file '%s': %v", file.Path, err),
			})
			continue
		}

		if file.SizeMin > 0 {
			result := AssertionResult{
				Passed: fileInfo.Size() >= file.SizeMin,
			}
			if !result.Passed {
				result.Message = fmt.Sprintf("Assertion failed: file '%s' size %d bytes is less than minimum %d bytes",
					file.Path, fileInfo.Size(), file.SizeMin)
			}
			results = append(results, result)
		}

		if file.SizeMax > 0 {
			result := AssertionResult{
				Passed: fileInfo.Size() <= file.SizeMax,
			}
			if !result.Passed {
				result.Message = fmt.Sprintf("Assertion failed: file '%s' size %d bytes is greater than maximum %d bytes",
					file.Path, fileInfo.Size(), file.SizeMax)
			}
			results = append(results, result)
		}
	}

	return results
}