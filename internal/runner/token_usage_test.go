package runner

import (
	"testing"
)

func TestParseTokenUsage(t *testing.T) {
	testCases := []struct {
		name     string
		stderr   string
		expected TokenUsage
	}{
		{
			name: "Basic format",
			stderr: `Some other output
Total input tokens: 12345
Total output tokens: 6789
More output`,
			expected: TokenUsage{InputTokens: 12345, OutputTokens: 6789},
		},
		{
			name: "Alternative format",
			stderr: `Processing task...
Input tokens: 54321
Output tokens: 9876
Task completed`,
			expected: TokenUsage{InputTokens: 54321, OutputTokens: 9876},
		},
		{
			name: "With commas",
			stderr: `Model response received
Total input tokens: 1,234,567
Total output tokens: 987,654
Done`,
			expected: TokenUsage{InputTokens: 1234567, OutputTokens: 987654},
		},
		{
			name:     "No tokens",
			stderr:   "Just some regular output",
			expected: TokenUsage{InputTokens: 0, OutputTokens: 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseTokenUsage(tc.stderr)
			if result != tc.expected {
				t.Errorf("Expected %+v, got %+v", tc.expected, result)
			}
		})
	}
}