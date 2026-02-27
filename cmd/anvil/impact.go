package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
)

// Conflict represents a scheduling conflict between the proposed task and an existing task.
type ImpactConflict struct {
	TaskName    string    `json:"task"`
	Schedule    string    `json:"schedule"`
	OverlapTime time.Time `json:"overlap_time"`
}

// Suggestion represents an alternative schedule with fewer conflicts.
type Suggestion struct {
	Schedule      string `json:"schedule"`
	ConflictCount int    `json:"conflicts"`
	Description   string `json:"description"`
}

// ImpactReport is the result of analyzing a proposed schedule against existing tasks.
type ImpactReport struct {
	Schedule        string       `json:"schedule"`
	IsValid         bool         `json:"valid"`
	ParseError      string       `json:"parse_error,omitempty"`
	NextRun         time.Time    `json:"next_run,omitempty"`
	Conflicts       []ImpactConflict   `json:"conflicts"`
	PeakConcurrency int          `json:"peak_concurrency"`
	PeakTime        time.Time    `json:"peak_time,omitempty"`
	Suggestions     []Suggestion `json:"suggestions,omitempty"`
	NoSchedule      bool         `json:"no_schedule,omitempty"`
}

// computeConflicts checks a proposed schedule against all active tasks and returns conflicts.
// This reuses the overlap detection algorithm from the original taskCreateCmd.
func computeConflicts(schedule string, todos []project.Todo) []ImpactConflict {
	newParser, err := cron.Parse(schedule)
	if err != nil {
		return nil
	}
	now := time.Now().Truncate(time.Minute)
	var conflicts []ImpactConflict
	for _, t := range todos {
		if t.Schedule == "" || t.Schedule == "persistent" || t.Disabled {
			continue
		}
		existParser, err := cron.Parse(t.Schedule)
		if err != nil {
			continue
		}
		cur := now
		for k := 0; k < 30; k++ {
			newNext, e1 := newParser.Next(cur)
			existNext, e2 := existParser.Next(cur)
			if e1 != nil || e2 != nil {
				break
			}
			diff := newNext.Sub(existNext)
			if diff < 0 {
				diff = -diff
			}
			if diff <= time.Minute {
				conflicts = append(conflicts, ImpactConflict{
					TaskName:    strings.TrimSuffix(t.Name, ".md"),
					Schedule:    t.Schedule,
					OverlapTime: newNext,
				})
				break
			}
			if newNext.Before(existNext) {
				cur = newNext
			} else {
				cur = existNext
			}
		}
	}
	return conflicts
}

// computePeakConcurrency enumerates next-24h firing times for all active tasks plus the
// proposed schedule, and finds the time slot with maximum concurrent tasks.
func computePeakConcurrency(schedule string, todos []project.Todo) (int, time.Time) {
	now := time.Now().Truncate(time.Minute)
	end := now.Add(24 * time.Hour)

	// Collect firing times per minute slot
	slotCounts := make(map[time.Time]int)

	// Add proposed schedule's firing times
	if p, err := cron.Parse(schedule); err == nil {
		cur := now
		for {
			next, err := p.Next(cur)
			if err != nil || next.After(end) {
				break
			}
			slotCounts[next.Truncate(time.Minute)]++
			cur = next
		}
	}

	// Add existing tasks' firing times
	for _, t := range todos {
		if t.Schedule == "" || t.Schedule == "persistent" || t.Disabled {
			continue
		}
		p, err := cron.Parse(t.Schedule)
		if err != nil {
			continue
		}
		cur := now
		for {
			next, err := p.Next(cur)
			if err != nil || next.After(end) {
				break
			}
			slotCounts[next.Truncate(time.Minute)]++
			cur = next
		}
	}

	// Find peak
	peak := 0
	var peakTime time.Time
	for t, count := range slotCounts {
		if count > peak {
			peak = count
			peakTime = t
		}
	}
	return peak, peakTime
}

