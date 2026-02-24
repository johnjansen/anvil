package runner

import (
	"testing"
)

func TestParseTokenUsage_StandardFormat(t *testing.T) {
	stderr := `Some startup output
Total input tokens: 12345
Total output tokens: 6789
Done.`
	usage := ParseTokenUsage(stderr)
	if usage.InputTokens != 12345 {
		t.Errorf("expected InputTokens=12345, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 6789 {
		t.Errorf("expected OutputTokens=6789, got %d", usage.OutputTokens)
	}
}

func TestParseTokenUsage_WithCommas(t *testing.T) {
	stderr := `Input tokens: 1,234,567
Output tokens: 89,012`
	usage := ParseTokenUsage(stderr)
	if usage.InputTokens != 1234567 {
		t.Errorf("expected InputTokens=1234567, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 89012 {
		t.Errorf("expected OutputTokens=89012, got %d", usage.OutputTokens)
	}
}

func TestParseTokenUsage_NoTokenInfo(t *testing.T) {
	stderr := `Some random error output
No token info here`
	usage := ParseTokenUsage(stderr)
	if usage.InputTokens != 0 {
		t.Errorf("expected InputTokens=0, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 0 {
		t.Errorf("expected OutputTokens=0, got %d", usage.OutputTokens)
	}
}

func TestParseTokenUsage_EmptyStderr(t *testing.T) {
	usage := ParseTokenUsage("")
	if usage.InputTokens != 0 || usage.OutputTokens != 0 {
		t.Errorf("expected zero values for empty stderr, got %+v", usage)
	}
}

func TestParseTokenUsage_CaseInsensitive(t *testing.T) {
	stderr := `TOTAL INPUT TOKENS: 500
TOTAL OUTPUT TOKENS: 200`
	usage := ParseTokenUsage(stderr)
	if usage.InputTokens != 500 {
		t.Errorf("expected InputTokens=500, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 200 {
		t.Errorf("expected OutputTokens=200, got %d", usage.OutputTokens)
	}
}

func TestParseTokenUsage_MixedContent(t *testing.T) {
	stderr := `Starting task...
⠋ Running...
input tokens: 42000
some other line
output tokens: 8500
Finished.`
	usage := ParseTokenUsage(stderr)
	if usage.InputTokens != 42000 {
		t.Errorf("expected InputTokens=42000, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 8500 {
		t.Errorf("expected OutputTokens=8500, got %d", usage.OutputTokens)
	}
}

func TestExtractTokenCount_Direction(t *testing.T) {
	// Should not match "output" when looking for "input"
	n, ok := extractTokenCount("output tokens: 500", "input")
	if ok {
		t.Errorf("should not match output line for input direction, got %d", n)
	}

	n, ok = extractTokenCount("input tokens: 500", "input")
	if !ok || n != 500 {
		t.Errorf("expected 500, got %d (ok=%v)", n, ok)
	}
}
