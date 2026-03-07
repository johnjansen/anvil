// Package cron provides a cron expression parser and matcher.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Field represents a single field in a cron expression.
type Field struct {
	values map[int]bool
}

// Parser holds parsed cron expression fields.
type Parser struct {
	minute     Field
	hour       Field
	dayOfMonth Field
	month      Field
	dayOfWeek  Field
}

// bounds defines the valid range for each cron field.
type bounds struct {
	min, max int
}

var fieldBounds = []bounds{
	{0, 59}, // minute
	{0, 23}, // hour
	{1, 31}, // day of month
	{1, 12}, // month
	{0, 6},  // day of week (0 = Sunday)
}

// Parse parses a standard 5-field cron expression.
// Format: minute hour day-of-month month day-of-week
// Supports: * (any), */n (every n), n (specific), n,m (list), n-m (range)
func Parse(expr string) (*Parser, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	p := &Parser{}
	var err error

	p.minute, err = parseField(fields[0], fieldBounds[0])
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}

	p.hour, err = parseField(fields[1], fieldBounds[1])
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}

	p.dayOfMonth, err = parseField(fields[2], fieldBounds[2])
	if err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}

	p.month, err = parseField(fields[3], fieldBounds[3])
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}

	p.dayOfWeek, err = parseField(fields[4], fieldBounds[4])
	if err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}

	return p, nil
}

// parseField parses a single cron field.
func parseField(field string, b bounds) (Field, error) {
	f := Field{values: make(map[int]bool)}

	// Handle multiple parts separated by comma
	parts := strings.Split(field, ",")
	for _, part := range parts {
		if err := parsePart(part, b, f.values); err != nil {
			return Field{}, err
		}
	}

	return f, nil
}

// parsePart parses a single part of a field (handles *, */n, n, n-m).
func parsePart(part string, b bounds, values map[int]bool) error {
	// Handle step values (*/n or n-m/s)
	step := 1
	if idx := strings.Index(part, "/"); idx != -1 {
		stepStr := part[idx+1:]
		var err error
		step, err = strconv.Atoi(stepStr)
		if err != nil || step <= 0 {
			return fmt.Errorf("invalid step value: %s", stepStr)
		}
		part = part[:idx]
	}

	var rangeStart, rangeEnd int

	switch {
	case part == "*":
		rangeStart = b.min
		rangeEnd = b.max
	case strings.Contains(part, "-"):
		// Range: n-m
		parts := strings.Split(part, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid range: %s", part)
		}
		var err error
		rangeStart, err = strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid range start: %s", parts[0])
		}
		rangeEnd, err = strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid range end: %s", parts[1])
		}
	default:
		// Single value
		val, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("invalid value: %s", part)
		}
		if val < b.min || val > b.max {
			return fmt.Errorf("value %d out of bounds [%d, %d]", val, b.min, b.max)
		}
		values[val] = true
		return nil
	}

	// Validate bounds
	if rangeStart < b.min || rangeStart > b.max {
		return fmt.Errorf("range start %d out of bounds [%d, %d]", rangeStart, b.min, b.max)
	}
	if rangeEnd < b.min || rangeEnd > b.max {
		return fmt.Errorf("range end %d out of bounds [%d, %d]", rangeEnd, b.min, b.max)
	}

	// Add values with step
	for i := rangeStart; i <= rangeEnd; i += step {
		values[i] = true
	}

	return nil
}

// Matches checks if the given time matches the cron expression.
func (p *Parser) Matches(t time.Time) bool {
	return p.minute.values[t.Minute()] &&
		p.hour.values[t.Hour()] &&
		p.dayOfMonth.values[t.Day()] &&
		p.month.values[int(t.Month())] &&
		p.dayOfWeek.values[int(t.Weekday())]
}

// Next returns the next time that matches the cron expression after the given time.
// Returns an error if no matching time is found within the next 5 years.
func (p *Parser) Next(after time.Time) (time.Time, error) {
	// Start from the next minute
	t := after.Add(time.Minute).Truncate(time.Minute)

	// Limit search to 5 years to prevent infinite loops
	end := after.AddDate(5, 0, 0)

	for t.Before(end) {
		if p.Matches(t) {
			return t, nil
		}

		// Advance time
		t = t.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within 5 years")
}

// Prev returns the previous time that matches the cron expression before the given time.
// Returns an error if no matching time is found within the last 5 years.
func (p *Parser) Prev(before time.Time) (time.Time, error) {
	// Start from the previous minute
	t := before.Add(-time.Minute).Truncate(time.Minute)

	// Limit search to 5 years to prevent infinite loops
	end := before.AddDate(-5, 0, 0)

	for t.After(end) {
		if p.Matches(t) {
			return t, nil
		}

		// Go back in time
		t = t.Add(-time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within 5 years")
}

// CountMissed returns the number of times the cron expression would have matched
// between from (exclusive) and to (inclusive). It returns 0 if no runs were missed.
func (p *Parser) CountMissed(from time.Time, to time.Time) (int, error) {
	if !to.After(from) {
		return 0, nil
	}

	count := 0
	t := from.Truncate(time.Minute).Add(time.Minute) // Start from next minute after 'from'

	for !t.After(to) {
		if p.Matches(t) {
			count++
		}
		t = t.Add(time.Minute)
	}

	return count, nil
}

// String returns a string representation of the parsed expression.
func (p *Parser) String() string {
	return fmt.Sprintf("minute=%v, hour=%v, dom=%v, month=%v, dow=%v",
		setToSlice(p.minute.values),
		setToSlice(p.hour.values),
		setToSlice(p.dayOfMonth.values),
		setToSlice(p.month.values),
		setToSlice(p.dayOfWeek.values),
	)
}

// Matches is a convenience function that parses the expression and checks if it matches the given time.
// It returns false if the expression cannot be parsed.
func Matches(expr string, t time.Time) bool {
	p, err := Parse(expr)
	if err != nil {
		return false
	}
	return p.Matches(t)
}

// setToSlice converts a map[int]bool to a sorted slice of values.
func setToSlice(m map[int]bool) []int {
	result := make([]int, 0, len(m))
	for v := range m {
		result = append(result, v)
	}
	// Sort not needed for display, but could be added
	return result
}
