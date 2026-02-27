package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// GetTaskTree returns the parent run record and its children for a given task.
func GetTaskTree(projectPath, taskID string) (parent RunRecord, children []RunRecord, err error) {
	// Get all run records for this task
	records, err := ReadAllRunRecords(projectPath, taskID)
	if err != nil {
		return parent, nil, fmt.Errorf("failed to read run records: %w", err)
	}

	// Find parent runs (no ParentID) - most recent first
	for _, r := range records {
		if r.ParentID == "" {
			parent = r
			break
		}
	}

	if parent.RunID == "" {
		return parent, nil, nil // No runs yet
	}

	// Find children
	for _, r := range records {
		if r.ParentID == parent.RunID {
			children = append(children, r)
		}
	}

	// Sort children by start time
	sort.Slice(children, func(i, j int) bool {
		return children[i].Started.Before(children[j].Started)
	})

	return parent, children, nil
}

// GetChildTasks resolves spawn task names to Todo objects.
func GetChildTasks(projectPath string, spawnNames []string) ([]*Todo, error) {
	proj, err := Load(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load project: %w", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		return nil, fmt.Errorf("failed to load todos: %w", err)
	}

	// Build name lookup
	todoByName := make(map[string]*Todo)
	for i := range todos {
		todoByName[todos[i].Name] = &todos[i]
	}

	var children []*Todo
	for _, name := range spawnNames {
		child, ok := todoByName[name]
		if !ok {
			return nil, fmt.Errorf("spawn task not found: %s", name)
		}
		children = append(children, child)
	}

	return children, nil
}

// GetSpawnedChildren returns all child run records for a given parent run.
func GetSpawnedChildren(projectPath string, parentRunID string) ([]RunRecord, error) {
	// This is a simplified version - in practice we'd want to look up by TaskID too
	// For now, scan all run records
	runsDir := filepath.Join(projectPath, ".anvil", "runs")

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, nil // No runs yet
	}

	var children []RunRecord
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runPath := filepath.Join(runsDir, entry.Name(), "current.json")
		data, err := os.ReadFile(runPath)
		if err != nil {
			continue
		}

		var rec RunRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}

		if rec.ParentID == parentRunID {
			children = append(children, rec)
		}
	}

	return children, nil
}
