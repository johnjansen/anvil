package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

func taskSubscriptionCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task subscription <subcommand> [options]\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	switch args[0] {
	case "ls", "list":
		taskSubscriptionLsCmd(args[1:])
	case "pause":
		taskSubscriptionPauseCmd(args[1:])
	case "resume":
		taskSubscriptionResumeCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subscription subcommand: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}
}

// taskSubscriptionLsCmd handles the 'anvil task subscription ls' command
func taskSubscriptionLsCmd(args []string) {
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

	// Filter todos with subscriptions
	var subscribedTodos []project.Todo
	for _, todo := range todos {
		if todo.Subscription != nil {
			subscribedTodos = append(subscribedTodos, todo)
		}
	}

	if len(subscribedTodos) == 0 {
		fmt.Println("No subscriptions found")
		return
	}

	// Print table header
	fmt.Printf("%-20s  %-10s  %-20s  %s\n", "TASK", "TYPE", "TARGET", "STATUS")
	fmt.Println(strings.Repeat("-", 70))

	// Print subscription info
	for _, todo := range subscribedTodos {
		subType := "unknown"
		target := "N/A"

		if todo.Subscription != nil {
			subType = todo.Subscription.Type

			switch todo.Subscription.Type {
			case "amqp":
				target = todo.Subscription.Queue
			case "webhook":
				target = todo.Subscription.Path
			case "fs":
				target = todo.Subscription.FsPath
			}
		}

		// For now, we'll show status as "active" - in a real implementation,
		// we'd need to check if the daemon is actually consuming from the subscription
		status := "active"

		fmt.Printf("%-20s  %-10s  %-20s  %s\n",
			strings.TrimSuffix(todo.Name, ".md"),
			subType,
			target,
			status)
	}
}

// taskSubscriptionPauseCmd handles the 'anvil task subscription pause' command
func taskSubscriptionPauseCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task subscription pause <task-name>\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	taskName := args[0]
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

	// Find the task by name
	var targetTodo *project.Todo
	for i := range todos {
		if strings.TrimSuffix(todos[i].Name, ".md") == taskName {
			targetTodo = &todos[i]
			break
		}
	}

	if targetTodo == nil {
		fmt.Fprintf(os.Stderr, "task '%s' not found\n", taskName)
		os.Exit(1)
	}

	// Check if task has a subscription
	if targetTodo.Subscription == nil {
		fmt.Fprintf(os.Stderr, "task '%s' does not have a subscription\n", taskName)
		os.Exit(1)
	}

	// Pause the task by setting Disabled = true
	if err := updateTaskDisabledStatus(proj.Path, targetTodo, true); err != nil {
		log.Fatalf("failed to pause task: %v", err)
	}

	fmt.Printf("Paused subscription for task '%s'\n", taskName)
}

// taskSubscriptionResumeCmd handles the 'anvil task subscription resume' command
func taskSubscriptionResumeCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task subscription resume <task-name>\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	taskName := args[0]
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

	// Find the task by name
	var targetTodo *project.Todo
	for i := range todos {
		if strings.TrimSuffix(todos[i].Name, ".md") == taskName {
			targetTodo = &todos[i]
			break
		}
	}

	if targetTodo == nil {
		fmt.Fprintf(os.Stderr, "task '%s' not found\n", taskName)
		os.Exit(1)
	}

	// Check if task has a subscription
	if targetTodo.Subscription == nil {
		fmt.Fprintf(os.Stderr, "task '%s' does not have a subscription\n", taskName)
		os.Exit(1)
	}

	// Resume the task by setting Disabled = false
	if err := updateTaskDisabledStatus(proj.Path, targetTodo, false); err != nil {
		log.Fatalf("failed to resume task: %v", err)
	}

	fmt.Printf("Resumed subscription for task '%s'\n", taskName)
}

// updateTaskDisabledStatus updates the disabled status of a task by rewriting its file
