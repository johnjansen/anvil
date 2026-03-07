package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

func taskTriggerCheckCmd(args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(os.Stderr, `usage: anvil task trigger-check <name>

Evaluate trigger conditions for a task and report results.

Options:
  -h, --help    Show this help message
`)
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
			os.Exit(0)
		}
		os.Exit(1)
	}

	taskName := args[0]
	if !strings.HasSuffix(taskName, ".md") {
		taskName += ".md"
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("failed to get absolute path: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	var todo *project.Todo
	for i := range todos {
		if todos[i].Name == taskName {
			todo = &todos[i]
			break
		}
	}

	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	if todo.Trigger == nil || len(todo.Trigger.Conditions) == 0 {
		fmt.Printf("Task %s has no trigger conditions configured.\n", todo.Name)
		if todo.Schedule != "" {
			fmt.Printf("Schedule: %s\n", todo.Schedule)
		}
		os.Exit(0)
	}

	trigger := *todo.Trigger

	fmt.Printf("Task: %s\n", todo.Name)
	if trigger.Schedule != "" {
		fmt.Printf("Schedule: %s\n", trigger.Schedule)
	}

	logic := strings.ToUpper(trigger.ConditionLogic)
	if logic == "" {
		logic = "AND"
	}
	fmt.Printf("Logic: %s\n", logic)

	if trigger.ManualTriggerOnly {
		fmt.Println("Mode: manual trigger only")
	}

	if trigger.PollingConfig != nil && trigger.PollingConfig.Enabled {
		fmt.Printf("Polling: enabled (interval=%v, timeout=%v, run_once=%v)\n",
			trigger.PollingConfig.Interval, trigger.PollingConfig.Timeout, trigger.PollingConfig.RunOnce)
	}

	fmt.Printf("\nConditions (%d):\n", len(trigger.Conditions))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eval, err := project.EvaluateTriggerConditions(ctx, trigger, *todo)
	if err != nil {
		log.Fatalf("failed to evaluate conditions: %v", err)
	}

	for i, result := range eval.ConditionResults {
		cond := trigger.Conditions[i]
		status := "FAIL"
		if result.Met {
			status = "PASS"
		}
		fmt.Printf("  [%s] %s: %s (%s, %v)\n", status, cond.Type, cond.Value, result.Message, result.Duration.Round(time.Millisecond))
	}

	fmt.Println()
	if eval.ConditionsMet {
		fmt.Println("Result: ALL conditions met — task WOULD trigger")
	} else {
		fmt.Println("Result: conditions NOT met — task would NOT trigger")
	}
}
