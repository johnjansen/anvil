package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/tools"

	"gopkg.in/yaml.v3"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd()
	case "watch":
		watchCmd(os.Args[2:])
	case "unwatch":
		unwatchCmd(os.Args[2:])
	case "status":
		statusCmd()
	case "ps":
		psCmd()
	case "init":
		initCmd(os.Args[2:])
	case "add":
		addCmd(os.Args[2:])
	case "list":
		listCmd()
	case "get":
		getCmd(os.Args[2:])
	case "delete":
		deleteCmd(os.Args[2:])
	case "log":
		logCmd(os.Args[2:])
	case "task":
		taskCmd(os.Args[2:])
	case "project":
		projectCmd(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("anvil %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `anvil - central task dispatcher for LLM projects

Usage:
  anvil <command> [options]

Commands:
  init [path]              Initialize a project (.anvil/ and .claude/skills/)
  serve                    Start the daemon (once per machine)
  watch [path]             Register a project directory
  unwatch [path]           Stop watching a project directory
  add [options] <task>     Add a todo task to the current project
  list                     List all todos in the current project
  get <name>               Show details of a todo by name
  delete <name>            Delete a todo by name
  log <name>               Show session log for a todo
  status                   Show watched projects
  ps                       Show running tasks
  task <subcommand>        Task management commands
  project <subcommand>     Project management commands
  version                  Show version

Add options:
  -p, --priority int    Task priority 0-9 (default 1)
  -s, --schedule string Cron schedule (default "* * * * *")

Task subcommands:
  create [options] <task>   Create a new task
  -p, --priority int        Task priority 0-9 (default 1)
  -s, --schedule string     Cron schedule (default "* * * * *"), "" for one-shot
  ls                        List tasks in current project
  get <name>                Show details of a task
  log <name>                Show execution log for a task
  rm <name>                 Remove a task (kills if running)
  kill <name>               Kill a running task

Project subcommands:
  create [path]            Initialize and watch a project in one step
  ls [-a|--all]            List watched projects (default: current directory)
  get [path]               Show project details (default: current directory)

Configuration:
  ~/.anvil/config.yaml   Daemon config
  <project>/.anvil/      Project config and todos
`)
}

func initCmd(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	if err := project.Init(abs, tools.FS); err != nil {
		log.Fatalf("failed to init project: %v", err)
	}

	fmt.Printf("initialized %s\n", abs)
}

func serveCmd() {
	if err := config.EnsureDir(); err != nil {
		log.Fatalf("failed to create ~/.anvil: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	d := daemon.New(cfg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go d.Run()

	sig := <-sigCh
	log.Printf("received %v, shutting down", sig)
	d.Stop()
}

func watchCmd(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Initialize project .anvil/ if it doesn't exist
	if _, err := os.Stat(filepath.Join(abs, ".anvil", "todos")); os.IsNotExist(err) {
		if err := project.Init(abs, tools.FS); err != nil {
			log.Fatalf("failed to init project: %v", err)
		}
		fmt.Printf("initialized %s/.anvil/\n", abs)
	}

	if err := config.EnsureDir(); err != nil {
		log.Fatalf("failed to create ~/.anvil: %v", err)
	}

	// Check if already watched
	hash := projectHash(abs)
	watchDir := filepath.Join(config.WatchedDir(), hash)

	if entries, err := os.ReadDir(watchDir); err == nil && len(entries) > 0 {
		fmt.Printf("already watching %s\n", abs)
		return
	}

	// Create watched/{hash}/timestamp.md
	if err := os.MkdirAll(watchDir, 0755); err != nil {
		log.Fatalf("failed to create watch dir: %v", err)
	}

	now := time.Now()
	filename := now.Format("2006-01-02T15-04-05") + ".md"

	frontmatter := watchFrontmatter{
		Path:      abs,
		WatchedAt: now,
	}
	data, err := yaml.Marshal(frontmatter)
	if err != nil {
		log.Fatalf("failed to marshal: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(data)
	sb.WriteString("---\n")

	if err := os.WriteFile(filepath.Join(watchDir, filename), []byte(sb.String()), 0644); err != nil {
		log.Fatalf("failed to write watch file: %v", err)
	}

	fmt.Printf("watching %s\n", abs)
}

func unwatchCmd(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
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
}

func statusCmd() {
	watched, err := loadAllWatched()
	if err != nil {
		log.Fatalf("failed to read watched: %v", err)
	}

	if len(watched) == 0 {
		fmt.Println("no watched projects")
		return
	}

	for _, w := range watched {
		proj, err := project.Load(w.Path)
		if err != nil {
			fmt.Printf("  %s  (error: %v)\n", w.Path, err)
			continue
		}
		todos, _ := proj.LoadTodos()
		fmt.Printf("  %s  todos=%d\n", w.Path, len(todos))
	}
}

func psCmd() {
	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	tasks, err := daemon.SendPsRequest()
	if err != nil {
		fmt.Printf("failed to get tasks: %v\n", err)
		return
	}

	if len(tasks) == 0 {
		fmt.Println("no running tasks")
		return
	}

	// Print table header
	fmt.Printf("%-30s %-20s %-10s %-10s %s\n", "PROJECT", "TASK", "PID", "ELAPSED", "STARTED")
	fmt.Printf("%s\n", strings.Repeat("-", 100))

	// Print each task
	for _, t := range tasks {
		fmt.Printf("%-30s %-20s %-10d %-10s %s\n",
			truncate(t.Project, 30),
			truncate(t.Name, 20),
			t.PID,
			t.Elapsed,
			t.Started)
	}
}

// truncate shortens a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func addCmd(args []string) {
	priority := 1
	schedule := "* * * * *"

	// Simple flag parsing — pull out -p/-s before treating the rest as task text
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p", "--priority":
			if i+1 >= len(args) {
				log.Fatal("missing value for -p/--priority")
			}
			i++
			n := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					log.Fatalf("invalid priority: %s (must be 0-9)", args[i])
				}
				n = n*10 + int(c-'0')
			}
			if n > 9 {
				log.Fatalf("priority must be 0-9, got %d", n)
			}
			priority = n
		case "-s", "--schedule":
			if i+1 >= len(args) {
				log.Fatal("missing value for -s/--schedule")
			}
			i++
			schedule = args[i]
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		log.Fatal("usage: anvil add [-p priority] [-s schedule] <task text>")
	}

	taskText := strings.Join(rest, " ")

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Ensure project is initialized
	if _, err := os.Stat(filepath.Join(abs, ".anvil", "todos")); os.IsNotExist(err) {
		if err := project.Init(abs, tools.FS); err != nil {
			log.Fatalf("failed to init project: %v", err)
		}
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	relPath, err := proj.AddTodo(priority, schedule, taskText)
	if err != nil {
		log.Fatalf("failed to add todo: %v", err)
	}

	fmt.Printf("added %s\n", relPath)
}

func listCmd() {
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

	if len(todos) == 0 {
		fmt.Println("no todos")
		return
	}

	for _, t := range todos {
		preview := strings.TrimSpace(t.Content)
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		fmt.Printf("p%d  %-14s  %-40s  %s\n", t.Priority, t.Schedule, t.Name, preview)
	}
}

func getCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil get <name>\n")
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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "todo not found: %s\n", args[0])
		os.Exit(1)
	}

	fmt.Printf("File:     p%d/%s\n", todo.Priority, todo.Name)
	fmt.Printf("ID:       %s\n", todo.ID)
	fmt.Printf("Schedule: %s\n", todo.Schedule)
	fmt.Printf("Priority: %d\n", todo.Priority)
	if todo.ID != "" {
		sessionPath := project.SessionPath(abs, todo.ID)
		if _, err := os.Stat(sessionPath); err == nil {
			fmt.Printf("Session:  %s\n", sessionPath)
		}
	}
	fmt.Printf("\n%s", todo.Content)
}

func deleteCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil delete <name>\n")
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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "todo not found: %s\n", args[0])
		os.Exit(1)
	}

	if err := project.RemoveTodo(*todo); err != nil {
		log.Fatalf("failed to delete todo: %v", err)
	}

	fmt.Printf("deleted p%d/%s\n", todo.Priority, todo.Name)
}

func logCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil log <name|uuid>\n")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Try as todo name first, fall back to treating arg as a UUID directly
	id := args[0]
	proj, err := project.Load(abs)
	if err == nil {
		todos, err := proj.LoadTodos()
		if err == nil {
			if todo := findTodo(todos, args[0]); todo != nil {
				id = todo.ID
			}
		}
	}

	if id == "" {
		fmt.Fprintf(os.Stderr, "todo has no session ID\n")
		os.Exit(1)
	}

	sessionPath := project.SessionPath(abs, id)
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no session log found (looked at %s)\n", sessionPath)
			os.Exit(1)
		}
		log.Fatalf("failed to read session log: %v", err)
	}

	fmt.Print(string(data))
}

func taskCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task <subcommand> [options]\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		taskCreateCmd(args[1:])
	case "ls":
		taskLsCmd()
	case "get":
		taskGetCmd(args[1:])
	case "log":
		taskLogCmd(args[1:])
	case "rm":
		taskRmCmd(args[1:])
	case "kill":
		taskKillCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown task command: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}
}

func taskCreateCmd(args []string) {
	priority := 1
	schedule := "* * * * *"

	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p", "--priority":
			if i+1 >= len(args) {
				log.Fatal("missing value for -p/--priority")
			}
			i++
			n := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					log.Fatalf("invalid priority: %s (must be 0-9)", args[i])
				}
				n = n*10 + int(c-'0')
			}
			if n > 9 {
				log.Fatalf("priority must be 0-9, got %d", n)
			}
			priority = n
		case "-s", "--schedule":
			if i+1 >= len(args) {
				log.Fatal("missing value for -s/--schedule")
			}
			i++
			schedule = args[i]
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		log.Fatal("usage: anvil task create [-p priority] [-s schedule] <task text>")
	}

	taskText := strings.Join(rest, " ")

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	if _, err := os.Stat(filepath.Join(abs, ".anvil", "todos")); os.IsNotExist(err) {
		if err := project.Init(abs, tools.FS); err != nil {
			log.Fatalf("failed to init project: %v", err)
		}
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	relPath, err := proj.AddTodo(priority, schedule, taskText)
	if err != nil {
		log.Fatalf("failed to add todo: %v", err)
	}

	fmt.Printf("added %s\n", relPath)
}

func taskLsCmd() {
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

	if len(todos) == 0 {
		fmt.Println("no tasks")
		return
	}

	for _, t := range todos {
		preview := strings.TrimSpace(t.Content)
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		fmt.Printf("p%d  %-14s  %-40s  %s\n", t.Priority, t.Schedule, t.Name, preview)
	}
}

func taskGetCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task get <name>\n")
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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", args[0])
		os.Exit(1)
	}

	// Check if task is running
	runStatus := "idle"
	if daemon.IsDaemonRunning() {
		runningTasks, err := daemon.SendPsRequest()
		if err == nil {
			taskName := fmt.Sprintf("%s/%s", abs, todo.Name)
			for _, t := range runningTasks {
				if t.Name == taskName {
					runStatus = fmt.Sprintf("running (PID %d, elapsed %s)", t.PID, t.Elapsed)
					break
				}
			}
		}
	}

	fmt.Printf("File:     %s\n", todo.Path)
	fmt.Printf("ID:       %s\n", todo.ID)
	fmt.Printf("Schedule: %s\n", todo.Schedule)
	fmt.Printf("Priority: %d\n", todo.Priority)
	if todo.ID != "" {
		sessionPath := project.SessionPath(abs, todo.ID)
		if _, err := os.Stat(sessionPath); err == nil {
			fmt.Printf("Session:  %s\n", sessionPath)
		}
	}
	fmt.Printf("Status:   %s\n", runStatus)
	fmt.Printf("\n%s", todo.Content)
}

func taskLogCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task log <name|uuid>\n")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	id := args[0]
	proj, err := project.Load(abs)
	if err == nil {
		todos, err := proj.LoadTodos()
		if err == nil {
			if todo := findTodo(todos, args[0]); todo != nil {
				id = todo.ID
			}
		}
	}

	if id == "" {
		fmt.Fprintf(os.Stderr, "task has no session ID\n")
		os.Exit(1)
	}

	sessionPath := project.SessionPath(abs, id)
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no session log found (looked at %s)\n", sessionPath)
			os.Exit(1)
		}
		log.Fatalf("failed to read session log: %v", err)
	}

	fmt.Print(string(data))
}

func taskRmCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task rm <name>\n")
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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", args[0])
		os.Exit(1)
	}

	// Kill if running
	if daemon.IsDaemonRunning() {
		if err := daemon.SendKillRequest(todo.ID); err != nil {
			log.Printf("warning: failed to kill task: %v", err)
		} else {
			log.Printf("killed running task")
		}
	}

	if err := project.RemoveTodo(*todo); err != nil {
		log.Fatalf("failed to remove todo: %v", err)
	}

	fmt.Printf("removed p%d/%s\n", todo.Priority, todo.Name)
}

func taskKillCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task kill <name>\n")
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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", args[0])
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	if err := daemon.SendKillRequest(todo.ID); err != nil {
		fmt.Printf("failed to kill task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("killed task: %s\n", args[0])
}

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

	// Initialize project .anvil/ structure
	if err := project.Init(abs, tools.FS); err != nil {
		log.Fatalf("failed to init project: %v", err)
	}

	// Register with daemon (watch)
	if err := config.EnsureDir(); err != nil {
		log.Fatalf("failed to create ~/.anvil: %v", err)
	}

	hash := projectHash(abs)
	watchDir := filepath.Join(config.WatchedDir(), hash)

	if entries, err := os.ReadDir(watchDir); err == nil && len(entries) > 0 {
		fmt.Printf("created %s (already watched)\n", abs)
		return
	}

	if err := os.MkdirAll(watchDir, 0755); err != nil {
		log.Fatalf("failed to create watch dir: %v", err)
	}

	now := time.Now()
	filename := now.Format("2006-01-02T15-04-05") + ".md"

	frontmatter := watchFrontmatter{
		Path:      abs,
		WatchedAt: now,
	}
	data, err := yaml.Marshal(frontmatter)
	if err != nil {
		log.Fatalf("failed to marshal: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(data)
	sb.WriteString("---\n")

	if err := os.WriteFile(filepath.Join(watchDir, filename), []byte(sb.String()), 0644); err != nil {
		log.Fatalf("failed to write watch file: %v", err)
	}

	fmt.Printf("created and watching %s\n", abs)
}

func projectLsCmd(args []string) {
	allProjects := false
	for _, a := range args {
		if a == "--all" || a == "-a" {
			allProjects = true
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
		fmt.Println("no watched projects")
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

// --- helpers ---

type watchFrontmatter struct {
	Path      string    `yaml:"path"`
	WatchedAt time.Time `yaml:"watched_at"`
}

func projectHash(absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	return fmt.Sprintf("%x", h[:4])
}

func loadAllWatched() ([]watchFrontmatter, error) {
	watchedDir := config.WatchedDir()
	dirs, err := os.ReadDir(watchedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []watchFrontmatter
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		dirPath := filepath.Join(watchedDir, d.Name())
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		// Sort entries, take the latest .md file
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() > entries[j].Name()
		})

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
			if err != nil {
				break
			}

			content := string(data)
			start := strings.Index(content, "---\n")
			if start == -1 {
				break
			}
			end := strings.Index(content[start+4:], "\n---")
			if end == -1 {
				break
			}

			var fm watchFrontmatter
			if err := yaml.Unmarshal([]byte(content[start+4:start+4+end]), &fm); err != nil {
				break
			}

			result = append(result, fm)
			break
		}
	}

	return result, nil
}
