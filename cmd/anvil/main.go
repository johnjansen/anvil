package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/tools"

	"gopkg.in/yaml.v3"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// If not set (e.g. go run), init() falls back to the module version from debug.BuildInfo.
var version = "dev"

func init() {
	if version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		fmt.Fprintln(os.Stderr, "'anvil serve' has been renamed. Did you mean 'anvil watch'?")
		serveCmd()
	case "watch":
		watchCmd2(os.Args[2:])
	case "unwatch":
		fmt.Fprintln(os.Stderr, "'anvil unwatch' has been removed. Did you mean 'anvil project rm [path]'?")
		os.Exit(1)
	case "status":
		statusCmd()
	case "ps":
		fmt.Fprintln(os.Stderr, "'anvil ps' has been removed. Did you mean 'anvil task ls'?")
		os.Exit(1)
	case "init":
		initCmd(os.Args[2:])
	case "add":
		addCmd(os.Args[2:])
	case "list":
		fmt.Fprintln(os.Stderr, "'anvil list' has been removed. Did you mean 'anvil task ls'?")
		os.Exit(1)
	case "get":
		fmt.Fprintln(os.Stderr, "'anvil get' has been removed. Did you mean 'anvil task get <name>'?")
		os.Exit(1)
	case "delete":
		fmt.Fprintln(os.Stderr, "'anvil delete' has been removed. Did you mean 'anvil task rm <name>'?")
		os.Exit(1)
	case "log":
		fmt.Fprintln(os.Stderr, "'anvil log' has been removed. Did you mean 'anvil task log <name>'?")
		os.Exit(1)
	case "logs":
		logsCmd(os.Args[2:])
	case "stop-on-idle":
		stopOnIdleCmd()
	case "task":
		taskCmd(os.Args[2:])
	case "project":
		projectCmd(os.Args[2:])
	case "update":
		updateCmd(os.Args[2:])
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
  init [path]              Initialize a project and register it for watching
  watch [-d|--daemonize]   Start the daemon (once per machine)
  watch --stop             Stop the background daemon
  add [options] <task>     Add a task to the current project
  logs [<name>]            Raw worker output (all tasks if no name given)
  status                   Show watched projects
  stop-on-idle             Drain running tasks then exit the daemon
  task <subcommand>        Task management commands
  project <subcommand>     Project management commands
  update [--check]         Update anvil to the latest release
  version                  Show version

Add options:
  -p, --priority int          Task priority 0-9 (default 1)
  -s, --schedule string        Cron schedule (default "* * * * *"), "" for one-shot
      --pre-check string       Shell command to skip task if non-zero exit
      --allowed-tools string  Comma-separated tool allowlist (e.g. "Bash,Read")
      --max-concurrent int    Max parallel instances (default 1)
      --skip-permissions       Bypass all tool permission prompts

Task subcommands:
  create [options] <task>   Create a new task
  ls [-a|--all]             List tasks (--all for all watched projects)
  get <name>                Show task details including run status
  log [-f] <name>           Show execution log (-f to follow)
  rm <name>                 Remove a task (kills if running)
  run <name>                Trigger immediate execution (bypass cron)
  kill <name>               Kill a running task
  stop-on-idle <name>       Finish current run then stop rescheduling task
  unlock <name>             Remove stale lock file to allow retry

Project subcommands:
  create [path]            Initialize and watch a project in one step
  ls [-a|--all]            List watched projects
  get [path]               Show project details and running tasks
  rm [path] [--clean]      Unwatch a project (--clean removes .anvil/ too)

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

	created, err := config.EnsureConfig()
	if err != nil {
		log.Fatalf("failed to create ~/.anvil/config.yaml: %v", err)
	}
	if created {
		fmt.Printf("created %s\n", config.Path())
	}

	fmt.Printf("initialized %s\n", abs)

	// Register project for watching (fold in old 'anvil watch' behavior)
	registerProject(abs)
}

