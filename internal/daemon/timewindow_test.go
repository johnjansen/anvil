package daemon

import (
	"testing"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input   string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"09:00", 9, 0, false},
		{"18:30", 18, 30, false},
		{"00:00", 0, 0, false},
		{"23:59", 23, 59, false},
		{"24:00", 0, 0, true},
		{"12:60", 0, 0, true},
		{"bad", 0, 0, true},
		{"", 0, 0, true},
	}
	for _, tc := range tests {
		h, m, err := parseHHMM(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseHHMM(%q): expected error, got %d:%d", tc.input, h, m)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseHHMM(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if h != tc.wantH || m != tc.wantM {
			t.Errorf("parseHHMM(%q) = %d:%d, want %d:%d", tc.input, h, m, tc.wantH, tc.wantM)
		}
	}
}

func TestParseDays(t *testing.T) {
	tests := []struct {
		input   string
		want    map[int]bool
		wantErr bool
	}{
		{"1-5", map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true}, false},
		{"0,6", map[int]bool{0: true, 6: true}, false},
		{"1-5,0", map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true, 5: true}, false},
		{"", nil, false},
		{"7", nil, true},
		{"-1", nil, true},
	}
	for _, tc := range tests {
		got, err := parseDays(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDays(%q): expected error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDays(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if tc.want == nil && got != nil {
			t.Errorf("parseDays(%q) = %v, want nil", tc.input, got)
			continue
		}
		if tc.want != nil {
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("parseDays(%q): day %d = %v, want %v", tc.input, k, got[k], v)
				}
			}
		}
	}
}

func TestIsInTimeWindow(t *testing.T) {
	// Monday 2026-03-02 10:30
	monday1030 := time.Date(2026, 3, 2, 10, 30, 0, 0, time.Local)
	// Monday 2026-03-02 20:30
	monday2030 := time.Date(2026, 3, 2, 20, 30, 0, 0, time.Local)
	// Saturday 2026-03-07 10:30
	saturday1030 := time.Date(2026, 3, 7, 10, 30, 0, 0, time.Local)
	// Monday 2026-03-02 23:30
	monday2330 := time.Date(2026, 3, 2, 23, 30, 0, 0, time.Local)
	// Tuesday 2026-03-03 03:00
	tuesday0300 := time.Date(2026, 3, 3, 3, 0, 0, 0, time.Local)
	// Monday 2026-03-02 09:00 (exactly at boundary)
	monday0900 := time.Date(2026, 3, 2, 9, 0, 0, 0, time.Local)
	// Monday 2026-03-02 18:00 (exactly at end boundary)
	monday1800 := time.Date(2026, 3, 2, 18, 0, 0, 0, time.Local)

	tests := []struct {
		name  string
		now   time.Time
		start string
		end   string
		days  string
		want  bool
	}{
		{"within normal window", monday1030, "09:00", "18:00", "1-5", true},
		{"outside normal window (too late)", monday2030, "09:00", "18:00", "1-5", false},
		{"wrong day (Saturday)", saturday1030, "09:00", "18:00", "1-5", false},
		{"midnight-spanning window (in late)", monday2330, "22:00", "06:00", "", true},
		{"midnight-spanning window (in early)", tuesday0300, "22:00", "06:00", "", true},
		{"midnight-spanning window (out)", monday1030, "22:00", "06:00", "", false},
		{"no window (empty)", monday1030, "", "", "", true},
		{"only start set", monday1030, "09:00", "", "", true},
		{"exactly at start boundary", monday0900, "09:00", "18:00", "", true},
		{"exactly at end boundary", monday1800, "09:00", "18:00", "", false},
		{"all days allowed (empty days)", monday1030, "09:00", "18:00", "", true},
		{"all days allowed (Saturday, no day restriction)", saturday1030, "09:00", "18:00", "", true},
	}

	for _, tc := range tests {
		got := isInTimeWindow(tc.now, tc.start, tc.end, tc.days)
		if got != tc.want {
			t.Errorf("%s: isInTimeWindow(%v, %q, %q, %q) = %v, want %v",
				tc.name, tc.now, tc.start, tc.end, tc.days, got, tc.want)
		}
	}
}

