package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/tools"
)

// Use blank imports to ensure packages are available for command functions
// that will be moved to this file in subsequent tasks
var (
	_ = config.Load
	_ = daemon.New
	_ = project.Load
	_ = fmt.Sprintf
	_ = os.Exit
	_ = strings.TrimSpace
	_ = time.Now
)

func projectCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil project <subcommand> [options]\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		projectCreateCmd(args[1:])
	case "ls":
		projectLsCmd(args[1:])
	case "get":
		projectGetCmd(args[1:])
	case "rm":
		projectRmCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown project command: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}
}

func projectCreateCmd(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Initialize project .anvil/ structure (preserves existing tasks)
	result, err := project.Init(abs, tools.FS, false)
	if err != nil {
		log.Fatalf("failed to init project: %v", err)
	}
	if result.AlreadyExists {
		fmt.Printf("existing project with %d task(s) preserved\n", result.TaskCount)
	}

	fmt.Printf("created %s\n", abs)

	// Register with daemon (watch)
	registerProject(abs)
}

func projectRmCmd(args []string) {
	path := "."
	clean := false

	var filtered []string
	for _, a := range args {
		switch a {
		case "--clean":
			clean = true
		default:
			filtered = append(filtered, a)
		}
	}
	if len(filtered) > 0 {
		path = filtered[0]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	hash := projectHash(abs)
	watchDir := filepath.Join(config.WatchedDir(), hash)

	if _, err := os.Stat(watchDir); os.IsNotExist(err) {
		fmt.Printf("not watching %s\n", abs)
		return
	}

	if err := os.RemoveAll(watchDir); err != nil {
		log.Fatalf("failed to unwatch: %v", err)
	}

	fmt.Printf("unwatched %s\n", abs)

	if clean {
		anvilDir := filepath.Join(abs, ".anvil")
		if _, err := os.Stat(anvilDir); os.IsNotExist(err) {
			return
		}
		if err := os.RemoveAll(anvilDir); err != nil {
			log.Fatalf("failed to clean .anvil/: %v", err)
		}
		fmt.Printf("removed %s\n", anvilDir)
	}
}

func projectLsCmd(args []string) {
	allProjects := false
	jsonOutput := false
	for _, a := range args {
		if a == "--all" || a == "-a" {
			allProjects = true
		}
		if a == "--json" {
			jsonOutput = true
		}
	}

	watched, err := loadAllWatched()
	if err != nil {
		log.Fatalf("failed to read watched: %v", err)
	}

	if !allProjects {
		// Scope to current directory
		abs, err := filepath.Abs(".")
		if err != nil {
			log.Fatalf("bad path: %v", err)
		}
		var filtered []watchFrontmatter
		for _, w := range watched {
			if w.Path == abs || strings.HasPrefix(w.Path, abs+"/") {
				filtered = append(filtered, w)
			}
		}
		watched = filtered
	}

	if len(watched) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("no watched projects")
		}
		return
	}

	// Get running tasks from daemon
	var runningTasks []daemon.TaskInfo
	if daemon.IsDaemonRunning() {
		runningTasks, _ = daemon.SendPsRequest()
	}

	// Count running tasks per project
	runningByProject := make(map[string]int)
	for _, t := range runningTasks {
		runningByProject[t.Project]++
	}

	if jsonOutput {
		type projectJSON struct {
			Path      string `json:"path"`
			Tasks     int    `json:"tasks"`
			Running   int    `json:"running"`
			Status    string `json:"status"`
			WatchedAt string `json:"watched_at,omitempty"`
		}
		var items []projectJSON
		for _, w := range watched {
			todoCount := 0
			status := "idle"

			proj, err := project.Load(w.Path)
			if err != nil {
				items = append(items, projectJSON{
					Path:      w.Path,
					Tasks:     0,
					Status:    fmt.Sprintf("error: %v", err),
					WatchedAt: w.WatchedAt.Format(time.RFC3339),
				})
				continue
			}

			todos, _ := proj.LoadTodos()
			todoCount = len(todos)

			running := runningByProject[w.Path]
			if running > 0 {
				status = "busy"
			}

			items = append(items, projectJSON{
				Path:      w.Path,
				Tasks:     todoCount,
				Running:   running,
				Status:    status,
				WatchedAt: w.WatchedAt.Format(time.RFC3339),
			})
		}
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Print header
	fmt.Printf("%-50s %-8s %s\n", "PATH", "TASKS", "STATUS")
	fmt.Printf("%s\n", strings.Repeat("-", 70))

	for _, w := range watched {
		todoCount := 0
		status := "idle"

		proj, err := project.Load(w.Path)
		if err != nil {
			fmt.Printf("%-50s %-8s %s\n", truncate(w.Path, 50), "?", fmt.Sprintf("error: %v", err))
			continue
		}

		todos, _ := proj.LoadTodos()
		todoCount = len(todos)

		if n := runningByProject[w.Path]; n > 0 {
			status = fmt.Sprintf("busy (%d running)", n)
		}

		fmt.Printf("%-50s %-8d %s\n", truncate(w.Path, 50), todoCount, status)
	}
}

func projectGetCmd(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Check if project is initialized
	if _, err := os.Stat(filepath.Join(abs, ".anvil", "todos")); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "not an anvil project: %s\n", abs)
		os.Exit(1)
	}

	// Check watch status
	watched := "no"
	hash := projectHash(abs)
	watchDir := filepath.Join(config.WatchedDir(), hash)
	if entries, err := os.ReadDir(watchDir); err == nil && len(entries) > 0 {
		watched = "yes"
	}

	// Load todos and count by priority
	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	priorityCounts := make(map[int]int)
	for _, t := range todos {
		priorityCounts[t.Priority]++
	}

	// Print project details
	fmt.Printf("Path:     %s\n", abs)
	fmt.Printf("Watched:  %s\n", watched)
	fmt.Printf("Tasks:    %d\n", len(todos))

	if len(priorityCounts) > 0 {
		// Print priority breakdown sorted
		var priorities []int
		for p := range priorityCounts {
			priorities = append(priorities, p)
		}
		sort.Ints(priorities)
		var parts []string
		for _, p := range priorities {
			parts = append(parts, fmt.Sprintf("p%d=%d", p, priorityCounts[p]))
		}
		fmt.Printf("          %s\n", strings.Join(parts, ", "))
	}

	// Show running tasks
	if daemon.IsDaemonRunning() {
		runningTasks, err := daemon.SendPsRequest()
		if err == nil {
			var projectTasks []daemon.TaskInfo
			for _, t := range runningTasks {
				if t.Project == abs {
					projectTasks = append(projectTasks, t)
				}
			}
			if len(projectTasks) > 0 {
				fmt.Printf("\nRunning:\n")
				for _, t := range projectTasks {
					// Strip project path prefix from task name for cleaner display
					name := t.Name
					if strings.HasPrefix(name, abs+"/") {
						name = strings.TrimPrefix(name, abs+"/")
					}
					fmt.Printf("  %-30s  PID %-8d  %s\n", name, t.PID, t.Elapsed)
				}
			}
		}
	}
}