// registerProject registers a project directory with the daemon watcher.
// Extracted so both initCmd and watchCmd (legacy) can share this logic.
func registerProject(abs string) {
	if err := config.EnsureDir(); err != nil {
		log.Fatalf("failed to create ~/.anvil: %v", err)
	}

	hash := projectHash(abs)
	watchDir := filepath.Join(config.WatchedDir(), hash)

	if entries, err := os.ReadDir(watchDir); err == nil && len(entries) > 0 {
		fmt.Printf("already watching %s\n", abs)
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

	fmt.Printf("watching %s\n", abs)
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

	select {
	case sig := <-sigCh:
		log.Printf("received %v, shutting down", sig)
		d.Stop()
	case <-d.Done():
		// daemon stopped itself (e.g. stop-on-idle drain completed)
	}
}

// watchCmd2 handles "anvil watch" with optional --daemonize/-d, --stop, and --child flags.
func watchCmd2(args []string) {
	daemonize := false
	stop := false
	child := false
	for _, arg := range args {
		switch arg {
		case "--daemonize", "-d":
			daemonize = true
		case "--stop":
			stop = true
		case "--child":
			child = true
		}
	}

	if stop {
		stopDaemon()
		return
	}

	if child {
		runDaemonChild()
		return
	}

	if daemonize {
		daemonizeProcess()
		return
	}

	// Default: run in foreground (existing behavior)
	serveCmd()
}

// readDaemonPID reads the PID from the daemon PID file.
// Returns 0 if the file doesn't exist or the process is not alive.
func readDaemonPID() int {
	data, err := os.ReadFile(config.PidFile())
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	// Check if process is alive
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// Process not alive — clean up stale PID file
		os.Remove(config.PidFile())
		return 0
	}
	return pid
}

// daemonizeProcess re-execs the current binary with --child in a detached session.
func daemonizeProcess() {
	if err := config.EnsureDir(); err != nil {
		log.Fatalf("failed to create ~/.anvil: %v", err)
	}

	if pid := readDaemonPID(); pid != 0 {
		fmt.Fprintf(os.Stderr, "daemon already running (PID %d)\n", pid)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to find executable path: %v", err)
	}

	logFile, err := os.OpenFile(config.DaemonLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("failed to open daemon log: %v", err)
	}

	cmd := exec.Command(exe, "watch", "--child")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		log.Fatalf("failed to start daemon: %v", err)
	}

	logFile.Close()
	fmt.Fprintf(os.Stderr, "daemon started (PID %d)\n", cmd.Process.Pid)
}

// runDaemonChild is the internal entry point for the detached child process.
func runDaemonChild() {
	if err := config.EnsureDir(); err != nil {
		log.Fatalf("failed to create ~/.anvil: %v", err)
	}

	pid := os.Getpid()
	if err := os.WriteFile(config.PidFile(), []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Fatalf("failed to write PID file: %v", err)
	}
	defer os.Remove(config.PidFile())

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	d := daemon.New(cfg)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go d.Run()

	select {
	case sig := <-sigCh:
		log.Printf("received %v, shutting down", sig)
		d.Stop()
	case <-d.Done():
		d.Stop()
	}
}

// stopDaemon sends SIGTERM to the running daemon and waits for it to exit.
func stopDaemon() {
	pid := readDaemonPID()
	if pid == 0 {
		fmt.Fprintln(os.Stderr, "no daemon running")
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no daemon running")
		return
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop daemon: %v\n", err)
		return
	}

	// Wait up to 5 seconds for the PID file to disappear
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(config.PidFile()); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "daemon stopped (PID %d)\n", pid)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "daemon did not stop in time (PID %d)\n", pid)
}

