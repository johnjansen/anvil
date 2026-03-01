package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

// taskSnapshotDiffCmd implements the 'anvil task snapshot-diff' command
func taskSnapshotDiffCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil task snapshot-diff <task-name> --run1 <id1> --run2 <id2>\n")
		os.Exit(1)
	}

	taskName := args[0]
	var runID1, runID2 string

	// Parse flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--run1":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --run1 requires a run ID\n")
				os.Exit(1)
			}
			i++
			runID1 = args[i]
		case "--run2":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --run2 requires a run ID\n")
				os.Exit(1)
			}
			i++
			runID2 = args[i]
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if runID1 == "" || runID2 == "" {
		fmt.Fprintf(os.Stderr, "error: both --run1 and --run2 are required\n")
		os.Exit(1)
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

	// Compare the two snapshots
	err = compareSnapshots(proj.Path, taskID, runID1, runID2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func compareSnapshots(projectPath, taskID, runID1, runID2 string) error {
	// Read both snapshots
	snapshot1Path := project.GetSnapshotPath(projectPath, taskID, runID1)
	snapshot2Path := project.GetSnapshotPath(projectPath, taskID, runID2)

	// Check if snapshots exist
	if _, err := os.Stat(snapshot1Path); os.IsNotExist(err) {
		return fmt.Errorf("snapshot does not exist for run %s", runID1)
	}
	if _, err := os.Stat(snapshot2Path); os.IsNotExist(err) {
		return fmt.Errorf("snapshot does not exist for run %s", runID2)
	}

	fmt.Printf("Comparing snapshots for task %s:\n", taskID)
	fmt.Printf("Run 1: %s\n", runID1)
	fmt.Printf("Run 2: %s\n", runID2)
	fmt.Println()

	// Compare config.yaml
	fmt.Println("=== Task Configuration (config.yaml) ===")
	config1, err1 := project.ReadSnapshotFile(projectPath, taskID, runID1, "config.yaml")
	config2, err2 := project.ReadSnapshotFile(projectPath, taskID, runID2, "config.yaml")
	if err1 != nil || err2 != nil {
		fmt.Printf("Error reading configs: %v, %v\n\n", err1, err2)
	} else {
		diffStrings(string(config1), string(config2), "config.yaml")
	}

	// Compare env.yaml
	fmt.Println("\n=== Environment Variables (env.yaml) ===")
	env1, err1 := project.ReadSnapshotFile(projectPath, taskID, runID1, "env.yaml")
	env2, err2 := project.ReadSnapshotFile(projectPath, taskID, runID2, "env.yaml")
	if err1 != nil || err2 != nil {
		fmt.Printf("Error reading env vars: %v, %v\n\n", err1, err2)
	} else {
		diffStrings(string(env1), string(env2), "env.yaml")
	}

	// Compare prompt.txt
	fmt.Println("\n=== Expanded Prompt (prompt.txt) ===")
	prompt1, err1 := project.ReadSnapshotFile(projectPath, taskID, runID1, "prompt.txt")
	prompt2, err2 := project.ReadSnapshotFile(projectPath, taskID, runID2, "prompt.txt")
	if err1 != nil || err2 != nil {
		fmt.Printf("Error reading prompts: %v, %v\n\n", err1, err2)
	} else {
		diffStrings(string(prompt1), string(prompt2), "prompt.txt")
	}

	// Compare run_record.json
	fmt.Println("\n=== Run Record (run_record.json) ===")
	record1, err1 := project.ReadSnapshotFile(projectPath, taskID, runID1, "run_record.json")
	record2, err2 := project.ReadSnapshotFile(projectPath, taskID, runID2, "run_record.json")
	if err1 != nil || err2 != nil {
		fmt.Printf("Error reading run records: %v, %v\n\n", err1, err2)
	} else {
		diffStrings(string(record1), string(record2), "run_record.json")
	}

	return nil
}

func diffStrings(s1, s2, filename string) {
	if s1 == s2 {
		fmt.Printf("No differences in %s\n", filename)
		return
	}

	// Split into lines for comparison
	lines1 := strings.Split(s1, "\n")
	lines2 := strings.Split(s2, "\n")

	// Simple line-by-line diff
	maxLen := len(lines1)
	if len(lines2) > maxLen {
		maxLen = len(lines2)
	}

	fmt.Printf("--- %s ---\n", filename)
	for i := 0; i < maxLen; i++ {
		var line1, line2 string
		if i < len(lines1) {
			line1 = lines1[i]
		}
		if i < len(lines2) {
			line2 = lines2[i]
		}

		if line1 != line2 {
			if line1 == "" {
				fmt.Printf("+ %s\n", line2)
			} else if line2 == "" {
				fmt.Printf("- %s\n", line1)
			} else {
				fmt.Printf("- %s\n", line1)
				fmt.Printf("+ %s\n", line2)
			}
		}
	}
}