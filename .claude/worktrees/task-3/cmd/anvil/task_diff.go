package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

func taskDiffCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil task diff <name> [<v1> [v2]]\n")
		fmt.Fprintf(os.Stderr, "       anvil task diff <name> [--run1 ID] [--run2 ID] [--context N] [--ignore-whitespace] [--json]\n")
		os.Exit(1)
	}

	// Detect output-diff mode: if only 1 positional arg (task name) or any --run1/--run2 flags present
	hasOutputDiffFlags := false
	positionalCount := 0
	for _, a := range args {
		if a == "--run1" || a == "--run2" || a == "--context" || a == "--ignore-whitespace" || a == "--json" {
			hasOutputDiffFlags = true
		}
		if !strings.HasPrefix(a, "--") {
			positionalCount++
		}
	}
	if positionalCount < 2 || hasOutputDiffFlags {
		taskOutputDiffCmd(args)
		return
	}

	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: anvil task diff <name> <v1> [v2]\n")
		os.Exit(1)
	}

	taskName := args[0]
	v1Str := args[1]
	v2Str := ""
	if len(args) >= 3 {
		v2Str = args[2]
	}

	// Parse version numbers (accept "v1" or "1")
	v1Str = strings.TrimPrefix(v1Str, "v")
	v1Num := 0
	if _, err := fmt.Sscanf(v1Str, "%d", &v1Num); err != nil {
		fmt.Fprintf(os.Stderr, "invalid version: %s\n", args[1])
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "error: task not found: %s\n", taskName)
		os.Exit(1)
	}

	taskNameClean := strings.TrimSuffix(todo.Name, ".md")

	ver1, err := project.ReadVersion(abs, taskNameClean, v1Num)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: version not found: v%d\n", v1Num)
		os.Exit(1)
	}

	var oldLabel, newLabel, oldContent, newContent string

	if v2Str != "" {
		v2Str = strings.TrimPrefix(v2Str, "v")
		v2Num := 0
		if _, err := fmt.Sscanf(v2Str, "%d", &v2Num); err != nil {
			fmt.Fprintf(os.Stderr, "invalid version: %s\n", args[2])
			os.Exit(1)
		}
		ver2, err := project.ReadVersion(abs, taskNameClean, v2Num)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: version not found: v%d\n", v2Num)
			os.Exit(1)
		}
		oldLabel = fmt.Sprintf("v%d  %s", ver1.VersionNumber, ver1.Timestamp.Format("2006-01-02 15:04:05"))
		newLabel = fmt.Sprintf("v%d  %s", ver2.VersionNumber, ver2.Timestamp.Format("2006-01-02 15:04:05"))
		oldContent = ver1.Content
		newContent = ver2.Content
	} else {
		// Diff against current file
		currentContent, err := os.ReadFile(todo.Path)
		if err != nil {
			log.Fatalf("failed to read current task file: %v", err)
		}
		oldLabel = fmt.Sprintf("v%d  %s", ver1.VersionNumber, ver1.Timestamp.Format("2006-01-02 15:04:05"))
		newLabel = "current"
		oldContent = ver1.Content
		newContent = string(currentContent)
	}

	diff := project.UnifiedDiff(oldLabel, newLabel, oldContent, newContent)
	if diff == "" {
		fmt.Println("no differences")
		return
	}
	fmt.Print(diff)
}

func taskRestoreCmd(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: anvil task restore <name> <version>\n")
		os.Exit(1)
	}

	taskName := args[0]
	vStr := strings.TrimPrefix(args[1], "v")
	vNum := 0
	if _, err := fmt.Sscanf(vStr, "%d", &vNum); err != nil {
		fmt.Fprintf(os.Stderr, "invalid version: %s\n", args[1])
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "error: task not found: %s\n", taskName)
		os.Exit(1)
	}

	taskNameClean := strings.TrimSuffix(todo.Name, ".md")

	ver, err := project.ReadVersion(abs, taskNameClean, vNum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: version not found: v%d\n", vNum)
		os.Exit(1)
	}

	// Read current content
	currentContent, err := os.ReadFile(todo.Path)
	if err != nil {
		log.Fatalf("failed to read current task file: %v", err)
	}

	if project.ComputeFileHash(string(currentContent)) == ver.ContentHash {
		fmt.Printf("no changes: current content matches v%d\n", vNum)
		return
	}

	// Check if already at this version
	versions, _ := project.ReadAllVersions(abs, taskNameClean)
	if len(versions) > 0 && versions[0].ContentHash == ver.ContentHash {
		fmt.Printf("task is already at v%d\n", vNum)
		return
	}

	// Write restored content to task file
	if err := os.WriteFile(todo.Path, []byte(ver.Content), 0644); err != nil {
		log.Fatalf("failed to write task file: %v", err)
	}

	// Create new version snapshot
	author := project.GetAuthor(abs)
	summary := fmt.Sprintf("restored from v%d", vNum)
	newVer, err := project.WriteTaskVersion(abs, taskNameClean, ver.Content, author, summary)
	if err != nil {
		log.Fatalf("failed to create version snapshot: %v", err)
	}

	fmt.Printf("restored %s to v%d (created v%d)\n", taskNameClean, vNum, newVer.VersionNumber)
}
