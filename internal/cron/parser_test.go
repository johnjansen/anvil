package cron

import (
	"testing"
	"time"
)

func TestCountMissed_EveryThirtyMinutes(t *testing.T) {
	p, err := Parse("*/30 * * * *")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// 3 hours of downtime = 6 missed runs for */30
	from := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 15, 13, 0, 0, 0, time.UTC)

	missed, err := p.CountMissed(from, to)
	if err != nil {
		t.Fatalf("CountMissed error: %v", err)
	}

	if missed != 6 {
		t.Errorf("expected 6 missed runs, got %d", missed)
	}
}

func TestCountMissed_NoMissed(t *testing.T) {
	p, err := Parse("0 9 * * *")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Last run was 5 minutes ago — no missed runs for a daily schedule
	from := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 15, 9, 5, 0, 0, time.UTC)

	missed, err := p.CountMissed(from, to)
	if err != nil {
		t.Fatalf("CountMissed error: %v", err)
	}

	if missed != 0 {
		t.Errorf("expected 0 missed runs, got %d", missed)
	}
}

func TestCountMissed_DailyOverMultipleDays(t *testing.T) {
	p, err := Parse("0 9 * * *")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// 3 days of downtime = 3 missed daily runs
	from := time.Date(2026, 1, 12, 9, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	missed, err := p.CountMissed(from, to)
	if err != nil {
		t.Fatalf("CountMissed error: %v", err)
	}

	if missed != 3 {
		t.Errorf("expected 3 missed runs, got %d", missed)
	}
}

func TestCountMissed_ToBeforeFrom(t *testing.T) {
	p, err := Parse("*/5 * * * *")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	from := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)

	missed, err := p.CountMissed(from, to)
	if err != nil {
		t.Fatalf("CountMissed error: %v", err)
	}

	if missed != 0 {
		t.Errorf("expected 0 missed runs when to < from, got %d", missed)
	}
}

func TestMatches_Basic(t *testing.T) {
	// Test every 5 minutes
	if !Matches("*/5 * * * *", time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)) {
		t.Error("expected */5 to match :00")
	}
	if !Matches("*/5 * * * *", time.Date(2026, 1, 15, 10, 15, 0, 0, time.UTC)) {
		t.Error("expected */5 to match :15")
	}
	if Matches("*/5 * * * *", time.Date(2026, 1, 15, 10, 3, 0, 0, time.UTC)) {
		t.Error("expected */5 to NOT match :03")
	}
}
