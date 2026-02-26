package daemon

import (
	"testing"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func TestGetEffectiveSLA(t *testing.T) {
	tests := []struct {
		name           string
		todo           project.Todo
		globalCfg      config.SLAGlobalConfig
		wantMaxDelay   time.Duration
		wantStrict     bool
	}{
		{
			name:         "per-task SLA only",
			todo:         project.Todo{SLA: project.SLAConfig{MaxDelay: 15 * time.Minute, Strict: true}},
			globalCfg:    config.SLAGlobalConfig{},
			wantMaxDelay: 15 * time.Minute,
			wantStrict:   true,
		},
		{
			name:         "global SLA only",
			todo:         project.Todo{},
			globalCfg:    config.SLAGlobalConfig{DefaultMaxDelay: "30m"},
			wantMaxDelay: 30 * time.Minute,
			wantStrict:   false,
		},
		{
			name:         "per-task overrides global",
			todo:         project.Todo{SLA: project.SLAConfig{MaxDelay: 10 * time.Minute, Strict: false}},
			globalCfg:    config.SLAGlobalConfig{DefaultMaxDelay: "30m"},
			wantMaxDelay: 10 * time.Minute,
			wantStrict:   false,
		},
		{
			name:         "neither configured",
			todo:         project.Todo{},
			globalCfg:    config.SLAGlobalConfig{},
			wantMaxDelay: 0,
			wantStrict:   false,
		},
		{
			name:         "global with invalid duration",
			todo:         project.Todo{},
			globalCfg:    config.SLAGlobalConfig{DefaultMaxDelay: "invalid"},
			wantMaxDelay: 0,
			wantStrict:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxDelay, strict := getEffectiveSLA(tt.todo, tt.globalCfg)
			if maxDelay != tt.wantMaxDelay {
				t.Errorf("maxDelay = %v, want %v", maxDelay, tt.wantMaxDelay)
			}
			if strict != tt.wantStrict {
				t.Errorf("strict = %v, want %v", strict, tt.wantStrict)
			}
		})
	}
}

func TestCheckSLA(t *testing.T) {
	// A schedule that matches every minute: "* * * * *"
	everyMinute := "* * * * *"

	tests := []struct {
		name          string
		todo          project.Todo
		globalCfg     config.SLAGlobalConfig
		now           time.Time
		wantHasSLA    bool
		wantViolation bool
	}{
		{
			name: "on time — no violation",
			todo: project.Todo{
				Schedule: everyMinute,
				SLA:      project.SLAConfig{MaxDelay: 5 * time.Minute},
			},
			// 30 seconds after the minute boundary: delay ~30s, threshold 5m
			now:           time.Date(2026, 2, 27, 9, 0, 30, 0, time.UTC),
			wantHasSLA:    true,
			wantViolation: false,
		},
		{
			name: "within threshold — no violation",
			todo: project.Todo{
				Schedule: "0 9 * * *", // every day at 09:00
				SLA:      project.SLAConfig{MaxDelay: 15 * time.Minute},
			},
			// 10 minutes after scheduled time
			now:           time.Date(2026, 2, 27, 9, 10, 0, 0, time.UTC),
			wantHasSLA:    true,
			wantViolation: false,
		},
		{
			name: "exceeds threshold — violation",
			todo: project.Todo{
				Schedule: "0 9 * * *",
				SLA:      project.SLAConfig{MaxDelay: 15 * time.Minute},
			},
			// 20 minutes after scheduled time
			now:           time.Date(2026, 2, 27, 9, 20, 0, 0, time.UTC),
			wantHasSLA:    true,
			wantViolation: true,
		},
		{
			name: "no SLA configured — no tracking",
			todo: project.Todo{
				Schedule: "0 9 * * *",
			},
			now:           time.Date(2026, 2, 27, 9, 20, 0, 0, time.UTC),
			wantHasSLA:    false,
			wantViolation: false,
		},
		{
			name: "no schedule — no SLA tracking",
			todo: project.Todo{
				SLA: project.SLAConfig{MaxDelay: 15 * time.Minute},
			},
			now:           time.Date(2026, 2, 27, 9, 20, 0, 0, time.UTC),
			wantHasSLA:    false,
			wantViolation: false,
		},
		{
			name: "global SLA fallback — violation",
			todo: project.Todo{
				Schedule: "0 9 * * *",
			},
			globalCfg:     config.SLAGlobalConfig{DefaultMaxDelay: "15m"},
			now:           time.Date(2026, 2, 27, 9, 20, 0, 0, time.UTC),
			wantHasSLA:    true,
			wantViolation: true,
		},
		{
			name: "global SLA fallback — within threshold",
			todo: project.Todo{
				Schedule: "0 9 * * *",
			},
			globalCfg:     config.SLAGlobalConfig{DefaultMaxDelay: "30m"},
			now:           time.Date(2026, 2, 27, 9, 20, 0, 0, time.UTC),
			wantHasSLA:    true,
			wantViolation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkSLA(tt.todo, tt.globalCfg, tt.now)
			if result.HasSLA != tt.wantHasSLA {
				t.Errorf("HasSLA = %v, want %v", result.HasSLA, tt.wantHasSLA)
			}
			if result.Violation != tt.wantViolation {
				t.Errorf("Violation = %v, want %v", result.Violation, tt.wantViolation)
			}
		})
	}
}

func TestCheckSLAStrictFlag(t *testing.T) {
	result := checkSLA(
		project.Todo{
			Schedule: "0 9 * * *",
			SLA:      project.SLAConfig{MaxDelay: 15 * time.Minute, Strict: true},
		},
		config.SLAGlobalConfig{},
		time.Date(2026, 2, 27, 9, 20, 0, 0, time.UTC),
	)

	if !result.HasSLA {
		t.Fatal("expected HasSLA to be true")
	}
	if !result.Violation {
		t.Fatal("expected Violation to be true")
	}
	if !result.Strict {
		t.Fatal("expected Strict to be true")
	}
	if result.Delay != 20*time.Minute {
		t.Errorf("Delay = %v, want 20m", result.Delay)
	}
}
