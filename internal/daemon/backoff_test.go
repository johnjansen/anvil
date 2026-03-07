package daemon

import (
	"testing"
	"time"
)

func TestCalculateBackoff_Exponential(t *testing.T) {
	base := time.Minute
	// attempt 0: 1m, attempt 1: 2m, attempt 2: 4m, attempt 3: 8m
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Minute},
		{1, 2 * time.Minute},
		{2, 4 * time.Minute},
		{3, 8 * time.Minute},
	}
	for _, tt := range tests {
		got := calculateBackoff("exponential", base, tt.attempt, 0)
		if got != tt.expected {
			t.Errorf("exponential attempt %d: got %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestCalculateBackoff_Linear(t *testing.T) {
	base := time.Minute
	// attempt 0: 1m, attempt 1: 2m, attempt 2: 3m, attempt 3: 4m
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Minute},
		{1, 2 * time.Minute},
		{2, 3 * time.Minute},
		{3, 4 * time.Minute},
	}
	for _, tt := range tests {
		got := calculateBackoff("linear", base, tt.attempt, 0)
		if got != tt.expected {
			t.Errorf("linear attempt %d: got %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestCalculateBackoff_Constant(t *testing.T) {
	base := time.Minute
	for attempt := 0; attempt < 5; attempt++ {
		got := calculateBackoff("constant", base, attempt, 0)
		if got != base {
			t.Errorf("constant attempt %d: got %v, want %v", attempt, got, base)
		}
	}
}

func TestCalculateBackoff_DefaultsToExponential(t *testing.T) {
	base := time.Minute
	// Empty strategy should default to exponential
	got := calculateBackoff("", base, 2, 0)
	expected := 4 * time.Minute
	if got != expected {
		t.Errorf("default strategy attempt 2: got %v, want %v", got, expected)
	}
}

func TestCalculateBackoff_OneHourCap(t *testing.T) {
	base := time.Minute
	// attempt 10 exponential would be 1024 minutes, should cap at 1 hour
	got := calculateBackoff("exponential", base, 10, 0)
	if got != time.Hour {
		t.Errorf("cap: got %v, want %v", got, time.Hour)
	}
}

func TestCalculateBackoff_Jitter(t *testing.T) {
	base := time.Minute
	jitter := 0.5

	// Run multiple times and verify range [30s, 90s]
	minSeen := time.Hour
	maxSeen := time.Duration(0)
	for i := 0; i < 100; i++ {
		got := calculateBackoff("constant", base, 0, jitter)
		if got < minSeen {
			minSeen = got
		}
		if got > maxSeen {
			maxSeen = got
		}
		// Must be within [30s, 90s]
		lower := time.Duration(float64(base) * (1 - jitter))
		upper := time.Duration(float64(base) * (1 + jitter))
		if got < lower || got > upper {
			t.Errorf("jitter out of range: got %v, want [%v, %v]", got, lower, upper)
		}
	}
	// Verify we saw some spread (not all identical)
	if minSeen == maxSeen {
		t.Error("jitter produced no variation across 100 samples")
	}
}

func TestCalculateBackoff_ZeroJitter(t *testing.T) {
	base := time.Minute
	// With jitter=0, should be deterministic
	got1 := calculateBackoff("constant", base, 0, 0)
	got2 := calculateBackoff("constant", base, 0, 0)
	if got1 != got2 {
		t.Errorf("zero jitter not deterministic: %v vs %v", got1, got2)
	}
	if got1 != base {
		t.Errorf("zero jitter: got %v, want %v", got1, base)
	}
}

func TestCalculateBackoff_FloorOneSecond(t *testing.T) {
	// Very small base with high jitter could produce near-zero
	base := time.Second
	got := calculateBackoff("constant", base, 0, 0)
	if got < time.Second {
		t.Errorf("floor: got %v, want >= 1s", got)
	}
}

func TestCalculateBackoff_ZeroBaseDefault(t *testing.T) {
	// Zero base should default to 1 minute
	got := calculateBackoff("constant", 0, 0, 0)
	if got != time.Minute {
		t.Errorf("zero base: got %v, want 1m", got)
	}
}
