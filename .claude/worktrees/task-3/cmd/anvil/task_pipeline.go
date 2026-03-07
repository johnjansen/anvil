package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

func taskPipelineCmd(args []string) {
	var allProjects bool
	var dotOutput bool
	var verbose bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil task pipeline [--dot] [--verbose] [--all]

Visualize task dependency pipelines.

Options:
  --dot      Output in GraphViz DOT format
  --verbose  Show schedules and last run status
  --all      Show pipelines across all watched projects

Examples:
  anvil task pipeline
  anvil task pipeline --dot > pipeline.dot
  anvil task pipeline --verbose
  anvil task pipeline --all
`)
			os.Exit(0)
		case "--all":
			allProjects = true
		case "--dot":
			dotOutput = true
		case "--verbose":
			verbose = true
		}
	}

	// Load projects
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

	if dotOutput {
		pipelineDOT(projects, allProjects)
	} else {
		pipelineASCII(projects, allProjects, verbose)
	}
}

// pipelineTaskInfo holds dependency graph info for a single task.
type pipelineTaskInfo struct {
	name      string
	schedule  string
	dependsOn []string
	projPath  string
	taskID    string
}

// buildPipelineGraph builds an adjacency list (parent->children) and collects task info.
// Returns tasks map, children adjacency, and any errors (cycles, missing deps).
func buildPipelineGraph(projects []*project.Project) (map[string]*pipelineTaskInfo, map[string][]string, []string) {
	tasks := make(map[string]*pipelineTaskInfo)
	children := make(map[string][]string) // parent -> list of children that depend on it
	var warnings []string

	for _, proj := range projects {
		todos, err := proj.LoadTodos()
		if err != nil {
			continue
		}
		for _, t := range todos {
			name := strings.TrimSuffix(t.Name, ".md")
			tasks[name] = &pipelineTaskInfo{
				name:      name,
				schedule:  t.Schedule,
				dependsOn: t.DependsOn,
				projPath:  proj.Path,
				taskID:    t.ID,
			}
		}
	}

	// Build children map and validate deps
	for name, info := range tasks {
		for _, dep := range info.dependsOn {
			depName := strings.TrimSuffix(dep, ".md")
			if _, exists := tasks[depName]; !exists {
				warnings = append(warnings, fmt.Sprintf("WARNING: %s depends on %q which does not exist", name, depName))
				continue
			}
			children[depName] = append(children[depName], name)
		}
	}

	// Detect cycles using DFS with coloring
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully processed
	)
	color := make(map[string]int)
	var cyclePath []string
	var hasCycle bool

	var dfs func(node string)
	dfs = func(node string) {
		if hasCycle {
			return
		}
		color[node] = gray
		cyclePath = append(cyclePath, node)
		for _, child := range children[node] {
			if color[child] == gray {
				// Found cycle - extract the cycle portion
				hasCycle = true
				cycleStart := -1
				for i, n := range cyclePath {
					if n == child {
						cycleStart = i
						break
					}
				}
				cycle := append(cyclePath[cycleStart:], child)
				warnings = append(warnings, fmt.Sprintf("ERROR: Circular dependency detected: %s", strings.Join(cycle, " -> ")))
				return
			}
			if color[child] == white {
				dfs(child)
			}
		}
		cyclePath = cyclePath[:len(cyclePath)-1]
		color[node] = black
	}

	for name := range tasks {
		if color[name] == white {
			dfs(name)
		}
	}

	return tasks, children, warnings
}

// pipelineASCII renders a tree view of task dependency pipelines.
func pipelineASCII(projects []*project.Project, showProjectPrefix bool, verbose bool) {
	tasks, children, warnings := buildPipelineGraph(projects)

	// Print warnings first
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, w)
	}

	// Find root tasks (tasks with no dependencies)
	roots := []string{}
	for name, info := range tasks {
		if len(info.dependsOn) == 0 {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)

	// Find tasks that are part of pipelines (have deps or are depended upon)
	inPipeline := make(map[string]bool)
	for name, info := range tasks {
		if len(info.dependsOn) > 0 {
			inPipeline[name] = true
			for _, dep := range info.dependsOn {
				inPipeline[strings.TrimSuffix(dep, ".md")] = true
			}
		}
		if len(children[name]) > 0 {
			inPipeline[name] = true
		}
	}

	// Only show tasks that are part of pipelines
	pipelineRoots := []string{}
	for _, r := range roots {
		if inPipeline[r] {
			pipelineRoots = append(pipelineRoots, r)
		}
	}

	if len(pipelineRoots) == 0 {
		fmt.Println("No task dependencies found")
		return
	}

	// Render each pipeline tree
	var printTree func(name string, prefix string, isLast bool)
	printTree = func(name string, prefix string, isLast bool) {
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		label := name
		if verbose {
			info := tasks[name]
			if info != nil {
				label = formatVerboseLabel(info)
			}
		}

		if prefix == "" {
			fmt.Println(label)
		} else {
			fmt.Println(prefix + connector + label)
		}

		childPrefix := prefix
		if prefix != "" {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
		}

		kids := children[name]
		sort.Strings(kids)
		for i, child := range kids {
			printTree(child, childPrefix, i == len(kids)-1)
		}
	}

	for i, root := range pipelineRoots {
		if i > 0 {
			fmt.Println()
		}
		printTree(root, "", true)
	}
}

// formatVerboseLabel formats a task name with schedule and status info.
func formatVerboseLabel(info *pipelineTaskInfo) string {
	label := info.name
	if info.schedule != "" {
		label += " [" + info.schedule + "]"
	}

	// Try to get last run status (try UUID first, then task name)
	rec, err := project.ReadCurrentRunRecord(info.projPath, info.taskID)
	if err != nil {
		// Fall back to task name (with .md suffix) as some runs are keyed by name
		rec, err = project.ReadCurrentRunRecord(info.projPath, info.name+".md")
	}
	if err == nil {
		if rec.Success {
			label += " ✓ success"
		} else {
			label += " ✗ failed"
		}
	} else {
		label += " - no runs"
	}

	return label
}

// pipelineDOT outputs the dependency graph in GraphViz DOT format.
func pipelineDOT(projects []*project.Project, showProjectPrefix bool) {
	tasks, _, warnings := buildPipelineGraph(projects)

	// Print warnings to stderr
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, w)
	}

	fmt.Println("digraph pipeline {")
	fmt.Println("  rankdir=LR;")
	fmt.Println("  node [shape=box, style=rounded];")

	// Emit nodes
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		info := tasks[name]
		if len(info.dependsOn) == 0 && len(info.schedule) > 0 {
			// No deps, has schedule — just a standalone task, only include if depended upon
			hasDependents := false
			for _, other := range tasks {
				for _, dep := range other.dependsOn {
					if strings.TrimSuffix(dep, ".md") == name {
						hasDependents = true
						break
					}
				}
				if hasDependents {
					break
				}
			}
			if !hasDependents {
				continue
			}
		}
		label := name
		if info.schedule != "" {
			label += "\\n" + info.schedule
		}
		fmt.Printf("  %q [label=%q];\n", name, label)
	}

	// Emit edges (dependency -> dependent)
	for _, name := range names {
		info := tasks[name]
		for _, dep := range info.dependsOn {
			depName := strings.TrimSuffix(dep, ".md")
			if _, exists := tasks[depName]; exists {
				fmt.Printf("  %q -> %q;\n", depName, name)
			}
		}
	}

	fmt.Println("}")
}