// watchCmd is the legacy "register a project" command, now superseded by
// 'anvil init' which combines init + register. Kept for backward compatibility.
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

	registerProject(abs)
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

	// Show daemon drain state if running
	if daemon.IsDaemonRunning() {
		if status, err := daemon.SendStatusRequest(); err == nil && status.Draining {
			fmt.Println("daemon: draining (stop-on-idle active)")
		}
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

	// Show drain status header if applicable
	if status, err := daemon.SendStatusRequest(); err == nil && status.Draining {
		fmt.Println("(draining — no new tasks will be dispatched)")
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
	fmt.Printf("%-30s %-20s %-10s %-10s %-30s %s\n", "PROJECT", "TASK", "PID", "ELAPSED", "STATUS", "STARTED")
	fmt.Printf("%s\n", strings.Repeat("-", 120))

	// Print each task
	for _, t := range tasks {
		status := ""
		if t.Status != "" {
			status = t.Status
		}
		fmt.Printf("%-30s %-20s %-10d %-10s %-30s %s\n",
			truncate(t.Project, 30),
			truncate(t.Name, 20),
			t.PID,
			t.Elapsed,
			truncate(status, 30),
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
	taskCreateCmd(args)
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
	follow := false
	var rest []string
	for _, a := range args {
		switch a {
		case "-f", "--follow":
			follow = true
		default:
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil log [-f] <name|uuid>\n")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Try as todo name first, fall back to treating arg as a UUID directly
	id := rest[0]
	var todoName string
	proj, err := project.Load(abs)
	if err == nil {
		todos, err := proj.LoadTodos()
		if err == nil {
			if todo := findTodo(todos, rest[0]); todo != nil {
				id = todo.ID
				todoName = todo.Name
			}
		}
	}

	if id == "" {
		fmt.Fprintf(os.Stderr, "todo has no session ID\n")
		os.Exit(1)
	}

	// Resolve the session ID from the latest run record (not the task ID)
	sessionID, err := project.LatestSessionID(abs, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no session log found (looked at %s): %v\n", project.SessionPath(abs, id), err)
		os.Exit(1)
	}

	sessionPath := project.SessionPathBySessionID(abs, sessionID)

	if follow {
		followLog(sessionPath, abs, todoName)
		return
	}

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
		taskLsCmd(args[1:])
	case "get":
		taskGetCmd(args[1:])
	case "log":
		taskLogCmd(args[1:])
	case "rm":
		taskRmCmd(args[1:])
	case "run":
		taskRunCmd(args[1:])
	case "kill":
		taskKillCmd(args[1:])
	case "history":
		taskHistoryCmd(args[1:])
	case "stop-on-idle":
		taskStopOnIdleCmd(args[1:])
	case "unlock":
		taskUnlockCmd(args[1:])
	case "edit":
		taskEditCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown task command: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}
}

func taskCreateCmd(args []string) {
	priority := 1
	schedule := ""
	preCheck := ""
	allowedTools := ""
	maxConcurrent := 1
	skipPermissions := false

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
		case "--pre-check":
			if i+1 >= len(args) {
				log.Fatal("missing value for --pre-check")
			}
			i++
			preCheck = args[i]
		case "--allowed-tools":
			if i+1 >= len(args) {
				log.Fatal("missing value for --allowed-tools")
			}
			i++
			allowedTools = args[i]
		case "--max-concurrent":
			if i+1 >= len(args) {
				log.Fatal("missing value for --max-concurrent")
			}
			i++
			n := 0
			for _, c := range args[i] {
				if c < '0' || c > '9' {
					log.Fatalf("invalid max-concurrent: %s (must be a number)", args[i])
				}
				n = n*10 + int(c-'0')
			}
			maxConcurrent = n
		case "--skip-permissions":
			skipPermissions = true
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		log.Fatal("usage: anvil task create [-p priority] [-s schedule] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] <task text>")
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

	relPath, err := proj.AddTodo(priority, schedule, taskText, preCheck, allowedTools, maxConcurrent, skipPermissions)
	if err != nil {
		log.Fatalf("failed to add todo: %v", err)
	}

	fmt.Printf("added %s\n", relPath)

	// Show next run time for scheduled tasks.
	if schedule != "" {
		if p, err := cron.Parse(schedule); err == nil {
			if next, err := p.Next(time.Now()); err == nil {
				until := time.Until(next).Round(time.Minute)
				fmt.Fprintf(os.Stderr, "Next run: %s (%s from now)\n", next.Format("Mon 15:04"), until)
			}
		}
	}
}

func taskLsCmd(args []string) {
	allProjects := false
	for _, a := range args {
		if a == "--all" || a == "-a" {
			allProjects = true
		}
	}

	// Gather running tasks once
	var runningTasks []daemon.TaskInfo
	if daemon.IsDaemonRunning() {
		runningTasks, _ = daemon.SendPsRequest()
	}
	runningByID := make(map[string]daemon.TaskInfo)
	for _, t := range runningTasks {
		runningByID[fmt.Sprintf("%s/%s", t.Project, t.Name)] = t
	}

	type projectTodos struct {
		path  string
		todos []project.Todo
	}

	var projects []projectTodos

	if allProjects {
		watched, err := loadAllWatched()
		if err != nil {
			log.Fatalf("failed to read watched: %v", err)
		}
		for _, w := range watched {
			proj, err := project.Load(w.Path)
			if err != nil {
				continue
			}
			todos, _ := proj.LoadTodos()
			projects = append(projects, projectTodos{path: w.Path, todos: todos})
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
		todos, err := proj.LoadTodos()
		if err != nil {
			log.Fatalf("failed to load todos: %v", err)
		}
		projects = append(projects, projectTodos{path: abs, todos: todos})
	}

	total := 0
	for _, p := range projects {
		total += len(p.todos)
	}
	if total == 0 {
		fmt.Println("no tasks")
		return
	}

	for _, p := range projects {
		if allProjects && len(p.todos) > 0 {
			fmt.Printf("%s\n", p.path)
		}
		for _, t := range p.todos {
			taskKey := fmt.Sprintf("%s/%s", p.path, t.Name)
			status := "idle"
			if t.IsLocked {
				status = "locked"
			} else if rt, ok := runningByID[taskKey]; ok {
				if rt.Status != "" {
					status = rt.Status
				} else {
					status = "running"
				}
			}
			preview := strings.TrimSpace(t.Content)
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			fmt.Printf("p%d  %-14s  %-10s  %-35s  %s\n", t.Priority, t.Schedule, status, t.Name, preview)
		}
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
	follow := false
	var rest []string
	for _, a := range args {
		switch a {
		case "-f", "--follow":
			follow = true
		default:
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task log [-f] <name|uuid>\n")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	id := rest[0]
	var todoName string
	proj, err := project.Load(abs)
	if err == nil {
		todos, err := proj.LoadTodos()
		if err == nil {
			if todo := findTodo(todos, rest[0]); todo != nil {
				id = todo.ID
				todoName = todo.Name
			}
		}
	}

	// Try to resolve session ID: first from run records, then from the running daemon.
	var sessionID string

	if id != "" {
		sessionID, _ = project.LatestSessionID(abs, id)
	}

	// If we couldn't resolve a session ID from disk, check if the task is
	// currently running — the daemon tracks the session ID in memory.
	if sessionID == "" && todoName != "" && daemon.IsDaemonRunning() {
		if tasks, psErr := daemon.SendPsRequest(); psErr == nil {
			fullKey := fmt.Sprintf("%s/%s", abs, todoName)
			for _, t := range tasks {
				if t.Name == fullKey && t.SessionID != "" {
					sessionID = t.SessionID
					break
				}
			}
		}
	}

	if sessionID == "" {
		if id == "" {
			fmt.Fprintf(os.Stderr, "task has no ID in frontmatter and is not currently running\n")
		} else {
			fmt.Fprintf(os.Stderr, "no session log found for task\n")
		}
		os.Exit(1)
	}

	sessionPath := project.SessionPathBySessionID(abs, sessionID)

	if follow {
		followLog(sessionPath, abs, todoName)
		return
	}

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
		if err := daemon.SendKillRequest(todo.ID); err == nil {
			fmt.Printf("killed running task %s\n", todo.Name)
		}
	}

	if err := project.RemoveTodo(*todo); err != nil {
		log.Fatalf("failed to remove todo: %v", err)
	}

	fmt.Printf("removed p%d/%s\n", todo.Priority, todo.Name)
}

func taskRunCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task run <name>\n")
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
		fmt.Fprintln(os.Stderr, "daemon not running — start it with: anvil watch")
		os.Exit(1)
	}

	if err := daemon.SendRunRequest(abs, todo.ID, todo.Name); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("▶ Dispatched %s for immediate execution\n", todo.Name)
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

func taskHistoryCmd(args []string) {
	limit := 10
	showFailuresOnly := false
	jsonOutput := false
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-n", "--limit":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "usage: anvil task history <name> [-n limit] [--failures] [--json]\n")
				os.Exit(1)
			}
			if _, err := fmt.Sscanf(args[i+1], "%d", &limit); err != nil {
				fmt.Fprintf(os.Stderr, "invalid limit: %s\n", args[i+1])
				os.Exit(1)
			}
			i += 2
		case "-f", "--failures", "--show-failures-only":
			showFailuresOnly = true
			i++
		case "--json":
			jsonOutput = true
			i++
		default:
			break
		}
	}
	taskName := strings.Join(args[i:], " ")
	if taskName == "" {
		fmt.Fprintf(os.Stderr, "usage: anvil task history <name> [-n limit] [--failures] [--json]\n")
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
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	records, err := project.ReadAllRunRecords(abs, todo.ID)
	if err != nil {
		log.Fatalf("failed to read run history: %v", err)
	}

	if len(records) == 0 {
		fmt.Println("no run history found")
		return
	}

	// Filter failures if requested
	if showFailuresOnly {
		var filtered []project.RunRecord
		for _, rec := range records {
			if !rec.Success {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	if jsonOutput {
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Print header
	fmt.Printf("%-20s %10s %10s\n", "STARTED", "DURATION", "STATUS")
	for _, rec := range records {
		duration := ""
		if !rec.Finished.IsZero() {
			d := rec.Finished.Sub(rec.Started)
			if d < time.Minute {
				duration = fmt.Sprintf("%.0fs", d.Seconds())
			} else {
				duration = fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*float64(d.Minutes()))
			}
		}

		status := "ok"
		if !rec.Success {
			status = "failed"
			if rec.Error != "" {
				// Truncate error for display
				errMsg := rec.Error
				if len(errMsg) > 20 {
					errMsg = errMsg[:20] + "..."
				}
				status = errMsg
			}
		}

		fmt.Printf("%-20s %10s %10s\n", rec.Started.Format("2006-01-02 15:04"), duration, status)
	}
}

func taskUnlockCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task unlock <name>\n")
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

	// Remove the lock file if it exists
	if err := project.RemoveLock(*todo); err != nil {
		log.Fatalf("failed to remove lock: %v", err)
	}

	fmt.Printf("unlocked: %s\n", todo.Name)
}

func taskEditCmd(args []string) {
	// Parse flags
	var newSchedule *string
	var newPriority *int

	i := 0
	for i < len(args) {
		switch args[i] {
		case "-s", "--schedule":
			if i+1 >= len(args) {
				log.Fatal("missing value for -s/--schedule")
			}
			i++
			newSchedule = &args[i]
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
			newPriority = &n
		default:
			break
		}
		i++
	}

	// Remaining arg should be task name
	nameArgs := args
	if newSchedule != nil || newPriority != nil {
		// Strip the flags we consumed
		for len(nameArgs) > 0 && (nameArgs[0] == "-s" || nameArgs[0] == "--schedule" || nameArgs[0] == "-p" || nameArgs[0] == "--priority") {
			if len(nameArgs) >= 2 {
				nameArgs = nameArgs[2:]
			} else {
				break
			}
		}
	}

	if len(nameArgs) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task edit <name> [-s schedule] [-p priority]\n")
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

	todo := findTodo(todos, nameArgs[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", nameArgs[0])
		os.Exit(1)
	}

	// If targeted flags provided, apply them without opening editor
	if newSchedule != nil || newPriority != nil {
		// Validate schedule if provided
		if newSchedule != nil && *newSchedule != "" && *newSchedule != "persistent" {
			if _, err := cron.Parse(*newSchedule); err != nil {
				log.Fatalf("invalid schedule %q: %v", *newSchedule, err)
			}
		}

		// Determine new priority (default to current)
		priority := todo.Priority
		if newPriority != nil {
			priority = *newPriority
		}

		// Read current file content
		raw, err := os.ReadFile(todo.Path)
		if err != nil {
			log.Fatalf("failed to read task file: %v", err)
		}

		// Parse and update front-matter
		contentStr := string(raw)
		body := contentStr

		if strings.HasPrefix(contentStr, "---\n") {
			parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
			if len(parts) == 2 {
				fm := parts[0]
				body = parts[1]

				var fmData struct {
					Schedule        string   `yaml:"schedule"`
					ID              string   `yaml:"id"`
					Resume          *bool    `yaml:"resume"`
					MaxConcurrent   int      `yaml:"max_concurrent"`
					SkipPermissions bool     `yaml:"skip_permissions"`
					AllowedTools    []string `yaml:"allowed_tools"`
					PreCheck        string   `yaml:"pre_check"`
					OnSuccess       string   `yaml:"on_success"`
					OnFailure       string   `yaml:"on_failure"`
				}
				if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
					// Update schedule if provided
					if newSchedule != nil {
						fmData.Schedule = *newSchedule
					}

					// Marshal back
					fmBytes, err := yaml.Marshal(fmData)
					if err != nil {
						log.Fatalf("failed to marshal front-matter: %v", err)
					}

					// Build new file content
					var sb strings.Builder
					sb.WriteString("---\n")
					sb.WriteString(string(fmBytes))
					sb.WriteString("---\n")
					sb.WriteString(body)

					// If priority changed, move to new directory
					if priority != todo.Priority {
						newDir := filepath.Join(abs, ".anvil", "todos", fmt.Sprintf("p%d", priority))
						if err := os.MkdirAll(newDir, 0755); err != nil {
							log.Fatalf("failed to create priority directory: %v", err)
						}
						newPath := filepath.Join(newDir, filepath.Base(todo.Path))
						// Write updated content to new location before moving
						if err := os.WriteFile(newPath, []byte(sb.String()), 0644); err != nil {
							log.Fatalf("failed to write task file: %v", err)
						}
						// Remove old file
						if err := os.Remove(todo.Path); err != nil {
							log.Fatalf("failed to remove old task file: %v", err)
						}
						fmt.Printf("updated priority: p%d -> p%d\n", todo.Priority, priority)
					} else {
						// Write back in place
						if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
							log.Fatalf("failed to write task file: %v", err)
						}
					}

					if newSchedule != nil {
						fmt.Printf("updated schedule: %s\n", *newSchedule)
					}
					return
				}
			}
		}
		log.Fatal("failed to parse task front-matter")
	}

	// No flags: open in editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, todo.Path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("editor exited with error: %v", err)
	}

	// Validate the edited file's schedule if it has one
	raw, err := os.ReadFile(todo.Path)
	if err == nil {
		contentStr := string(raw)
		if strings.HasPrefix(contentStr, "---\n") {
			parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
			if len(parts) == 2 {
				var fmData struct {
					Schedule string `yaml:"schedule"`
				}
				if yaml.Unmarshal([]byte(parts[0]), &fmData) == nil && fmData.Schedule != "" && fmData.Schedule != "persistent" {
					if _, err := cron.Parse(fmData.Schedule); err != nil {
						log.Fatalf("invalid schedule %q after edit: %v", fmData.Schedule, err)
					}
				}
			}
		}
	}

	fmt.Printf("edited: %s\n", todo.Name)
}

func stopOnIdleCmd() {
	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	if err := daemon.SendDrainRequest(); err != nil {
		fmt.Printf("failed to set stop-on-idle: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("stop-on-idle: daemon will finish running tasks then exit")
}

func taskStopOnIdleCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task stop-on-idle <name>\n")
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

	if err := daemon.SendDrainTaskRequest(todo.ID); err != nil {
		fmt.Printf("failed to set stop-on-idle for task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("stop-on-idle: task %s will not be rescheduled after its current run\n", todo.Name)
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

	// Initialize project .anvil/ structure
	if err := project.Init(abs, tools.FS); err != nil {
		log.Fatalf("failed to init project: %v", err)
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

// followLog tails a session log file, printing new lines as they are appended.
// Exits on Ctrl+C, or when the task is no longer running and the file stops growing.
func followLog(sessionPath string, projectPath string, taskName string) {
	// Wait for the file to exist (task may not have started yet)
	for {
		if _, err := os.Stat(sessionPath); err == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "waiting for log file...\n")
		time.Sleep(500 * time.Millisecond)
	}

	f, err := os.Open(sessionPath)
	if err != nil {
		log.Fatalf("failed to open session log: %v", err)
	}
	defer f.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	buf := make([]byte, 4096)
	stableCount := 0

	for {
		select {
		case <-sigCh:
			return
		default:
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			stableCount = 0
			continue
		}
		if readErr != nil && readErr != io.EOF {
			return
		}

		// No new data — check periodically if task has finished
		stableCount++
		if stableCount%10 == 0 {
			taskRunning := false
			if daemon.IsDaemonRunning() {
				tasks, psErr := daemon.SendPsRequest()
				if psErr == nil {
					fullKey := fmt.Sprintf("%s/%s", projectPath, taskName)
					for _, t := range tasks {
						if t.Name == fullKey {
							taskRunning = true
							break
						}
					}
				}
			}
			if !taskRunning && taskName != "" {
				fmt.Println("[task completed]")
				return
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// rawLogPath returns the path to the raw stdout/stderr log file for a completed run.
// The daemon writes these files to <project>/.anvil/logs/<taskID>/<runID>.log
func rawLogPath(projectPath, taskID, runID string) string {
	return filepath.Join(projectPath, ".anvil", "logs", taskID, runID+".log")
}

func logsCmd(args []string) {
	if !daemon.IsDaemonRunning() {
		fmt.Fprintln(os.Stderr, "daemon not running")
		os.Exit(1)
	}

	if len(args) == 0 {
		logsMultiplex()
		return
	}

	// Single task mode: anvil logs <name>
	name := args[0]
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Resolve todo name -> task ID and full daemon key
	var taskID string
	var todoName string
	proj, err := project.Load(abs)
	if err == nil {
		todos, err := proj.LoadTodos()
		if err == nil {
			if todo := findTodo(todos, name); todo != nil {
				taskID = todo.ID
				todoName = todo.Name
			}
		}
	}

	// Check if the task is currently running
	tasks, err := daemon.SendPsRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get running tasks: %v\n", err)
		os.Exit(1)
	}

	fullKey := fmt.Sprintf("%s/%s", abs, todoName)
	for _, t := range tasks {
		if t.Name == fullKey && t.LogPath != "" {
			// Task is running — follow the live raw log
			followLog(t.LogPath, abs, todoName)
			return
		}
	}

	// Task is not running — print the most recent completed raw log
	if taskID == "" {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", name)
		os.Exit(1)
	}

	rec, err := project.ReadCurrentRunRecord(abs, taskID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no run record found for task %s\n", name)
		os.Exit(1)
	}

	logPath := rawLogPath(abs, rec.TaskID, rec.RunID)
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no raw log found for task %s (looked at %s)\n", name, logPath)
			os.Exit(1)
		}
		log.Fatalf("failed to read raw log: %v", err)
	}
	fmt.Print(string(data))
}

// logsMultiplex follows raw output from all currently running tasks, prefixing
// each line with the task name. Exits when all followed tasks complete or on SIGINT.
func logsMultiplex() {
	tasks, err := daemon.SendPsRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get running tasks: %v\n", err)
		os.Exit(1)
	}

	// Collect tasks that have a raw log path
	type taskState struct {
		name    string
		logPath string
		file    *os.File
		offset  int64
		buf     []byte // partial line buffer
	}

	var states []*taskState
	for _, t := range tasks {
		if t.LogPath == "" {
			continue
		}
		f, err := os.Open(t.LogPath)
		if err != nil {
			continue
		}
		// Display name: strip project path prefix for readability
		displayName := t.Name
		if idx := strings.LastIndex(displayName, "/"); idx >= 0 {
			displayName = displayName[idx+1:]
		}
		// Strip .md suffix if present
		displayName = strings.TrimSuffix(displayName, ".md")
		states = append(states, &taskState{
			name:    displayName,
			logPath: t.LogPath,
			file:    f,
		})
	}

	if len(states) == 0 {
		fmt.Println("no tasks running")
		return
	}
	defer func() {
		for _, s := range states {
			s.file.Close()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// printLines flushes complete lines from a task's buffer to stdout with prefix.
	printLines := func(s *taskState, flushAll bool) {
		for {
			idx := -1
			for i, b := range s.buf {
				if b == '\n' {
					idx = i
					break
				}
			}
			if idx < 0 {
				if flushAll && len(s.buf) > 0 {
					fmt.Printf("%s: %s\n", s.name, string(s.buf))
					s.buf = s.buf[:0]
				}
				break
			}
			fmt.Printf("%s: %s\n", s.name, string(s.buf[:idx]))
			s.buf = s.buf[idx+1:]
		}
	}

	buf := make([]byte, 4096)
	psCheckTick := 0

	for {
		select {
		case <-sigCh:
			// Flush remaining partial lines before exit
			for _, s := range states {
				printLines(s, true)
			}
			return
		default:
		}

		// Read new bytes from each active task's log file
		for _, s := range states {
			if s.file == nil {
				continue
			}
			n, readErr := s.file.Read(buf)
			if n > 0 {
				s.buf = append(s.buf, buf[:n]...)
				printLines(s, false)
			}
			if readErr != nil && readErr != io.EOF {
				printLines(s, true)
				s.file.Close()
				s.file = nil
			}
		}

		// Periodically re-check /ps to remove finished tasks (~every 2s = 8 * 250ms)
		psCheckTick++
		if psCheckTick >= 8 {
			psCheckTick = 0
			running, psErr := daemon.SendPsRequest()
			if psErr == nil {
				runningPaths := make(map[string]bool)
				for _, t := range running {
					runningPaths[t.LogPath] = true
				}
				for _, s := range states {
					if s.file != nil && !runningPaths[s.logPath] {
						// Task finished — drain remaining bytes then close
						for {
							n, err := s.file.Read(buf)
							if n > 0 {
								s.buf = append(s.buf, buf[:n]...)
							}
							if err != nil {
								break
							}
						}
						printLines(s, true)
						s.file.Close()
						s.file = nil
					}
				}
			}
		}

		// Check if all tasks are done
		allDone := true
		for _, s := range states {
			if s.file != nil {
				allDone = false
				break
			}
		}
		if allDone {
			fmt.Println("all tasks completed")
			return
		}

		time.Sleep(250 * time.Millisecond)
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

func updateCmd(args []string) {
	checkOnly := false
	for _, a := range args {
		if a == "--check" {
			checkOnly = true
		}
	}

	// Fetch latest release from GitHub
	resp, err := http.Get("https://api.github.com/repos/johnjansen/anvil/releases/latest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "update check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "update check failed: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse release info: %v\n", err)
		os.Exit(1)
	}

	latest := release.TagName
	if latest == "" {
		fmt.Fprintf(os.Stderr, "no releases found\n")
		os.Exit(1)
	}

	// Compare versions (strip leading 'v' for comparison)
	normalizedCurrent := strings.TrimPrefix(version, "v")
	normalizedLatest := strings.TrimPrefix(latest, "v")

	if normalizedCurrent == normalizedLatest {
		fmt.Printf("already up to date (%s)\n", version)
		return
	}

	if checkOnly {
		fmt.Printf("update available: %s → %s\n", version, latest)
		return
	}

	fmt.Printf("updating: %s → %s\n", version, latest)

	// Build download URL matching install.sh conventions
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	binaryURL := fmt.Sprintf("https://github.com/johnjansen/anvil/releases/download/%s/anvil-%s-%s", latest, goos, goarch)

	// Find the path to the running binary
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to determine executable path: %v\n", err)
		os.Exit(1)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve executable path: %v\n", err)
		os.Exit(1)
	}

	// Download new binary to a temp file in the same directory for atomic rename
	fmt.Printf("downloading %s...\n", binaryURL)
	dlResp, err := http.Get(binaryURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
		os.Exit(1)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "download failed: HTTP %d from %s\n", dlResp.StatusCode, binaryURL)
		os.Exit(1)
	}

	execDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(execDir, ".anvil-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp file (permission denied?): %v\n", err)
		fmt.Fprintf(os.Stderr, "try: sudo anvil update\n")
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "failed to set permissions: %v\n", err)
		os.Exit(1)
	}

	// Atomically replace the binary
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "failed to replace binary: %v\n", err)
		fmt.Fprintf(os.Stderr, "try: sudo anvil update\n")
		os.Exit(1)
	}

	fmt.Printf("updated to %s\n", latest)
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
