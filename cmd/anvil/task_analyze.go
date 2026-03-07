package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/johnjansen/anvil/internal/project"
)

func taskAnalyzeCmd(args []string) {
	var allProjects bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil task analyze [--all]

Analyze task schedules for potential conflicts and overlaps.

Options:
  --all    Analyze all watched projects

Examples:
  anvil task analyze
  anvil task analyze --all
`)
			os.Exit(0)
		case "--all":
			allProjects = true
		}
	}

	// Load projects to analyze
	var projects []*project.Project
	if allProjects {
		watched, err := loadAllWatched()
		if err != nil {
			log.Fatalf("failed to load watched projects: %v", err)
		}
		if len(watched) == 0 {
			fmt.Println("No watched projects")
			return
		}
		fmt.Printf("Analyzing %d watched project(s)...\n\n", len(watched))
		for _, w := range watched {
			proj, err := project.Load(w.Path)
			if err != nil {
				continue
			}
			projects = append(projects, proj)
		}
	} else {
		abs, err := filepath.Abs(".")
		if err != nil {
			log.Fatalf("bad path: %v", err)
		}
		proj, err := project.Load(abs)
		if err != nil {
			log.Fatalf("failed to load project: %v", err)
		}
		projects = append(projects, proj)
	}

	conflictCount := 0
	for _, proj := range projects {
		todos, err := proj.LoadTodos()
		if err != nil {
			log.Printf("failed to load todos from %s: %v", proj.Path, err)
			continue
		}

		// Check for conflicts
		conflicts := detectConflicts(todos)
		fmt.Printf("Checking %s...\n", proj.Path)
		if len(conflicts) > 0 {
			for _, c := range conflicts {
				conflictCount++
				fmt.Printf("WARNING: %s and %s may overlap\n", c.Task1, c.Task2)
				fmt.Printf("  - %s: %s\n", c.Task1, c.Schedule1)
				fmt.Printf("  - %s: %s\n", c.Task2, c.Schedule2)
				if c.Reason != "" {
					fmt.Printf("  %s\n", c.Reason)
				}
				fmt.Println()
			}
		} else {
			fmt.Println("OK: No schedule conflicts detected")
			fmt.Println()
		}
	}

	if conflictCount == 0 {
		fmt.Println("All schedules look good!")
	} else {
		log.Fatalf("Found %d scheduling conflict(s)", conflictCount)
	}
}
