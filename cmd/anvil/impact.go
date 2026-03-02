package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
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
	Schedule        string           `json:"schedule"`
	IsValid         bool             `json:"valid"`
	ParseError      string           `json:"parse_error,omitempty"`
	NextRun         time.Time        `json:"next_run,omitempty"`
	Conflicts       []ImpactConflict `json:"conflicts"`
	PeakConcurrency int              `json:"peak_concurrency"`
	PeakTime        time.Time        `json:"peak_time,omitempty"`
	Suggestions     []Suggestion     `json:"suggestions,omitempty"`
	NoSchedule      bool             `json:"no_schedule,omitempty"`
	// Cost estimation fields
	PromptChars     int     `json:"prompt_chars,omitempty"`
	InputTokens     int     `json:"input_tokens,omitempty"`
	OutputTokens    int     `json:"output_tokens,omitempty"`
	EstimatedCost   float64 `json:"estimated_cost,omitempty"`
	MonthlyRuns     int     `json:"monthly_runs,omitempty"`
	MonthlyCost     float64 `json:"monthly_cost,omitempty"`
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
func analyzeImpact(schedule string, todos []project.Todo, taskContent string) ImpactReport {
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

	// Add cost estimation
	content := strings.TrimSpace(taskContent)
	report.PromptChars = len(content)
	// Rough estimate: ~4 characters per token
	report.InputTokens = report.PromptChars / 4
	// Estimate output tokens (typically 20-50% of input for prompts)
	report.OutputTokens = report.InputTokens / 4

	// Load config for token rates
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	inputRate := cfg.InputTokenRate
	if inputRate <= 0 {
		inputRate = 3.0 // default Sonnet rate
	}
	outputRate := cfg.OutputTokenRate
	if outputRate <= 0 {
		outputRate = 15.0 // default Sonnet rate
	}

	// Calculate costs
	inputCost := float64(report.InputTokens) / 1_000_000 * inputRate
	outputCost := float64(report.OutputTokens) / 1_000_000 * outputRate
	report.EstimatedCost = inputCost + outputCost

	// If there's a schedule, calculate frequency-based costs
	if schedule != "" && schedule != "persistent" {
		report.MonthlyRuns = estimateRunsPerMonth(schedule)
		if report.MonthlyRuns > 0 {
			report.MonthlyCost = report.EstimatedCost * float64(report.MonthlyRuns)
		}
	}

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

	// Print cost estimation
	fmt.Println()
	fmt.Println("Cost Estimation:")
	fmt.Println(strings.Repeat("\u2500", 20))
	fmt.Printf("Prompt length: %d characters\n", report.PromptChars)
	fmt.Printf("Estimated tokens: ~%d input, ~%d output\n", report.InputTokens, report.OutputTokens)
	fmt.Printf("Estimated cost: $%.4f per run\n", report.EstimatedCost)

	if report.MonthlyRuns > 0 {
		fmt.Printf("Frequency: %d runs/month\n", report.MonthlyRuns)
		fmt.Printf("Monthly estimate: $%.2f\n", report.MonthlyCost)
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

// printCostEstimate shows token and cost estimation for a task
func printCostEstimate(taskText, schedule, projectPath string) {
	// Load config for token rates
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	inputRate := cfg.InputTokenRate
	if inputRate <= 0 {
		inputRate = 3.0 // default Sonnet rate
	}
	outputRate := cfg.OutputTokenRate
	if outputRate <= 0 {
		outputRate = 15.0 // default Sonnet rate
	}

	// Estimate tokens using the same logic as prompt analyze
	content := strings.TrimSpace(taskText)
	charCount := len(content)
	// Rough estimate: ~4 characters per token
	inputTokens := charCount / 4
	// Estimate output tokens (typically 20-50% of input for prompts)
	outputTokens := inputTokens / 4

	// Calculate costs
	inputCost := float64(inputTokens) / 1_000_000 * inputRate
	outputCost := float64(outputTokens) / 1_000_000 * outputRate
	totalCost := inputCost + outputCost

	fmt.Println("Cost Analysis")
	fmt.Println(strings.Repeat("\u2500", 40))
	fmt.Printf("Prompt length: %d characters\n", charCount)
	fmt.Printf("Estimated tokens: ~%d input, ~%d output\n", inputTokens, outputTokens)
	fmt.Printf("Estimated cost: $%.4f (input: $%.4f, output: $%.4f)\n",
		totalCost, inputCost, outputCost)
	fmt.Printf("Rate: $%.2f/1M input, $%.2f/1M output\n", inputRate, outputRate)

	// If there's a schedule, calculate frequency-based costs
	if schedule != "" && schedule != "persistent" {
		runsPerMonth := estimateRunsPerMonth(schedule)
		if runsPerMonth > 0 {
			monthlyCost := totalCost * float64(runsPerMonth)
			fmt.Println()
			fmt.Printf("Schedule: %s (%d runs/month)\n", schedule, runsPerMonth)
			fmt.Printf("Monthly estimate: $%.2f\n", monthlyCost)
		}
	} else if schedule == "persistent" {
		fmt.Println()
		fmt.Println("Note: Persistent tasks have variable run frequency.")
		fmt.Println("Cost will depend on how often the task executes.")
	}

	fmt.Println()
}

// estimateRunsPerMonth estimates how many times a cron schedule runs per month
func estimateRunsPerMonth(schedule string) int {
	parser, err := cron.Parse(schedule)
	if err != nil {
		return 0
	}

	// Count runs over a typical month (30 days)
	now := time.Now()
	end := now.AddDate(0, 1, 0) // 1 month from now
	count := 0
	current := now

	for current.Before(end) {
		next, err := parser.Next(current)
		if err != nil {
			break
		}
		if next.After(end) {
			break
		}
		count++
		current = next
	}

	return count
}
