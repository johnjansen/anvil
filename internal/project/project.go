package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the project-level config from <project>/.anvil/anvil.yaml
type Config struct {
	Schedule string `yaml:"schedule"`
	Priority int    `yaml:"priority"`
}

// Project represents a watched project directory
type Project struct {
	Path   string
	Config Config
}

// Todo is a single todo file from the project's .anvil/todos/ tree
type Todo struct {
	Path     string // absolute path to the file
	Name     string // filename
	Priority int    // 0-9, from pN/ directory
	Content  string // file contents
}

// Load reads a project's .anvil/anvil.yaml and returns a Project
func Load(path string) (*Project, error) {
	configPath := filepath.Join(path, ".anvil", "anvil.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading project config %s: %w", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing project config: %w", err)
	}

	return &Project{Path: path, Config: cfg}, nil
}

// LoadTodos returns all todo files sorted by priority (p0 first) then by name (oldest first)
func (p *Project) LoadTodos() ([]Todo, error) {
	todosDir := filepath.Join(p.Path, ".anvil", "todos")
	var todos []Todo

	for pri := 0; pri <= 9; pri++ {
		dir := filepath.Join(todosDir, fmt.Sprintf("p%d", pri))
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading todos p%d: %w", pri, err)
		}

		// Sort by name so oldest-timestamped files come first
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			fp := filepath.Join(dir, e.Name())
			content, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			todos = append(todos, Todo{
				Path:     fp,
				Name:     e.Name(),
				Priority: pri,
				Content:  string(content),
			})
		}
	}

	return todos, nil
}

// RemoveTodo deletes a todo file from disk
func RemoveTodo(todo Todo) error {
	return os.Remove(todo.Path)
}

// Init creates the .anvil/ directory structure for a project
func Init(path string, schedule string, priority int) error {
	anvilDir := filepath.Join(path, ".anvil")
	if err := os.MkdirAll(anvilDir, 0755); err != nil {
		return fmt.Errorf("creating .anvil: %w", err)
	}

	// Create priority directories
	for i := 0; i <= 9; i++ {
		dir := filepath.Join(anvilDir, "todos", fmt.Sprintf("p%d", i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating todos/p%d: %w", i, err)
		}
	}

	// Write config
	cfg := Config{Schedule: schedule, Priority: priority}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(filepath.Join(anvilDir, "anvil.yaml"), data, 0644)
}
