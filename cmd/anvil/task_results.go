package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

func taskResultsCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task results <task-name> [--preview] [--run <run-id>] [--json]\n")
		os.Exit(1)
	}

	var taskName string
	var preview bool
	var runID string
	var jsonOutput bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--preview":
			preview = true
		case "--run":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --run requires a run ID argument\n")
				os.Exit(1)
			}
			runID = args[i+1]
			i++
		case "--json":
			jsonOutput = true
		default:
			if taskName == "" {
				taskName = args[i]
			}
		}
	}

	if taskName == "" {
		fmt.Fprintf(os.Stderr, "error: task name required\n")
		os.Exit(1)
	}

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

	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	if preview {
		showResultPreview(abs, todo, jsonOutput)
		return
	}

	showResult(abs, todo, runID, jsonOutput)
}

func showResult(projectPath string, todo *project.Todo, runID string, jsonOutput bool) {
	var rec project.RunRecord
	var err error

	if runID != "" {
		recPtr, readErr := project.ReadRunRecord(projectPath, todo.ID, runID)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error reading run %s: %v\n", runID, readErr)
			os.Exit(1)
		}
		rec = *recPtr
	} else {
		rec, err = project.ReadCurrentRunRecord(projectPath, todo.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "No captured results for task %q.\nEnsure the task has capture_output: true in its frontmatter.\n", todo.Name)
			os.Exit(1)
		}
	}

	if rec.ResultData == "" {
		fmt.Fprintf(os.Stderr, "No captured results for task %q.\nEnsure the task has capture_output: true in its frontmatter.\n", todo.Name)
		os.Exit(1)
	}

	if jsonOutput {
		fmt.Println(rec.ResultData)
		return
	}

	fmt.Printf("Task: %s\n", strings.TrimSuffix(todo.Name, ".md"))
	fmt.Printf("Run:  %s\n", rec.RunID)
	if !rec.Finished.IsZero() {
		fmt.Printf("Time: %s\n", rec.Finished.Format("2006-01-02 15:04:05"))
	}
	fmt.Println()
	fmt.Println("Result:")

	// Pretty-print JSON if valid
	if json.Valid([]byte(rec.ResultData)) {
		var buf json.RawMessage
		json.Unmarshal([]byte(rec.ResultData), &buf)
		pretty, err := json.MarshalIndent(buf, "  ", "  ")
		if err == nil {
			fmt.Printf("  %s\n", string(pretty))
			return
		}
	}
	fmt.Printf("  %s\n", rec.ResultData)
}

func showResultPreview(projectPath string, todo *project.Todo, jsonOutput bool) {
	if len(todo.DependsOn) == 0 {
		fmt.Fprintf(os.Stderr, "Task %q has no dependencies.\n", todo.Name)
		os.Exit(1)
	}

	depResults, err := project.CollectDependencyResults(projectPath, todo.DependsOn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error collecting dependency results: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		data, _ := json.Marshal(depResults)
		fmt.Println(string(data))
		return
	}

	taskName := strings.TrimSuffix(todo.Name, ".md")
	depNames := make([]string, len(todo.DependsOn))
	for i, dep := range todo.DependsOn {
		depNames[i] = strings.TrimSuffix(dep, ".md")
	}

	fmt.Printf("Task: %s\n", taskName)
	fmt.Printf("Dependencies: %s\n", strings.Join(depNames, ", "))
	fmt.Println()
	fmt.Println("Dependency Results (would be injected as ANVIL_DEPENDENCY_RESULTS):")

	data, _ := json.MarshalIndent(depResults, "", "  ")
	fmt.Println(string(data))
}
