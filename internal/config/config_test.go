package config

import (
	"testing"
	"time"
)

func TestParseRetentionAge(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"", 0, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"2h30m", 2*time.Hour + 30*time.Minute, false},
		{"bad", 0, true},
		{"xd", 0, true},
	}

	for _, tt := range tests {
		d, err := ParseRetentionAge(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("ParseRetentionAge(%q) expected error, got nil", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ParseRetentionAge(%q) unexpected error: %v", tt.input, err)
		}
		if d != tt.expected {
			t.Errorf("ParseRetentionAge(%q) = %v, want %v", tt.input, d, tt.expected)
		}
	}
}
