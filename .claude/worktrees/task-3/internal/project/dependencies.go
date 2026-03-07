package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/johnjansen/anvil/internal/config"
	"gopkg.in/yaml.v3"
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

// watchedFrontmatter is the YAML frontmatter in watched project entry files.
type watchedFrontmatter struct {
	Path string `yaml:"path"`
}

// GetProjectPath returns the absolute path to the project for this dependency.
// If IsLocal is true, returns the currentProjectPath.
// Otherwise, looks up the project in the watched directory and resolves its
// actual filesystem path from the watched entry's frontmatter.
func (d Dependency) GetProjectPath(currentProjectPath string) (string, error) {
	if d.IsLocal {
		return currentProjectPath, nil
	}

	return ResolveWatchedProjectPath(d.Project)
}

// ResolveWatchedProjectPath looks up a project by name in ~/.anvil/watched/
// and returns the actual filesystem path from the watched entry's frontmatter.
func ResolveWatchedProjectPath(projectName string) (string, error) {
	watchedDir := filepath.Join(config.Dir(), "watched")
	entries, err := os.ReadDir(watchedDir)
	if err != nil {
		return "", fmt.Errorf("cannot read watched directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() != projectName {
			continue
		}

		// Read the latest .md file from the project's watched directory
		dirPath := filepath.Join(watchedDir, entry.Name())
		mdEntries, err := os.ReadDir(dirPath)
		if err != nil {
			return "", fmt.Errorf("cannot read watched project directory %s: %w", projectName, err)
		}

		// Sort descending to get latest file first (same as loadWatchedPaths)
		sort.Slice(mdEntries, func(i, j int) bool {
			return mdEntries[i].Name() > mdEntries[j].Name()
		})

		for _, e := range mdEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
			if err != nil {
				return "", fmt.Errorf("cannot read watched entry for %s: %w", projectName, err)
			}

			p := parseWatchedProjectPath(string(data))
			if p != "" {
				return p, nil
			}
			break
		}

		return "", fmt.Errorf("project %s has no valid watched entry", projectName)
	}

	return "", fmt.Errorf("project not found in watched directory: %s", projectName)
}

// parseWatchedProjectPath extracts the path field from a watched entry's YAML frontmatter.
func parseWatchedProjectPath(content string) string {
	start := strings.Index(content, "---\n")
	if start == -1 {
		return ""
	}
	end := strings.Index(content[start+4:], "\n---")
	if end == -1 {
		return ""
	}

	var fm watchedFrontmatter
	if err := yaml.Unmarshal([]byte(content[start+4:start+4+end]), &fm); err != nil {
		return ""
	}
	return fm.Path
}

// ValidateDependency validates that a dependency is valid:
// - The referenced project must exist (for cross-project deps, must be in watched directory)
// - The referenced task file must exist in the project's todos directory
func ValidateDependency(dep Dependency, projectPath string) error {
	projectPathForDep, err := dep.GetProjectPath(projectPath)
	if err != nil {
		return err
	}

	// Check that the project directory exists
	if _, err := os.Stat(projectPathForDep); os.IsNotExist(err) {
		if dep.IsLocal {
			return fmt.Errorf("project path not found: %s", projectPathForDep)
		}
		return fmt.Errorf("project not found: %s (path: %s)", dep.Project, projectPathForDep)
	}

	// Verify the task file exists in the project's todos directory
	todosDir := filepath.Join(projectPathForDep, ".anvil", "todos")
	taskFile := dep.ResolveTaskName()

	for pri := 0; pri <= 9; pri++ {
		taskPath := filepath.Join(todosDir, fmt.Sprintf("p%d", pri), taskFile)
		if _, err := os.Stat(taskPath); err == nil {
			return nil // Found
		}
	}

	projectLabel := "local"
	if !dep.IsLocal {
		projectLabel = dep.Project
	}
	return fmt.Errorf("task not found in project %s: %s", projectLabel, dep.Task)
}

// FindTaskIDByName loads a project's todos and returns the task ID (UUID) for the given task name.
// The task name may or may not have the .md extension.
func FindTaskIDByName(projectPath string, taskName string) (string, error) {
	proj, err := Load(projectPath)
	if err != nil {
		return "", fmt.Errorf("loading project %s: %w", projectPath, err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		return "", fmt.Errorf("loading todos from %s: %w", projectPath, err)
	}

	// Normalize: ensure we compare with .md extension
	if !strings.HasSuffix(taskName, ".md") {
		taskName += ".md"
	}

	for _, todo := range todos {
		name := todo.Name
		if !strings.HasSuffix(name, ".md") {
			name += ".md"
		}
		if name == taskName {
			if todo.ID == "" {
				return "", fmt.Errorf("task %s has no ID in project %s", taskName, projectPath)
			}
			return todo.ID, nil
		}
	}

	return "", fmt.Errorf("task %s not found in project %s", taskName, projectPath)
}

// ResolveDependencyRunRecord resolves a dependency string (local or cross-project) to a RunRecord.
// For cross-project deps (project:task), it resolves the project path and task ID.
// For local deps, it resolves the task ID within the current project.
func ResolveDependencyRunRecord(currentProjectPath string, dep string) (RunRecord, error) {
	parsed := ParseDependency(dep)

	projectPath, err := parsed.GetProjectPath(currentProjectPath)
	if err != nil {
		return RunRecord{}, fmt.Errorf("resolving project for dependency %s: %w", dep, err)
	}

	taskName := parsed.ResolveTaskName()
	taskID, err := FindTaskIDByName(projectPath, taskName)
	if err != nil {
		return RunRecord{}, fmt.Errorf("resolving task ID for dependency %s: %w", dep, err)
	}

	return ReadCurrentRunRecord(projectPath, taskID)
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