// suggestAlternatives generates alternative schedules by shifting hours and checking conflicts.
func suggestAlternatives(schedule string, todos []project.Todo, currentConflicts int) []Suggestion {
	if currentConflicts == 0 {
		return nil
	}
	fields := strings.Fields(schedule)
	if len(fields) < 5 {
		return nil
	}

	// Parse the hour field
	var hour int
	if _, err := fmt.Sscanf(fields[1], "%d", &hour); err != nil {
		// Can't easily suggest alternatives for complex hour patterns
		return nil
	}

	shifts := []struct {
		offset int
		desc   string
	}{
		{1, "Shift +1 hour"},
		{-1, "Shift -1 hour"},
		{2, "Shift +2 hours"},
		{-2, "Shift -2 hours"},
		{3, "Shift +3 hours"},
		{-3, "Shift -3 hours"},
	}

	var suggestions []Suggestion
	for _, s := range shifts {
		newHour := (hour + s.offset + 24) % 24
		newFields := make([]string, len(fields))
		copy(newFields, fields)
		newFields[1] = fmt.Sprintf("%d", newHour)
		candidate := strings.Join(newFields, " ")

		conflicts := computeConflicts(candidate, todos)
		if len(conflicts) < currentConflicts {
			suggestions = append(suggestions, Suggestion{
				Schedule:      candidate,
				ConflictCount: len(conflicts),
				Description:   s.desc,
			})
		}
		if len(suggestions) >= 3 {
			break
		}
	}

	// Also try shifting minutes by 30 if we haven't found 3 yet
	if len(suggestions) < 3 {
		var minute int
		if _, err := fmt.Sscanf(fields[0], "%d", &minute); err == nil {
			newMinute := (minute + 30) % 60
			newFields := make([]string, len(fields))
			copy(newFields, fields)
			newFields[0] = fmt.Sprintf("%d", newMinute)
			candidate := strings.Join(newFields, " ")
			conflicts := computeConflicts(candidate, todos)
			if len(conflicts) < currentConflicts {
				suggestions = append(suggestions, Suggestion{
					Schedule:      candidate,
					ConflictCount: len(conflicts),
					Description:   "Shift +30 minutes",
				})
			}
		}
	}

	return suggestions
}

// analyzeImpact produces a full impact report for a proposed schedule against existing tasks.
func analyzeImpact(schedule string, todos []project.Todo) ImpactReport {
	report := ImpactReport{
		Schedule:  schedule,
		Conflicts: []ImpactConflict{}, // ensure non-nil for JSON
	}

	if schedule == "" {
		report.NoSchedule = true
		return report
	}

	p, err := cron.Parse(schedule)
	if err != nil {
		report.ParseError = err.Error()
		return report
	}
	report.IsValid = true

	if next, err := p.Next(time.Now()); err == nil {
		report.NextRun = next
	}

	report.Conflicts = computeConflicts(schedule, todos)
	if report.Conflicts == nil {
		report.Conflicts = []ImpactConflict{}
	}

	report.PeakConcurrency, report.PeakTime = computePeakConcurrency(schedule, todos)

	report.Suggestions = suggestAlternatives(schedule, todos, len(report.Conflicts))

	return report
}

// printImpactReport displays the impact analysis in human-readable format.
func printImpactReport(report ImpactReport) {
	if report.NoSchedule {
		fmt.Println("No schedule specified (one-shot task).")
		return
	}

	if report.ParseError != "" {
		log.Fatalf("invalid cron expression: %s (%s)", report.Schedule, report.ParseError)
	}

	fmt.Println()
	fmt.Println("Impact Analysis")
	fmt.Println(strings.Repeat("\u2500", 40))
	fmt.Printf("Schedule:   %s\n", report.Schedule)
	if !report.NextRun.IsZero() {
		until := time.Until(report.NextRun).Round(time.Minute)
		fmt.Printf("Next Run:   %s (%s from now)\n", report.NextRun.Format("Mon Jan 2 15:04:05"), until)
	}
	fmt.Println()

	if len(report.Conflicts) == 0 {
		fmt.Println("No scheduling conflicts.")
	} else {
		fmt.Printf("Scheduling Conflicts (%d):\n", len(report.Conflicts))
		for _, c := range report.Conflicts {
			fmt.Printf("  - %-20s (%s)\n", c.TaskName, c.Schedule)
		}
	}

	if report.PeakConcurrency > 0 {
		fmt.Println()
		fmt.Printf("Peak Concurrency: %d tasks at %s\n", report.PeakConcurrency,
			report.PeakTime.Format("15:04"))
	}

	if len(report.Suggestions) > 0 {
		fmt.Println()
		fmt.Println("Suggested Alternatives:")
		for _, s := range report.Suggestions {
			fmt.Printf("  - %-20s (%d conflict", s.Schedule, s.ConflictCount)
			if s.ConflictCount != 1 {
				fmt.Print("s")
			}
			fmt.Println(")")
		}
	}
	fmt.Println()
}

// printImpactJSON outputs the impact report in JSON format.
func printImpactJSON(report ImpactReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal JSON: %v", err)
	}
	fmt.Println(string(data))
}
