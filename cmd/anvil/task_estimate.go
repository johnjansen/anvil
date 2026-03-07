package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func taskEstimateCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task estimate <name>\n")
		os.Exit(1)
	}

	name := args[0]

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	todo := findTodo(todos, name)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", name)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	inputRate := cfg.InputTokenRate
	if inputRate <= 0 {
		inputRate = 3.0
	}
	outputRate := cfg.OutputTokenRate
	if outputRate <= 0 {
		outputRate = 15.0
	}

	content := strings.TrimSpace(todo.Content)
	charCount := len(content)
	inputTokens := charCount / 4
	outputTokens := inputTokens / 4

	// Check historical run data for better output token estimates
	records, _ := project.ReadAllRunRecords(abs, todo.ID)
	historyCount := 0
	if len(records) > 0 {
		var totalOut int
		for _, r := range records {
			if r.OutputTokens > 0 {
				totalOut += r.OutputTokens
				historyCount++
			}
		}
		if historyCount > 0 {
			outputTokens = totalOut / historyCount
		}
	}

	inputCost := float64(inputTokens) / 1_000_000 * inputRate
	outputCost := float64(outputTokens) / 1_000_000 * outputRate
	totalCost := inputCost + outputCost

	taskName := strings.TrimSuffix(todo.Name, ".md")

	fmt.Printf("Task: %s\n", taskName)
	fmt.Printf("Prompt length: %d characters\n", charCount)
	fmt.Printf("Estimated tokens: ~%d input, ~%d output\n", inputTokens, outputTokens)
	if historyCount > 0 {
		fmt.Printf("  (output estimate based on %d historical run(s))\n", historyCount)
	}
	fmt.Printf("Estimated cost: $%.4f (input: $%.4f, output: $%.4f)\n", totalCost, inputCost, outputCost)
	fmt.Printf("Rate: $%.2f/1M input, $%.2f/1M output\n", inputRate, outputRate)

	if todo.CostBudget > 0 {
		pct := (totalCost / todo.CostBudget) * 100
		fmt.Printf("\nBudget: $%.2f/task (%.1f%% of budget would be used)\n", todo.CostBudget, pct)
	}

	if todo.Schedule != "" && todo.Schedule != "persistent" {
		runsPerMonth := estimateRunsPerMonth(todo.Schedule)
		if runsPerMonth > 0 {
			dailyRuns := runsPerMonth / 30
			if dailyRuns < 1 {
				dailyRuns = 1
			}
			monthlyCost := totalCost * float64(runsPerMonth)
			fmt.Println()
			fmt.Printf("Schedule: %s\n", todo.Schedule)
			fmt.Printf("Estimated cost per run: $%.4f\n", totalCost)
			fmt.Printf("Daily (%d runs): $%.4f\n", dailyRuns, totalCost*float64(dailyRuns))
			fmt.Printf("Monthly (%d runs): $%.2f\n", runsPerMonth, monthlyCost)
		}
	}
}
