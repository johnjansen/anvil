package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
	"gopkg.in/yaml.v3"
)

func taskEditApply(todo *project.Todo, projectRoot string, newSchedule *string, newPriority *int, setDisabled *bool) error {
	raw, err := os.ReadFile(todo.Path)
	if err != nil {
		return fmt.Errorf("read task file: %w", err)
	}

	contentStr := string(raw)
	if !strings.HasPrefix(contentStr, "---\n") {
		return fmt.Errorf("task %s has no front-matter", todo.Name)
	}
	parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
	if len(parts) != 2 {
		return fmt.Errorf("failed to parse front-matter for %s", todo.Name)
	}

	var fmMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(parts[0]), &fmMap); err != nil {
		return fmt.Errorf("parse front-matter for %s: %w", todo.Name, err)
	}

	changes := []string{}

	if newSchedule != nil {
		fmMap["schedule"] = *newSchedule
		changes = append(changes, fmt.Sprintf("schedule: %s", *newSchedule))
	}

	if setDisabled != nil {
		if *setDisabled {
			fmMap["disabled"] = true
		} else {
			delete(fmMap, "disabled")
		}
		changes = append(changes, fmt.Sprintf("disabled: %t", *setDisabled))
	}

	priority := todo.Priority
	if newPriority != nil {
		priority = *newPriority
	}

	fmBytes, err := yaml.Marshal(fmMap)
	if err != nil {
		return fmt.Errorf("marshal front-matter for %s: %w", todo.Name, err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(string(fmBytes))
	sb.WriteString("---\n")
	sb.WriteString(parts[1])

	if priority != todo.Priority {
		newDir := filepath.Join(projectRoot, ".anvil", "todos", fmt.Sprintf("p%d", priority))
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return fmt.Errorf("create priority directory: %w", err)
		}
		newPath := filepath.Join(newDir, filepath.Base(todo.Path))
		if err := os.WriteFile(newPath, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("write task file: %w", err)
		}
		if err := os.Remove(todo.Path); err != nil {
			return fmt.Errorf("remove old task file: %w", err)
		}
		changes = append(changes, fmt.Sprintf("priority: p%d -> p%d", todo.Priority, priority))
	} else {
		if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("write task file: %w", err)
		}
	}

	fmt.Printf("  %s: %s\n", todo.Name, strings.Join(changes, ", "))
	return nil
}

func findTodo(todos []project.Todo, name string) *project.Todo {
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	for i := range todos {
		if todos[i].Name == name {
			return &todos[i]
		}
	}
	return nil
}
