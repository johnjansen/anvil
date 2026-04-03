package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/output"
	"github.com/johnjansen/anvil/internal/project"
)

// ImpactReport represents the analysis of a task's impact on the system
type ImpactReport struct {
	TaskName        string            `json:"taskName"`
	Schedule        string            `json:"schedule"`
	Priority        string            `json:"priority"`
	Dependencies    []string          `json:"dependencies"`
	Conflicts       []Conflict        `json:"conflicts"`
	Suggestions     []Suggestion      `json:"suggestions"`
	PromptChars     int               `json:"promptChars"`
	InputTokens     int               `json:"inputTokens"`
	OutputTokens    int               `json:"outputTokens"`
	EstimatedCost   float64           `json:"estimatedCost"`
	MonthlyRuns     int               `json:"monthlyRuns"`
	MonthlyCost     float64           `json:"monthlyCost"`
	ResourceUsage   ResourceUsage     `json:"resourceUsage"`
	ExecutionWindow ExecutionWindow   `json:"executionWindow"`
	Concurrency     ConcurrencyImpact `json:"concurrency"`
}

// Conflict represents a scheduling conflict with another task
type Conflict struct {
	TaskName string    `json:"taskName"`
	Time     time.Time `json:"time"`
	Type     string    `json:"type"` // "overlap", "resource", etc.
}

// Suggestion represents an alternative schedule to avoid conflicts
type Suggestion struct {
	Schedule      string `json:"schedule"`
	ConflictCount int    `json:"conflictCount"`
}

// ResourceUsage represents estimated resource consumption
type ResourceUsage struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

// ExecutionWindow represents when the task typically runs
type ExecutionWindow struct {
	AvgDuration string `json:"avgDuration"`
	PeakHours   string `json:"peakHours"`
}

// ConcurrencyImpact represents how the task affects concurrent operations
type ConcurrencyImpact struct {
	MaxConcurrent int    `json:"maxConcurrent"`
	Bottleneck    string `json:"bottleneck"`
	ImpactLevel   string `json:"impactLevel"` // "low", "medium", "high"
}

// analyzeImpact performs impact analysis for a task
func analyzeImpact(taskName, projectPath string) (*ImpactReport, error) {
	// Load the task
	task, err := project.LoadTask(projectPath, taskName)
	if err != nil {
		return nil, fmt.Errorf("failed to load task %s: %w", taskName, err)
	}

	// Parse schedule if present
	var schedule string
	if task.Schedule != "" {
		schedule = task.Schedule
	} else {
		schedule = "manual"
	}

	// Estimate tokens and cost
	content := strings.TrimSpace(task.Prompt)
	charCount := len(content)
	inputTokens := charCount / 4
	outputTokens := inputTokens / 4

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

	inputCost := float64(inputTokens) / 1_000_000 * inputRate
	outputCost := float64(outputTokens) / 1_000_000 * outputRate
	totalCost := inputCost + outputCost

	// Estimate monthly runs
	monthlyRuns := 0
	if schedule != "manual" && schedule != "persistent" {
		monthlyRuns = estimateRunsPerMonth(schedule)
	}
	monthlyCost := totalCost * float64(monthlyRuns)

	// Mock data for other fields (would be calculated in real implementation)
	report := &ImpactReport{
		TaskName:      taskName,
		Schedule:      schedule,
		Priority:      task.Priority,
		Dependencies:  task.Dependencies,
		PromptChars:   charCount,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		EstimatedCost: totalCost,
		MonthlyRuns:   monthlyRuns,
		MonthlyCost:   monthlyCost,
		ResourceUsage: ResourceUsage{
			CPU:    "moderate",
			Memory: "low",
			Disk:   "minimal",
		},
		ExecutionWindow: ExecutionWindow{
			AvgDuration: "2-5 minutes",
			PeakHours:   "business hours",
		},
		Concurrency: ConcurrencyImpact{
			MaxConcurrent: 1,
			Bottleneck:    "none",
			ImpactLevel:   "low",
		},
	}

	// Add mock conflicts and suggestions if this is a high-frequency task
	if monthlyRuns > 100 {
		report.Conflicts = []Conflict{
			{TaskName: "backup-database", Time: time.Now().Add(2 * time.Hour), Type: "resource"},
			{TaskName: "send-notifications", Time: time.Now().Add(3 * time.Hour), Type: "overlap"},
		}
		report.Suggestions = []Suggestion{
			{Schedule: "0 2 * * *", ConflictCount: 1},
			{Schedule: "0 3 * * *", ConflictCount: 0},
		}
	}

	return report, nil
}

