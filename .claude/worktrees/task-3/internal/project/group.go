package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Group represents a task group configuration
type Group struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Tasks       []string `yaml:"tasks"`
	Execution   string   `yaml:"execution"` // "parallel" or "sequential"
	OnFailure   string   `yaml:"on_failure,omitempty"` // "continue", "stop"
	Schedule    string   `yaml:"schedule,omitempty"`
	Path        string   `yaml:"-"`
}

// LoadGroup loads a group configuration from a YAML file
func LoadGroup(path string) (*Group, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read group file: %w", err)
	}

	var group Group
	if err := yaml.Unmarshal(data, &group); err != nil {
		return nil, fmt.Errorf("failed to parse group YAML: %w", err)
	}

	// Set default values
	if group.Execution == "" {
		group.Execution = "sequential"
	}
	if group.OnFailure == "" {
		group.OnFailure = "stop"
	}

	// Validate execution mode
	if group.Execution != "parallel" && group.Execution != "sequential" {
		return nil, fmt.Errorf("invalid execution mode: %s (must be 'parallel' or 'sequential')", group.Execution)
	}

	// Validate on_failure mode
	if group.OnFailure != "continue" && group.OnFailure != "stop" {
		return nil, fmt.Errorf("invalid on_failure mode: %s (must be 'continue' or 'stop')", group.OnFailure)
	}

	group.Path = path
	return &group, nil
}

// LoadGroups loads all group configurations from the .anvil/groups directory
func (p *Project) LoadGroups() ([]Group, error) {
	groupsDir := filepath.Join(p.Path, ".anvil", "groups")

	// Check if groups directory exists
	if _, err := os.Stat(groupsDir); os.IsNotExist(err) {
		return []Group{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat groups directory: %w", err)
	}

	// Read directory entries
	entries, err := os.ReadDir(groupsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read groups directory: %w", err)
	}

	var groups []Group
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .yaml files
		if filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml" {
			continue
		}

		path := filepath.Join(groupsDir, entry.Name())
		group, err := LoadGroup(path)
		if err != nil {
			// Log error but continue loading other groups
			fmt.Printf("Warning: failed to load group from %s: %v\n", path, err)
			continue
		}

		groups = append(groups, *group)
	}

	// Sort groups by name for consistent ordering
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups, nil
}