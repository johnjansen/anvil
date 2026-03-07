package runner

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/johnjansen/anvil/internal/project"
)

func TestEvaluateFileAssertions(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := ioutil.TempDir("", "assertion_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := "This is test content for assertion testing"
	err = ioutil.WriteFile(testFilePath, []byte(testContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		config      *project.AssertConfig
		projectPath string
		expectFail  bool
		expectError bool
	}{
		{
			name: "file exists success",
			config: &project.AssertConfig{
				Files: []project.FileAssertion{
					{
						Path:   "test.txt",
						Exists: boolPtr(true),
					},
				},
			},
			projectPath: tempDir,
			expectFail:  false,
		},
		{
			name: "file exists failure",
			config: &project.AssertConfig{
				Files: []project.FileAssertion{
					{
						Path:   "nonexistent.txt",
						Exists: boolPtr(true),
					},
				},
			},
			projectPath: tempDir,
			expectFail:  true,
		},
		{
			name: "file contains success",
			config: &project.AssertConfig{
				Files: []project.FileAssertion{
					{
						Path:     "test.txt",
						Exists:   boolPtr(true),
						Contains: "test content",
					},
				},
			},
			projectPath: tempDir,
			expectFail:  false,
		},
		{
			name: "file contains failure",
			config: &project.AssertConfig{
				Files: []project.FileAssertion{
					{
						Path:     "test.txt",
						Exists:   boolPtr(true),
						Contains: "nonexistent content",
					},
				},
			},
			projectPath: tempDir,
			expectFail:  true,
		},
		{
			name: "file size min success",
			config: &project.AssertConfig{
				Files: []project.FileAssertion{
					{
						Path:    "test.txt",
						Exists:  boolPtr(true),
						SizeMin: 10,
					},
				},
			},
			projectPath: tempDir,
			expectFail:  false,
		},
		{
			name: "file size min failure",
			config: &project.AssertConfig{
				Files: []project.FileAssertion{
					{
						Path:    "test.txt",
						Exists:  boolPtr(true),
						SizeMin: 1000,
					},
				},
			},
			projectPath: tempDir,
			expectFail:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := &project.Todo{
				Assert: tt.config,
				Path:   tt.projectPath,
			}

			results, err := EvaluateAssertions(todo, "", "")
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

func boolPtr(b bool) *bool {
	return &b
}