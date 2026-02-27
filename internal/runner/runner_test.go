package runner

import (
	"bytes"
	"sync"
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

func TestStatusWriter_DetectsStatusOnStdout(t *testing.T) {
	var downstream bytes.Buffer
	var mu sync.Mutex
	var captured string

	sw := newStatusWriter(&downstream, func(status string) {
		mu.Lock()
		captured = status
		mu.Unlock()
	}, nil, nil)

	sw.Write([]byte("normal output\n##anvil:status working on tests\nmore output\n"))
	sw.Flush()

	mu.Lock()
	defer mu.Unlock()
	if captured != "working on tests" {
		t.Errorf("expected status 'working on tests', got %q", captured)
	}

	// Status lines should be stripped from downstream
	if bytes.Contains(downstream.Bytes(), []byte("##anvil:status")) {
		t.Error("status line should be stripped from downstream output")
	}

	// Non-status lines should pass through
	if !bytes.Contains(downstream.Bytes(), []byte("normal output")) {
		t.Error("normal output should pass through")
	}
	if !bytes.Contains(downstream.Bytes(), []byte("more output")) {
		t.Error("more output should pass through")
	}
}

func TestStatusWriter_MultipleStatusUpdates(t *testing.T) {
	var downstream bytes.Buffer
	var mu sync.Mutex
	var statuses []string

	sw := newStatusWriter(&downstream, func(status string) {
		mu.Lock()
		statuses = append(statuses, status)
		mu.Unlock()
	}, nil, nil)

	sw.Write([]byte("##anvil:status step 1\n"))
	sw.Write([]byte("##anvil:status step 2\n"))
	sw.Write([]byte("##anvil:status step 3\n"))

	mu.Lock()
	defer mu.Unlock()
	if len(statuses) != 3 {
		t.Fatalf("expected 3 status updates, got %d", len(statuses))
	}
	if statuses[2] != "step 3" {
		t.Errorf("expected last status 'step 3', got %q", statuses[2])
	}
}

func TestStatusWriter_PartialLineBuffering(t *testing.T) {
	var downstream bytes.Buffer
	var mu sync.Mutex
	var captured string

	sw := newStatusWriter(&downstream, func(status string) {
		mu.Lock()
		captured = status
		mu.Unlock()
	}, nil, nil)

	// Write status prefix split across two Write calls
	sw.Write([]byte("##anvil:sta"))
	sw.Write([]byte("tus chunked message\n"))

	mu.Lock()
	defer mu.Unlock()
	if captured != "chunked message" {
		t.Errorf("expected status 'chunked message', got %q", captured)
	}
}

func TestStatusWriter_FlushPartialLine(t *testing.T) {
	var downstream bytes.Buffer
	var mu sync.Mutex
	var captured string

	sw := newStatusWriter(&downstream, func(status string) {
		mu.Lock()
		captured = status
		mu.Unlock()
	}, nil, nil)

	// Write without trailing newline, then flush
	sw.Write([]byte("##anvil:status final status"))
	sw.Flush()

	mu.Lock()
	defer mu.Unlock()
	if captured != "final status" {
		t.Errorf("expected status 'final status', got %q", captured)
	}
}

func TestStatusWriter_EmptyStatusIgnored(t *testing.T) {
	var downstream bytes.Buffer
	callCount := 0

	sw := newStatusWriter(&downstream, func(status string) {
		callCount++
	}, nil, nil)

	sw.Write([]byte("##anvil:status \n"))

	if callCount != 0 {
		t.Errorf("expected 0 callbacks for empty status, got %d", callCount)
	}
}
