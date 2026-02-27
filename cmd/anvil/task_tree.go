package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/johnjansen/anvil/internal/project"
)

// taskTreeCmd shows the task tree for a given task.
func taskTreeCmd(args []string) {
	flags := flag.NewFlagSet("task tree", flag.ContinueOnError)
	depth := flags.Int("depth", 3, "maximum depth to display (max 5)")
	jsonOutput := flags.Bool("json", false, "output as JSON")
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: anvil task tree <task-name> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Show parent-child relationships for a task.\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		os.Exit(1)
	}

	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(1)
	}

	// Limit depth to 5
	if *depth > 5 {
		*depth = 5
	}

	taskName := flags.Arg(0)

	// Load project
	proj, err := project.Load(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load todos to find task ID
	todos, err := proj.LoadTodos()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Find task
	var taskID string
	for _, t := range todos {
		if t.Name == taskName {
			taskID = t.ID
			break
		}
	}

	if taskID == "" {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	// Get task tree
	parent, children, err := project.GetTaskTree(proj.Path, taskID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if parent.RunID == "" {
		fmt.Printf("No runs found for task: %s\n", taskName)
		os.Exit(0)
	}

	if *jsonOutput {
		output := map[string]interface{}{
			"task":     taskName,
			"parent":   parent,
			"children": children,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Print tree
	fmt.Printf("%s (parent)\n", taskName)
	if len(children) > 0 {
		for _, child := range children {
			status := "running"
			if child.Finished.IsZero() {
				status = "running"
			} else if child.Success {
				status = "done"
			} else {
				status = "failed"
			}
			fmt.Printf("├── %s (%s)\n", child.TaskID, status)
		}
	} else {
		fmt.Printf("└── (no children)\n")
	}
}
