package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/config"
)

// Dependency represents a single dependency which may be local or cross-project.
type Dependency struct {
	Project string // empty means current project
	Task    string // task name (may include .md extension)
	IsLocal bool   // true if this is a local dependency (same project)
}

// ParseDependency parses a dependency string which may be in the format:
//   - "taskname" or "taskname.md" (local dependency)
//   - "project:taskname" or "project:taskname.md" (cross-project dependency)
func ParseDependency(dep string) Dependency {
	if strings.Contains(dep, ":") {
		parts := strings.SplitN(dep, ":", 2)
		return Dependency{
			Project: parts[0],
			Task:    parts[1],
			IsLocal: false,
		}
	}
	return Dependency{
		Project: "",
		Task:    dep,
		IsLocal: true,
	}
}

// ParseDependencies parses a list of dependency strings.
func ParseDependencies(deps []string) []Dependency {
	result := make([]Dependency, len(deps))
	for i, dep := range deps {
		result[i] = ParseDependency(dep)
	}
	return result
}

// ResolveTaskName resolves the task name to a full filename with .md extension.
func (d Dependency) ResolveTaskName() string {
	if strings.HasSuffix(d.Task, ".md") {
		return d.Task
	}
	return d.Task + ".md"
}

// GetProjectPath returns the absolute path to the project for this dependency.
// If IsLocal is true, returns the currentProjectPath.
// Otherwise, looks up the project in the watched directory.
func (d Dependency) GetProjectPath(currentProjectPath string) (string, error) {
	if d.IsLocal {
		return currentProjectPath, nil
	}

	// Look up project in watched directory
	watchedDir := filepath.Join(config.Dir(), "watched")
	entries, err := os.ReadDir(watchedDir)
	if err != nil {
		return "", fmt.Errorf("cannot read watched directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Check if directory name matches project name
		if entry.Name() == d.Project {
			return filepath.Join(watchedDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("project not found: %s", d.Project)
}

// ValidateDependency validates that a dependency is valid:
// - Local dependencies must exist in the project
// - Cross-project dependencies must reference an existing watched project
func ValidateDependency(dep Dependency, projectPath string) error {
	projectPathForDep, err := dep.GetProjectPath(projectPath)
	if err != nil {
		return err
	}

	// Check that the project directory exists
	if _, err := os.Stat(projectPathForDep); os.IsNotExist(err) {
		return fmt.Errorf("project not found: %s", dep.Project)
	}

	// For local dependencies, verify the task file exists
	if dep.IsLocal {
		todosDir := filepath.Join(projectPathForDep, ".anvil", "todos")
		taskFile := dep.ResolveTaskName()

		// Check in all priority directories p0-p9
		for pri := 0; pri <= 9; pri++ {
			taskPath := filepath.Join(todosDir, fmt.Sprintf("p%d", pri), taskFile)
			if _, err := os.Stat(taskPath); err == nil {
				return nil // Found
			}
		}
		return fmt.Errorf("task not found in project %s: %s", dep.Project, dep.Task)
	}

	return nil
}

// DependencyGraph represents a graph of task dependencies for cycle detection.
type DependencyGraph struct {
	edges map[string][]string // task -> dependencies
}

// NewDependencyGraph creates a new dependency graph from a list of tasks with their dependencies.
func NewDependencyGraph(todos []Todo) *DependencyGraph {
	g := &DependencyGraph{
		edges: make(map[string][]string),
	}

	for _, todo := range todos {
		taskName := todo.Name
		if !strings.HasSuffix(taskName, ".md") {
			taskName += ".md"
		}
		g.edges[taskName] = todo.DependsOn
	}

	return g
}

// HasCycle detects if there are any circular dependencies in the graph.
// Returns true if a cycle is found, and the cycle path if one exists.
func (g *DependencyGraph) HasCycle() (bool, []string) {
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)
	path := []string{}

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recursionStack[node] = true
		path = append(path, node)

		for _, dep := range g.edges[node] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recursionStack[dep] {
				// Found a cycle
				// Find where the cycle starts in path
				for i := len(path) - 1; i >= 0; i-- {
					if path[i] == dep {
						path = path[i:]
						return true
					}
				}
			}
		}

		path = path[:len(path)-1]
		recursionStack[node] = false
		return false
	}

	for node := range g.edges {
		if !visited[node] {
			if dfs(node) {
				return true, path
			}
		}
	}

	return false, nil
}

// DetectCrossProjectCycles checks for circular dependencies across multiple projects.
// Takes a map of projectName -> todos for each project.
func DetectCrossProjectCycles(projects map[string][]Todo) (bool, []string) {
	// Build combined graph with project prefixes
	combined := &DependencyGraph{
		edges: make(map[string][]string),
	}

	for projectName, todos := range projects {
		prefix := projectName
		if prefix == "" {
			prefix = "local"
		}

		for _, todo := range todos {
			// Use project:task format for all tasks
			taskKey := prefix + ":" + todo.Name
			var deps []string
			for _, dep := range todo.DependsOn {
				// Check if this is a cross-project dependency
				if strings.Contains(dep, ":") {
					deps = append(deps, dep) // Keep as-is
				} else {
					// Local dependency - add project prefix
					deps = append(deps, prefix+":"+dep)
				}
			}
			combined.edges[taskKey] = deps
		}
	}

	return combined.HasCycle()
}
