package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncateLog_SmallFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	content := "line1\nline2\nline3\n"
	os.WriteFile(logPath, []byte(content), 0644)

	// File is 18 bytes, maxSize is 1024 — should not truncate
	truncateLog(logPath, 1024)

	data, _ := os.ReadFile(logPath)
	if string(data) != content {
		t.Errorf("expected file unchanged, got %q", string(data))
	}
}

func TestTruncateLog_OversizedFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Create a 10KB file
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, strings.Repeat("x", 50))
	}
	content := strings.Join(lines, "\n") + "\n"
	os.WriteFile(logPath, []byte(content), 0644)

	// Truncate to 1KB
	truncateLog(logPath, 1024)

	data, _ := os.ReadFile(logPath)
	if len(data) > 1024 {
		t.Errorf("expected file <= 1024 bytes, got %d", len(data))
	}
	if !strings.HasPrefix(string(data), "\n--- [log truncated: exceeded max_log_size] ---\n") {
		t.Errorf("expected truncation marker, got prefix: %q", string(data[:80]))
	}
}

func TestTruncateLog_PreservesRecentOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	// Create content with identifiable last line
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, strings.Repeat("a", 50))
	}
	lines = append(lines, "LAST_LINE_MARKER")
	content := strings.Join(lines, "\n") + "\n"
	os.WriteFile(logPath, []byte(content), 0644)

	truncateLog(logPath, 2048)

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "LAST_LINE_MARKER") {
		t.Error("expected last line to be preserved after truncation")
	}
}

func TestTruncateLog_NonExistentFile(t *testing.T) {
	// Should not panic on missing file
	truncateLog("/nonexistent/path/test.log", 1024)
}

func TestTruncateLog_ZeroMaxSize(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	content := "some content\n"
	os.WriteFile(logPath, []byte(content), 0644)

	// maxSize 0 should be handled by caller (not calling truncateLog)
	// but if called directly, it should still work — file is always > 0
	truncateLog(logPath, 0)

	data, _ := os.ReadFile(logPath)
	// With maxSize=0, file size > 0, so truncation triggers
	if !strings.Contains(string(data), "log truncated") {
		t.Error("expected truncation with maxSize=0")
	}
}

func TestTruncateLog_ExactMaxSize(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")
	content := "exactly this content\n"
	os.WriteFile(logPath, []byte(content), 0644)

	info, _ := os.Stat(logPath)
	// File is exactly maxSize — should not truncate
	truncateLog(logPath, info.Size())

	data, _ := os.ReadFile(logPath)
	if string(data) != content {
		t.Errorf("expected file unchanged at exact maxSize, got %q", string(data))
	}
}