// printImpactReport outputs the impact report in a human-readable format
func printImpactReport(report *ImpactReport, jsonOutput bool) {
	if jsonOutput {
		printImpactJSON(*report)
		return
	}

	// Use the output package instead of fmt for synchronized printing
	output.Printf("Impact Analysis for Task: %s\n", report.TaskName)
	output.Println(strings.Repeat("=", 50))
	output.Printf("Schedule: %s\n", report.Schedule)
	output.Printf("Priority: %s\n", report.Priority)
	output.Printf("Prompt Length: %d characters (%d input tokens)\n", report.PromptChars, report.InputTokens)

	if len(report.Dependencies) > 0 {
		output.Println("\nDependencies:")
		for _, dep := range report.Dependencies {
			output.Printf("  - %s\n", dep)
		}
	}

	if len(report.Conflicts) > 0 {
		output.Println("\nPotential Conflicts:")
		for _, conflict := range report.Conflicts {
			output.Printf("  - %s (%s) at %s\n", conflict.TaskName, conflict.Type, conflict.Time.Format("15:04"))
		}
	}

	if len(report.Suggestions) > 0 {
		output.Println()
		output.Println("Suggested Alternatives:")
		for _, s := range report.Suggestions {
			output.Printf("  - %-20s (%d conflict", s.Schedule, s.ConflictCount)
			if s.ConflictCount != 1 {
				output.Print("s")
			}
			output.Println(")")
		}
	}

	// Print cost estimation
	output.Println()
	output.Println("Cost Estimation:")
	output.Println(strings.Repeat("\u2500", 20))
	output.Printf("Prompt length: %d characters\n", report.PromptChars)
	output.Printf("Estimated tokens: ~%d input, ~%d output\n", report.InputTokens, report.OutputTokens)
	output.Printf("Estimated cost: $%.4f per run\n", report.EstimatedCost)

	if report.MonthlyRuns > 0 {
		output.Printf("Frequency: %d runs/month\n", report.MonthlyRuns)
		output.Printf("Monthly estimate: $%.2f\n", report.MonthlyCost)
	}

	output.Println()

	// Print resource usage
	output.Println("Resource Usage:")
	output.Println(strings.Repeat("\u2500", 20))
	output.Printf("CPU:    %s\n", report.ResourceUsage.CPU)
	output.Printf("Memory: %s\n", report.ResourceUsage.Memory)
	output.Printf("Disk:   %s\n", report.ResourceUsage.Disk)

	// Print execution window
	output.Println("\nExecution Window:")
	output.Println(strings.Repeat("\u2500", 20))
	output.Printf("Avg Duration: %s\n", report.ExecutionWindow.AvgDuration)
	output.Printf("Peak Hours:   %s\n", report.ExecutionWindow.PeakHours)

	// Print concurrency impact
	output.Println("\nConcurrency Impact:")
	output.Println(strings.Repeat("\u2500", 20))
	output.Printf("Max Concurrent: %d\n", report.Concurrency.MaxConcurrent)
	output.Printf("Bottleneck:     %s\n", report.Concurrency.Bottleneck)
	output.Printf("Impact Level:   %s\n", report.Concurrency.ImpactLevel)

	output.Println()
}

// printImpactJSON outputs the impact report in JSON format.
func printImpactJSON(report ImpactReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal JSON: %v", err)
	}
	output.Println(string(data))
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

	output.Println("Cost Analysis")
	output.Println(strings.Repeat("\u2500", 40))
	output.Printf("Prompt length: %d characters\n", charCount)
	output.Printf("Estimated tokens: ~%d input, ~%d output\n", inputTokens, outputTokens)
	output.Printf("Estimated cost: $%.4f (input: $%.4f, output: $%.4f)\n",
		totalCost, inputCost, outputCost)
	output.Printf("Rate: $%.2f/1M input, $%.2f/1M output\n", inputRate, outputRate)

	// If there's a schedule, calculate frequency-based costs
	if schedule != "" && schedule != "persistent" {
		runsPerMonth := estimateRunsPerMonth(schedule)
		if runsPerMonth > 0 {
			monthlyCost := totalCost * float64(runsPerMonth)
			output.Println()
			output.Printf("Schedule: %s (%d runs/month)\n", schedule, runsPerMonth)
			output.Printf("Monthly estimate: $%.2f\n", monthlyCost)
		}
	} else if schedule == "persistent" {
		output.Println()
		output.Println("Note: Persistent tasks have variable run frequency.")
		output.Println("Cost will depend on how often the task executes.")
	}

	output.Println()
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
