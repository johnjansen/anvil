package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

func updateTaskDisabledStatus(projectPath string, todo *project.Todo, disabled bool) error {
	// Read the current task file
	taskPath := filepath.Join(projectPath, todo.Name)
	content, err := os.ReadFile(taskPath)
	if err != nil {
		return fmt.Errorf("reading task file: %w", err)
	}

	// Parse the YAML frontmatter and content
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) < 3 {
		return fmt.Errorf("invalid task file format")
	}

	frontmatter := parts[1]
	taskContent := parts[2]

	// Parse the frontmatter to modify the disabled field
	lines := strings.Split(frontmatter, "\n")
	var newLines []string
	disabledLineFound := false

	for _, line := range lines {
		if strings.HasPrefix(line, "disabled:") {
			// Replace the disabled line
			newLines = append(newLines, fmt.Sprintf("disabled: %t", disabled))
			disabledLineFound = true
		} else {
			newLines = append(newLines, line)
		}
	}

	// If disabled line wasn't found, add it
	if !disabledLineFound {
		// Insert the disabled line before the closing --- if possible
		if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
			// Remove the last empty line
			newLines = newLines[:len(newLines)-1]
		}
		newLines = append(newLines, fmt.Sprintf("disabled: %t", disabled))
	}

	// Reconstruct the frontmatter
	newFrontmatter := strings.Join(newLines, "\n")

	// Reconstruct the full file content
	newContent := fmt.Sprintf("---\n%s\n---%s", newFrontmatter, taskContent)

	// Write the updated content back to the file
	return os.WriteFile(taskPath, []byte(newContent), 0644)
}

// groupsCmd shows the status of concurrency groups