func TestIsTaskInWindow(t *testing.T) {
	monday1030 := time.Date(2026, 3, 2, 10, 30, 0, 0, time.Local)
	monday2030 := time.Date(2026, 3, 2, 20, 30, 0, 0, time.Local)

	tests := []struct {
		name string
		todo project.Todo
		now  time.Time
		want bool
	}{
		{
			"no window configured",
			project.Todo{Name: "test"},
			monday1030,
			true,
		},
		{
			"within window",
			project.Todo{Name: "test", Window: project.AllowedWindow{Start: "09:00", End: "18:00", Days: "1-5"}},
			monday1030,
			true,
		},
		{
			"outside window",
			project.Todo{Name: "test", Window: project.AllowedWindow{Start: "09:00", End: "18:00", Days: "1-5"}},
			monday2030,
			false,
		},
		{
			"force window bypass",
			project.Todo{Name: "test", Window: project.AllowedWindow{Start: "09:00", End: "18:00", Days: "1-5"}, ForceWindow: true},
			monday2030,
			true,
		},
	}

	for _, tc := range tests {
		got := isTaskInWindow(tc.todo, tc.now)
		if got != tc.want {
			t.Errorf("%s: isTaskInWindow() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsInQuietHours(t *testing.T) {
	monday2330 := time.Date(2026, 3, 2, 23, 30, 0, 0, time.Local)
	monday1030 := time.Date(2026, 3, 2, 10, 30, 0, 0, time.Local)

	tests := []struct {
		name     string
		now      time.Time
		cfg      config.QuietHoursConfig
		priority int
		want     bool
	}{
		{
			"disabled",
			monday2330,
			config.QuietHoursConfig{Enabled: false, Start: "22:00", End: "07:00"},
			2,
			false,
		},
		{
			"enabled, in quiet hours, p2 task",
			monday2330,
			config.QuietHoursConfig{Enabled: true, Start: "22:00", End: "07:00", ExcludePriority: 0},
			2,
			true,
		},
		{
			"enabled, in quiet hours, p0 task (exempt)",
			monday2330,
			config.QuietHoursConfig{Enabled: true, Start: "22:00", End: "07:00", ExcludePriority: 0},
			0,
			false,
		},
		{
			"enabled, outside quiet hours",
			monday1030,
			config.QuietHoursConfig{Enabled: true, Start: "22:00", End: "07:00", ExcludePriority: 0},
			2,
			false,
		},
		{
			"enabled, in quiet hours, p1 exempt (exclude_priority=1)",
			monday2330,
			config.QuietHoursConfig{Enabled: true, Start: "22:00", End: "07:00", ExcludePriority: 1},
			1,
			false,
		},
		{
			"enabled, in quiet hours, p2 blocked (exclude_priority=1)",
			monday2330,
			config.QuietHoursConfig{Enabled: true, Start: "22:00", End: "07:00", ExcludePriority: 1},
			2,
			true,
		},
	}

	for _, tc := range tests {
		got := isInQuietHours(tc.now, tc.cfg, tc.priority)
		if got != tc.want {
			t.Errorf("%s: isInQuietHours() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNextAllowedRun(t *testing.T) {
	after := time.Date(2026, 3, 2, 20, 0, 0, 0, time.Local) // Monday 8 PM

	// Every 15 minutes, but only during business hours on weekdays
	next, err := NextAllowedRun(
		"*/15 * * * *",
		project.AllowedWindow{Start: "09:00", End: "18:00", Days: "1-5"},
		config.QuietHoursConfig{},
		2,
		after,
	)
	if err != nil {
		t.Fatalf("NextAllowedRun: %v", err)
	}
	// Should skip to Tuesday 09:00
	expected := time.Date(2026, 3, 3, 9, 0, 0, 0, time.Local)
	if !next.Equal(expected) {
		t.Errorf("NextAllowedRun = %v, want %v", next, expected)
	}
}

func TestNextAllowedRunNoWindow(t *testing.T) {
	after := time.Date(2026, 3, 2, 20, 0, 0, 0, time.Local)

	next, err := NextAllowedRun(
		"*/15 * * * *",
		project.AllowedWindow{},
		config.QuietHoursConfig{},
		2,
		after,
	)
	if err != nil {
		t.Fatalf("NextAllowedRun: %v", err)
	}
	// Should be 20:15 (next 15-minute mark)
	expected := time.Date(2026, 3, 2, 20, 15, 0, 0, time.Local)
	if !next.Equal(expected) {
		t.Errorf("NextAllowedRun = %v, want %v", next, expected)
	}
}
