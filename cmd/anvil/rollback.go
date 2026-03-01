package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/johnjansen/anvil/internal/project"
)

func taskRollbackCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil task rollback <task-name> [run-id] [--dry-run] [--files <file-list>]\n")
		os.Exit(1)
	}

	taskName := args[0]
	projectPath, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting project path: %v\n", err)
		os.Exit(1)
	}

	// Load project to get task ID
	proj, err := project.Load(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading project: %v\n", err)
		os.Exit(1)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading todos: %v\n", err)
		os.Exit(1)
	}

	var todo *project.Todo
	for _, t := range todos {
		if strings.TrimSuffix(t.Name, ".md") == taskName {
			todo = &t
			break
		}
	}

	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	taskID := todo.ID
	if taskID == "" {
		taskID = taskName // fallback to task name if ID is empty
	}

	// Parse additional arguments
	var runID string
	dryRun := false
	var files []string

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--files":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "missing file list for --files flag\n")
				os.Exit(1)
			}
			i++
			files = strings.Split(args[i], ",")
			// Trim whitespace from each file
			for j, f := range files {
				files[j] = strings.TrimSpace(f)
			}
		default:
			if runID == "" {
				runID = arg
			} else {
				fmt.Fprintf(os.Stderr, "unexpected argument: %s\n", arg)
				os.Exit(1)
			}
		}
	}

	// If no run ID specified, list available restore points
	if runID == "" {
		listRestorePoints(projectPath, taskID)
		return
	}

	// Execute rollback
	executeRollback(projectPath, taskID, runID, files, dryRun, *todo)
}

func listRestorePoints(projectPath, taskID string) {
	records, err := project.ListRestorePoints(projectPath, taskID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing restore points: %v\n", err)
		os.Exit(1)
	}

	if len(records) == 0 {
		fmt.Println("No restore points available")
		return
	}

	// Create a tab writer for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tTIMESTAMP\tSTATUS\tOUTPUT SIZE")

	for _, rec := range records {
		status := "success"
		if !rec.Success {
			status = "failed"
		}

		// Calculate approximate output size (this is a simplification)
		outputSize := "unknown"
		if rec.OutputSummary != "" {
			outputSize = fmt.Sprintf("%d bytes", len(rec.OutputSummary))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			rec.RunID,
			rec.Started.Format("2006-01-02 15:04"),
			status,
			outputSize)
	}

	w.Flush()
}

func executeRollback(projectPath, taskID, runID string, files []string, dryRun bool, todo project.Todo) {
	// Run rollback hook if configured
	if todo.OnRollback != "" && !dryRun {
		if err := project.RunRollbackHook(todo, runID); err != nil {
			fmt.Fprintf(os.Stderr, "error running rollback hook: %v\n", err)
			os.Exit(1)
		}
	}

	// Perform the rollback
	restoredFiles, err := project.RestoreFiles(projectPath, taskID, runID, files, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error restoring files: %v\n", err)
		os.Exit(1)
	}

	if dryRun {
		fmt.Println("DRY RUN - Would restore the following files:")
		for _, file := range restoredFiles {
			fmt.Printf("  %s\n", file)
		}
	} else {
		fmt.Println("Successfully restored files:")
		for _, file := range restoredFiles {
			fmt.Printf("  %s\n", file)
		}
	}
}