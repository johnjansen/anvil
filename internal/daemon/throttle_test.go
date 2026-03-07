package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseRate(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"1/m", 1, false},
		{"5/m", 5, false},
		{"10/m", 10, false},
		{"off", 0, false},
		{"0", 0, false},
		{"", 0, false},
		{"1/h", 0, true},       // unsupported unit
		{"abc/m", 0, true},     // bad number
		{"-1/m", 0, true},      // negative
		{"1", 0, true},         // no unit
		{"1/m/extra", 0, true}, // too many slashes
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseRate(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestThrottleManagerPauseResume(t *testing.T) {
	// Use a temp dir for throttle state
	tmpDir := t.TempDir()
	origDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origDir)

	// Create .anvil directory
	os.MkdirAll(filepath.Join(tmpDir, ".anvil"), 0755)

	tm := newThrottleManager()

	// Initially not paused
	if tm.IsPaused() {
		t.Error("expected not paused initially")
	}

	// Pause globally
	if err := tm.SetPaused(true); err != nil {
		t.Fatalf("SetPaused(true): %v", err)
	}
	if !tm.IsPaused() {
		t.Error("expected paused after SetPaused(true)")
	}

	// Resume
	if err := tm.SetPaused(false); err != nil {
		t.Fatalf("SetPaused(false): %v", err)
	}
	if tm.IsPaused() {
		t.Error("expected not paused after SetPaused(false)")
	}
}

func TestThrottleManagerLabels(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origDir)
	os.MkdirAll(filepath.Join(tmpDir, ".anvil"), 0755)

	tm := newThrottleManager()

	// No labels paused initially
	if tm.IsLabelPaused([]string{"batch"}) {
		t.Error("expected label 'batch' not paused initially")
	}

	// Pause a label
	if err := tm.PauseLabel("batch"); err != nil {
		t.Fatalf("PauseLabel: %v", err)
	}
	if !tm.IsLabelPaused([]string{"batch"}) {
		t.Error("expected label 'batch' paused")
	}
	if tm.IsLabelPaused([]string{"monitoring"}) {
		t.Error("expected label 'monitoring' not paused")
	}

	// Task with multiple labels, one paused
	if !tm.IsLabelPaused([]string{"monitoring", "batch"}) {
		t.Error("expected paused when task has a paused label")
	}

	// Resume label
	if err := tm.ResumeLabel("batch"); err != nil {
		t.Fatalf("ResumeLabel: %v", err)
	}
	if tm.IsLabelPaused([]string{"batch"}) {
		t.Error("expected label 'batch' not paused after resume")
	}
}

func TestThrottleManagerRate(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origDir)
	os.MkdirAll(filepath.Join(tmpDir, ".anvil"), 0755)

	tm := newThrottleManager()

	// No rate limit by default
	now := time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	if !tm.AllowDispatch(now) {
		t.Error("expected dispatch allowed with no rate limit")
	}

	// Set rate to 2/m
	if err := tm.SetRate(2); err != nil {
		t.Fatalf("SetRate: %v", err)
	}

	// First two dispatches should be allowed
	if !tm.AllowDispatch(now) {
		t.Error("expected 1st dispatch allowed")
	}
	if !tm.AllowDispatch(now) {
		t.Error("expected 2nd dispatch allowed")
	}
	// Third should be denied
	if tm.AllowDispatch(now) {
		t.Error("expected 3rd dispatch denied at rate 2/m")
	}

	// Next minute should reset
	nextMinute := now.Add(time.Minute)
	if !tm.AllowDispatch(nextMinute) {
		t.Error("expected dispatch allowed in new minute")
	}
}

func TestThrottleManagerPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origDir)
	os.MkdirAll(filepath.Join(tmpDir, ".anvil"), 0755)

	// Set state with first manager
	tm1 := newThrottleManager()
	tm1.SetPaused(true)
	tm1.SetRate(5)
	tm1.PauseLabel("batch")

	// Load with second manager
	tm2 := newThrottleManager()
	if err := tm2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	if !tm2.IsPaused() {
		t.Error("expected paused state to persist")
	}
	state := tm2.GetState()
	if state.RatePerMin != 5 {
		t.Errorf("expected rate 5, got %d", state.RatePerMin)
	}
	if !state.PausedLabels["batch"] {
		t.Error("expected label 'batch' to persist as paused")
	}
}

func TestThrottleManagerEmptyLabels(t *testing.T) {
	tm := newThrottleManager()
	// Tasks with no labels should never be affected by label pause
	tm.state.PausedLabels["batch"] = true
	if tm.IsLabelPaused(nil) {
		t.Error("nil labels should not be paused")
	}
	if tm.IsLabelPaused([]string{}) {
		t.Error("empty labels should not be paused")
	}
}
