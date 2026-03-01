package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
	"gopkg.in/yaml.v3"
)

// taskSnapshotCmd implements the 'anvil task snapshot' command
func taskSnapshotCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil task snapshot <task-name> [--run <id>] [--file <filename>]\n")
		os.Exit(1)
	}

	taskName := args[0]
	var runID, fileName string

	// Parse flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--run":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --run requires a run ID\n")
				os.Exit(1)
			}
			i++
			runID = args[i]
		case "--file":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --file requires a filename\n")
				os.Exit(1)
			}
			i++
			fileName = args[i]
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	// Find the project root
	proj, err := project.Load(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load project: %v\n", err)
		os.Exit(1)
	}

	// Find the task by name to get its ID
	todos, err := proj.LoadTodos()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load tasks: %v\n", err)
		os.Exit(1)
	}

	var taskID string
	for _, todo := range todos {
		if todo.Name == taskName {
			taskID = todo.ID
			break
		}
	}

	if taskID == "" {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	// If no run ID specified, find the latest run
	if runID == "" {
		runs, err := project.ReadAllRunRecords(proj.Path, taskID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to list runs: %v\n", err)
			os.Exit(1)
		}
		if len(runs) == 0 {
			fmt.Fprintf(os.Stderr, "no runs found for task %s\n", taskName)
			os.Exit(1)
		}
		// Sort runs by timestamp and get the latest (they're already sorted newest first)
		runID = runs[0].RunID
	}

	// If specific file requested, just show that file
	if fileName != "" {
		err := showSnapshotFile(proj.Path, taskID, runID, fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Show the full snapshot
	err = showFullSnapshot(proj.Path, taskID, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func showSnapshotFile(projectPath, taskID, runID, fileName string) error {
	snapshotPath := project.GetSnapshotPath(projectPath, taskID, runID)
	filePath := filepath.Join(snapshotPath, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("snapshot file does not exist: %s", fileName)
	}

	// Read and display the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read snapshot file: %w", err)
	}

	// For JSON and YAML files, pretty-print them
	switch {
	case strings.HasSuffix(fileName, ".json"):
		var data interface{}
		if err := json.Unmarshal(content, &data); err == nil {
			pretty, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(pretty))
			return nil
		}
	case strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml"):
		var data interface{}
		if err := yaml.Unmarshal(content, &data); err == nil {
			pretty, _ := yaml.Marshal(data)
			fmt.Println(string(pretty))
			return nil
		}
	}

	// For other files, just print as-is
	fmt.Print(string(content))
	return nil
}

func showFullSnapshot(projectPath, taskID, runID string) error {
	snapshotPath := project.GetSnapshotPath(projectPath, taskID, runID)

	// Check if snapshot exists
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return fmt.Errorf("snapshot does not exist for run %s", runID)
	}

	fmt.Printf("Snapshot for task %s, run %s:\n\n", taskID, runID)

	// Show config.yaml
	fmt.Println("=== Task Configuration (config.yaml) ===")
	configContent, err := project.ReadSnapshotFile(projectPath, taskID, runID, "config.yaml")
	if err != nil {
		fmt.Printf("Error reading config: %v\n\n", err)
	} else {
		fmt.Println(string(configContent))
	}

	// Show env.yaml
	fmt.Println("\n=== Environment Variables (env.yaml) ===")
	envContent, err := project.ReadSnapshotFile(projectPath, taskID, runID, "env.yaml")
	if err != nil {
		fmt.Printf("Error reading env vars: %v\n\n", err)
	} else {
		fmt.Println(string(envContent))
	}

	// Show prompt.txt
	fmt.Println("\n=== Expanded Prompt (prompt.txt) ===")
	promptContent, err := project.ReadSnapshotFile(projectPath, taskID, runID, "prompt.txt")
	if err != nil {
		fmt.Printf("Error reading prompt: %v\n\n", err)
	} else {
		fmt.Println(string(promptContent))
	}

	// Show files.json
	fmt.Println("\n=== Directory Listing (files.json) ===")
	filesContent, err := project.ReadSnapshotFile(projectPath, taskID, runID, "files.json")
	if err != nil {
		fmt.Printf("Error reading file listing: %v\n\n", err)
	} else {
		fmt.Println(string(filesContent))
	}

	// Show run_record.json
	fmt.Println("\n=== Run Record (run_record.json) ===")
	runRecordContent, err := project.ReadSnapshotFile(projectPath, taskID, runID, "run_record.json")
	if err != nil {
		fmt.Printf("Error reading run record: %v\n\n", err)
	} else {
		fmt.Println(string(runRecordContent))
	}

	return nil
}