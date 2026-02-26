package daemon

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
)

// parseHHMM parses a "HH:MM" string and returns hour and minute.
func parseHHMM(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format %q: expected HH:MM", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q", s)
	}
	return h, m, nil
}

// parseDays parses a days string like "1-5", "0,6", or "1-5,0" into a set of allowed weekdays.
// Uses 0=Sunday through 6=Saturday (matching Go's time.Weekday).
func parseDays(s string) (map[int]bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil // nil means all days allowed
	}
	days := make(map[int]bool)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil || lo < 0 || lo > 6 {
				return nil, fmt.Errorf("invalid day %q in range", bounds[0])
			}
			hi, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil || hi < 0 || hi > 6 {
				return nil, fmt.Errorf("invalid day %q in range", bounds[1])
			}
			for d := lo; d <= hi; d++ {
				days[d] = true
			}
		} else {
			d, err := strconv.Atoi(part)
			if err != nil || d < 0 || d > 6 {
				return nil, fmt.Errorf("invalid day %q", part)
			}
			days[d] = true
		}
	}
	return days, nil
}

// timeOfDayMinutes converts hours and minutes to total minutes since midnight.
func timeOfDayMinutes(h, m int) int {
	return h*60 + m
}

// isInTimeWindow checks whether the given time falls within the specified time window.
// If start or end is empty, returns true (no restriction).
// Handles windows that span midnight (e.g., start="22:00", end="06:00").
func isInTimeWindow(now time.Time, start, end, days string) bool {
	if start == "" && end == "" {
		return true
	}

	// Check day of week first
	if days != "" {
		allowedDays, err := parseDays(days)
		if err != nil {
			return true // on parse error, allow execution
		}
		if allowedDays != nil && !allowedDays[int(now.Weekday())] {
			return false
		}
	}

	if start == "" || end == "" {
		return true // need both to apply time restriction
	}

	startH, startM, err := parseHHMM(start)
	if err != nil {
		return true
	}
	endH, endM, err := parseHHMM(end)
	if err != nil {
		return true
	}

	nowMins := timeOfDayMinutes(now.Hour(), now.Minute())
	startMins := timeOfDayMinutes(startH, startM)
	endMins := timeOfDayMinutes(endH, endM)

	if startMins <= endMins {
		// Normal window (e.g., 09:00-18:00)
		return nowMins >= startMins && nowMins < endMins
	}
	// Midnight-spanning window (e.g., 22:00-06:00)
	return nowMins >= startMins || nowMins < endMins
}

// isTaskInWindow checks whether a task is currently within its allowed execution window.
// Returns true if no window is configured or if the current time is within the window.
func isTaskInWindow(t project.Todo, now time.Time) bool {
	if t.ForceWindow {
		return true
	}
	if t.Window.Start == "" && t.Window.End == "" {
		return true
	}
	return isInTimeWindow(now, t.Window.Start, t.Window.End, t.Window.Days)
}

// isInQuietHours checks whether a task should be blocked by global quiet hours.
// Returns true (blocked) if quiet hours are enabled, the current time is within the
// quiet window, and the task priority is not exempt.
func isInQuietHours(now time.Time, cfg config.QuietHoursConfig, taskPriority int) bool {
	if !cfg.Enabled {
		return false
	}
	if taskPriority <= cfg.ExcludePriority {
		return false // task priority is exempt
	}
	return isInTimeWindow(now, cfg.Start, cfg.End, "")
}

// NextAllowedRun finds the next time after 'after' when the cron schedule fires
// and the time window and quiet hours constraints are all satisfied.
// Returns zero time if no valid time found within 366 days.
func NextAllowedRun(schedule string, window project.AllowedWindow, quietHours config.QuietHoursConfig, priority int, after time.Time) (time.Time, error) {
	p, err := cron.Parse(schedule)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid schedule: %w", err)
	}

	candidate := after
	limit := after.Add(366 * 24 * time.Hour)

	for {
		next, err := p.Next(candidate)
		if err != nil {
			return time.Time{}, err
		}
		if next.After(limit) {
			return time.Time{}, fmt.Errorf("no valid run time found within 366 days")
		}

		inWindow := isInTimeWindow(next, window.Start, window.End, window.Days)
		inQuiet := isInQuietHours(next, quietHours, priority)

		if inWindow && !inQuiet {
			return next, nil
		}
		candidate = next
	}
}
