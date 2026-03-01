package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// taskSnapshotCmd represents the snapshot command
var taskSnapshotCmd = &cobra.Command{
	Use:   "snapshot [task-name]",
	Short: "View task execution snapshots for debugging",
	Long: `View task execution snapshots to debug failed runs.

Snapshots capture the complete execution context including:
- Task configuration (frontmatter)
- Resolved environment variables
- Expanded prompt
- Directory listing at start
- Run metadata

Examples:
  anvil task snapshot my-task          # View latest snapshot
  anvil task snapshot my-task --run abc123  # View specific run
  anvil task snapshot my-task --file prompt.txt  # View specific file`,
	Args: cobra.ExactArgs(1),
	RunE: taskSnapshotRun,
}

func init() {
	taskSnapshotCmd.Flags().String("run", "", "View snapshot for specific run ID")
	taskSnapshotCmd.Flags().String("file", "", "View specific file from snapshot")
	taskCmd.AddCommand(taskSnapshotCmd)
}

func taskSnapshotRun(cmd *cobra.Command, args []string) error {
	taskName := args[0]
	runID, _ := cmd.Flags().GetString("run")
	fileName, _ := cmd.Flags().GetString("file")

	// Find the project root
	proj, err := project.LoadFromCwd()
	if err != nil {
		return fmt.Errorf("failed to load project: %w", err)
	}

	// Find the task by name to get its ID
	todo, err := proj.LoadTodoByName(taskName)
	if err != nil {
		return fmt.Errorf("task not found: %s", taskName)
	}

	taskID := todo.ID

	// If no run ID specified, find the latest run
	if runID == "" {
		runs, err := project.ListRuns(proj.Path, taskID)
		if err != nil {
			return fmt.Errorf("failed to list runs: %w", err)
		}
		if len(runs) == 0 {
			return fmt.Errorf("no runs found for task %s", taskName)
		}
		// Sort runs by timestamp and get the latest
		runID = runs[len(runs)-1].RunID
	}

	// If specific file requested, just show that file
	if fileName != "" {
		return showSnapshotFile(proj.Path, taskID, runID, fileName)
	}

	// Show the full snapshot
	return showFullSnapshot(proj.Path, taskID, runID)
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