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
	"github.com/johnjansen/anvil/internal/diagnostic"
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
		psCmd()
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
	case "daemon":
		daemonCmd(os.Args[2:])
	case "stop-on-idle":
		stopOnIdleCmd()
	case "task":
		taskCmd(os.Args[2:])
	case "project":
		projectCmd(os.Args[2:])
	case "usage":
		usageCmd(os.Args[2:])
	case "update":
		updateCmd(os.Args[2:])
	case "doctor":
		doctorCmd()
	case "reload":
		reloadCmd(os.Args[2:])
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
  ps                       Show running tasks
  status                   Show watched projects
  reload                   Reload daemon configuration (SIGHUP)
  usage [options]           Show LLM token usage and estimated costs
  stop-on-idle             Drain running tasks then exit the daemon
  task <subcommand>        Task management commands
  project <subcommand>     Project management commands
  daemon <subcommand>      Daemon management commands
  update [--check]         Update anvil to the latest release
  version                  Show version

Add options:
  -p, --priority int          Task priority 0-9 (default 1)
  -s, --schedule string        Cron schedule (e.g., "*/15 * * * *"), "" for one-shot
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
  queue                     Show daemon queue status and skip reasons

Project subcommands:
  create [path]            Initialize and watch a project in one step
  ls [-a|--all]            List watched projects
  get [path]               Show project details and running tasks
  rm [path] [--clean]      Unwatch a project (--clean removes .anvil/ too)

Daemon subcommands:
  log [-f] [-n lines]    View daemon log (-f to follow, -n for last N lines)

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

	// Warn if daemon is not running
	if !daemon.IsDaemonRunning() {
		fmt.Fprintf(os.Stderr, "⚠ Daemon is not running. Run 'anvil watch' to start executing tasks.\n")
	}
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

func doctorCmd() {
	results := diagnostic.All()

	if len(results) == 0 {
		fmt.Println("All checks passed!")
		return
	}

	fmt.Println("Anvil Diagnostics")
	fmt.Println(strings.Repeat("=", 60))

	var passes, warnings, failures int

	for _, r := range results {
		switch r.Status {
		case "pass":
			fmt.Printf("[PASS] %s\n", r.Name)
			passes++
		case "warn":
			fmt.Printf("[WARN] %s\n", r.Name)
			fmt.Printf("       %s\n", r.Message)
			if r.Fix != "" {
				fmt.Printf("       Fix: %s\n", r.Fix)
			}
			warnings++
		case "fail":
			fmt.Printf("[FAIL] %s\n", r.Name)
			fmt.Printf("       %s\n", r.Message)
			if r.Fix != "" {
				fmt.Printf("       Fix: %s\n", r.Fix)
			}
			failures++
		}
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Summary: %d pass, %d warn, %d fail\n", passes, warnings, failures)

	if failures > 0 {
		os.Exit(1)
	}
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

func reloadCmd(args []string) {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Println("Usage: anvil reload")
		fmt.Println("")
		fmt.Println("Reload the daemon configuration without restarting.")
		fmt.Println("")
		fmt.Println("Sends SIGHUP to the daemon to reload ~/.anvil/config.yaml.")
		fmt.Println("New tasks will use the updated config; running tasks are unaffected.")
		return
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	if err := daemon.SendReloadRequest(); err != nil {
		fmt.Printf("failed to reload config: %v\n", err)
		return
	}

	fmt.Println("config reload triggered")
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
	// Handle -h/--help before creating task
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintf(os.Stderr, `usage: anvil add [-p priority] [-s schedule] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [-f file | -] <task text>

Add a new task to the project.

Options:
  -p, --priority n        Priority 0-9 (default: 1)
  -s, --schedule cron     Cron schedule (e.g., "*/15 * * * *")
  --pre-check cmd        Command to run before task execution
  --allowed-tools tools  Comma-separated list of allowed tools
  --max-concurrent n     Max concurrent runs (default: 1)
  --skip-permissions     Skip permission checks
  -f, --file path        Read task content from a file
  -                      Read task content from stdin

Frontmatter in file/stdin input is merged with CLI flags (CLI flags take precedence).

Examples:
  anvil add "Review pull requests"
  anvil add -p 2 -s "0 9 * * *" "Daily standup notes"
  anvil add --pre-check "git diff --quiet" "Sync documentation"
  anvil add -s "*/30 * * * *" --file triage-prompt.md
  cat prompt.md | anvil add -s "*/30 * * * *" -
  anvil add -s "*/30 * * * *" <<'EOF'
  Check GitHub for new untriaged issues...
  EOF
`)
			os.Exit(0)
		}
	}
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
	case "pause":
		taskPauseCmd(args[1:])
	case "resume":
		taskResumeCmd(args[1:])
	case "queue":
		taskQueueCmd(args[1:])
	case "timeout":
		taskTimeoutCmd(args[1:])
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
	filePath := ""
	readStdin := false

	// Track which flags were explicitly set on the CLI so they take precedence over frontmatter.
	prioritySet := false
	scheduleSet := false
	preCheckSet := false
	allowedToolsSet := false
	maxConcurrentSet := false

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
			prioritySet = true
		case "-s", "--schedule":
			if i+1 >= len(args) {
				log.Fatal("missing value for -s/--schedule")
			}
			i++
			schedule = args[i]
			scheduleSet = true
		case "--pre-check":
			if i+1 >= len(args) {
				log.Fatal("missing value for --pre-check")
			}
			i++
			preCheck = args[i]
			preCheckSet = true
		case "--allowed-tools":
			if i+1 >= len(args) {
				log.Fatal("missing value for --allowed-tools")
			}
			i++
			allowedTools = args[i]
			allowedToolsSet = true
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
			maxConcurrentSet = true
		case "--skip-permissions":
			skipPermissions = true
		case "-f", "--file":
			if i+1 >= len(args) {
				log.Fatal("missing value for -f/--file")
			}
			i++
			filePath = args[i]
		case "-":
			readStdin = true
		default:
			rest = append(rest, args[i])
		}
	}

	var taskText string

	switch {
	case filePath != "":
		// Read task content from file.
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("reading file %s: %v", filePath, err)
		}
		taskText = string(data)
	case readStdin:
		// Read task content from stdin.
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("reading stdin: %v", err)
		}
		taskText = string(data)
	default:
		if len(rest) == 0 {
			log.Fatal("usage: anvil add [-p priority] [-s schedule] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [-f file | -] <task text>")
		}
		taskText = strings.Join(rest, " ")
	}

	// If file/stdin content has positional args too, that's an error.
	if (filePath != "" || readStdin) && len(rest) > 0 {
		log.Fatal("cannot combine file/stdin input with positional task text")
	}

	// Parse frontmatter from file/stdin content and merge with CLI flags.
	// CLI flags take precedence over frontmatter values.
	if filePath != "" || readStdin {
		taskText, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions = parseFrontmatterAndMerge(
			taskText, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions,
			prioritySet, scheduleSet, preCheckSet, allowedToolsSet, maxConcurrentSet,
		)
	}

	if strings.TrimSpace(taskText) == "" {
		log.Fatal("task content must not be empty")
	}

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

	// Warn if daemon is not running (only for scheduled tasks)
	if schedule != "" && !daemon.IsDaemonRunning() {
		fmt.Fprintf(os.Stderr, "⚠ Daemon is not running. Run 'anvil watch' to start executing tasks.\n")
	}
}

// parseFrontmatterAndMerge extracts YAML frontmatter from content and merges
// it with CLI flags. CLI flags take precedence: if a flag was explicitly set on
// the command line, the frontmatter value for that field is ignored.
// Returns the body (without frontmatter) and the merged configuration values.
func parseFrontmatterAndMerge(
	content string,
	priority int, schedule, preCheck, allowedTools string, maxConcurrent int, skipPermissions bool,
	prioritySet, scheduleSet, preCheckSet, allowedToolsSet, maxConcurrentSet bool,
) (string, int, string, string, string, int, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return content, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions
	}

	parts := strings.SplitN(content[4:], "\n---\n", 2)
	if len(parts) != 2 {
		// No closing delimiter found; treat entire content as body.
		return content, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions
	}

	fm := parts[0]
	body := parts[1]

	var fmData struct {
		Priority        *int   `yaml:"priority"`
		Schedule        string `yaml:"schedule"`
		PreCheck        string `yaml:"pre_check"`
		AllowedTools    string `yaml:"allowed_tools"`
		MaxConcurrent   *int   `yaml:"max_concurrent"`
		SkipPermissions bool   `yaml:"skip_permissions"`
	}
	if err := yaml.Unmarshal([]byte(fm), &fmData); err != nil {
		// If frontmatter is invalid YAML, treat the whole thing as body.
		return content, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions
	}

	// Merge: CLI flags take precedence over frontmatter.
	if !prioritySet && fmData.Priority != nil {
		p := *fmData.Priority
		if p >= 0 && p <= 9 {
			priority = p
		}
	}
	if !scheduleSet && fmData.Schedule != "" {
		schedule = fmData.Schedule
	}
	if !preCheckSet && fmData.PreCheck != "" {
		preCheck = fmData.PreCheck
	}
	if !allowedToolsSet && fmData.AllowedTools != "" {
		allowedTools = fmData.AllowedTools
	}
	if !maxConcurrentSet && fmData.MaxConcurrent != nil {
		maxConcurrent = *fmData.MaxConcurrent
	}
	if fmData.SkipPermissions {
		skipPermissions = true
	}

	return body, priority, schedule, preCheck, allowedTools, maxConcurrent, skipPermissions
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
			if t.Disabled {
				status = "disabled"
			} else if t.IsLocked {
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
	// Show next run time for scheduled tasks
	if todo.Schedule != "" {
		if p, err := cron.Parse(todo.Schedule); err == nil {
			if next, err := p.Next(time.Now()); err == nil {
				until := time.Until(next).Round(time.Minute)
				fmt.Printf("Next:     %s (%s from now)\n", next.Format("Mon 15:04"), until)
			}
		}
	}
	fmt.Printf("Priority: %d\n", todo.Priority)
	if todo.Disabled {
		fmt.Printf("Disabled: true\n")
	}
	fmt.Printf("Disabled: %t\n", todo.Disabled)
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
	followMode := false
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-n", "--limit":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "usage: anvil task history <name> [-n limit] [-f] [--failures] [--json]\n")
				os.Exit(1)
			}
			if _, err := fmt.Sscanf(args[i+1], "%d", &limit); err != nil {
				fmt.Fprintf(os.Stderr, "invalid limit: %s\n", args[i+1])
				os.Exit(1)
			}
			i += 2
		case "-f", "--follow":
			followMode = true
			i++
		case "--failures", "--show-failures-only":
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
		fmt.Fprintf(os.Stderr, "usage: anvil task history <name> [-n limit] [-f] [--failures] [--json]\n")
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

	// In follow mode, wait for new runs to complete and display them
	if followMode {
		runFollowMode(abs, todo)
		return
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

		// Print output summary if available
		if rec.OutputSummary != "" {
			summaryLines := strings.Split(rec.OutputSummary, "\n")
			for _, line := range summaryLines {
				fmt.Printf("  %s\n", line)
			}
		}
	}
}

// runFollowMode watches for new runs and prints them as they complete.
func runFollowMode(projectPath string, todo *project.Todo) {
	fmt.Printf("Following runs for task %s (Ctrl+C to exit)...\n\n", todo.Name)

	lastRunID := ""
	for {
		records, err := project.ReadAllRunRecords(projectPath, todo.ID)
		if err != nil || len(records) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		// Get the most recent run
		latest := records[0]

		// If this is a new run we haven't displayed yet
		if latest.RunID != lastRunID {
			lastRunID = latest.RunID

			// Wait for the run to complete (Finished time is set)
			for latest.Finished.IsZero() {
				time.Sleep(1 * time.Second)
				records, err = project.ReadAllRunRecords(projectPath, todo.ID)
				if err != nil || len(records) == 0 {
					break
				}
				latest = records[0]
			}

			// Display the completed run
			duration := ""
			if !latest.Finished.IsZero() {
				d := latest.Finished.Sub(latest.Started)
				if d < time.Minute {
					duration = fmt.Sprintf("%.0fs", d.Seconds())
				} else {
					duration = fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*float64(d.Minutes()))
				}
			}

			status := "ok"
			if !latest.Success {
				status = "failed"
			}

			fmt.Printf("RUN %s: %s %s %s\n",
				latest.RunID[:8],
				latest.Started.Format("2006-01-02 15:04"),
				duration,
				status,
			)

			if latest.OutputSummary != "" {
				summaryLines := strings.Split(latest.OutputSummary, "\n")
				for _, line := range summaryLines {
					fmt.Printf("  %s\n", line)
				}
			}
			fmt.Println()
		}

		time.Sleep(2 * time.Second)
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

func taskQueueCmd(_ []string) {
	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	tasks, err := daemon.SendQueueRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get queue status: %v\n", err)
		os.Exit(1)
	}

	if len(tasks) == 0 {
		fmt.Println("no tasks in queue")
		return
	}

	fmt.Printf("%-40s %-10s %-10s %s\n", "TASK", "PRIORITY", "STATUS", "SKIP REASON")
	fmt.Printf("%s\n", strings.Repeat("-", 90))

	for _, t := range tasks {
		skipReason := t.SkipReason
		if skipReason == "" {
			skipReason = "-"
		}
		fmt.Printf("%-40s %-10d %-10s %s\n",
			truncate(t.Name, 40),
			t.Priority,
			t.Status,
			skipReason)
	}
}

func taskTimeoutCmd(args []string) {
	allTasks := false
	for _, a := range args {
		if a == "--all" || a == "-a" {
			allTasks = true
		}
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	tasks, err := daemon.SendTimeoutRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get timeout status: %v\n", err)
		os.Exit(1)
	}

	if len(tasks) == 0 {
		fmt.Println("no running tasks")
		return
	}

	// Filter by task name if provided and not --all
	targetName := ""
	if !allTasks && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		targetName = args[0]
	}

	fmt.Printf("%-40s %-15s %-15s %-10s %s\n", "TASK", "ELAPSED", "TIMEOUT", "REMAINING", "PROGRESS")
	fmt.Printf("%s\n", strings.Repeat("-", 100))

	for _, t := range tasks {
		// Filter by target name if specified
		if targetName != "" && !strings.Contains(t.Name, targetName) {
			continue
		}

		elapsed := t.Elapsed
		timeout := t.Timeout
		remaining := t.TimeRemaining
		percent := t.PercentUsed

		fmt.Printf("%-40s %-15s %-15s %-10s %.1f%%\n",
			truncate(t.Name, 40),
			elapsed,
			timeout,
			remaining,
			percent)
	}
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

func daemonCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: anvil daemon <subcommand>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  log [-f] [-n lines]   View daemon log (-f to follow, -n for last N lines)")
		os.Exit(1)
	}
	switch args[0] {
	case "log":
		daemonLogCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func daemonLogCmd(args []string) {
	follow := false
	numLines := 50

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--follow":
			follow = true
		case "-n":
			if i+1 >= len(args) {
				log.Fatal("missing value for -n")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				log.Fatalf("invalid line count: %s", args[i])
			}
			numLines = n
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil daemon log [-f] [-n lines]

View the daemon log file (~/.anvil/daemon.log).

Options:
  -f, --follow   Follow the log (like tail -f)
  -n lines       Show last N lines (default 50)
`)
			os.Exit(0)
		}
	}

	logPath := config.DaemonLogPath()
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "no daemon log found (daemon has not run yet)")
		os.Exit(1)
	}

	// Read and display the last N lines.
	data, err := os.ReadFile(logPath)
	if err != nil {
		log.Fatalf("failed to read daemon log: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	// Remove trailing empty line from Split.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	start := 0
	if len(lines) > numLines {
		start = len(lines) - numLines
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	if !follow {
		return
	}

	// Follow mode: tail the file for new content.
	f, err := os.Open(logPath)
	if err != nil {
		log.Fatalf("failed to open daemon log: %v", err)
	}
	defer f.Close()

	// Seek to end.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		log.Fatalf("failed to seek: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	buf := make([]byte, 4096)
	for {
		select {
		case <-sigCh:
			return
		default:
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			continue
		}
		if readErr != nil && readErr != io.EOF {
			return
		}

		// Check if the file was rotated (new file at same path).
		newInfo, statErr := os.Stat(logPath)
		if statErr == nil {
			curInfo, _ := f.Stat()
			if curInfo != nil && !os.SameFile(curInfo, newInfo) {
				// File was rotated — reopen.
				f.Close()
				f, err = os.Open(logPath)
				if err != nil {
					return
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
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

func taskPauseCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task pause <name>\n")
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

	if todo.Disabled {
		fmt.Printf("task %s is already paused\n", args[0])
		return
	}

	// Read current file content
	raw, err := os.ReadFile(todo.Path)
	if err != nil {
		log.Fatalf("failed to read task file: %v", err)
	}

	// Parse and update front-matter to add disabled: true
	contentStr := string(raw)
	body := contentStr

	if strings.HasPrefix(contentStr, "---\n") {
		parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
		if len(parts) == 2 {
			fm := parts[0]
			body = parts[1]

			var fmData struct {
				Schedule            string   `yaml:"schedule"`
				ID                  string   `yaml:"id"`
				Resume              *bool    `yaml:"resume"`
				MaxConcurrent       int      `yaml:"max_concurrent"`
				SkipPermissions     bool     `yaml:"skip_permissions"`
				AllowedTools        []string `yaml:"allowed_tools"`
				PreCheck            string   `yaml:"pre_check"`
				OnSuccess           string   `yaml:"on_success"`
				OnFailure           string   `yaml:"on_failure"`
				Disabled            bool     `yaml:"disabled"`
				Timeout             string   `yaml:"timeout"`
				Retry               int      `yaml:"retry"`
				RetryDelay          string   `yaml:"retry_delay"`
				PersistentCooldown  string   `yaml:"persistent_cooldown"`
				PersistentMaxRuntime string `yaml:"persistent_max_runtime"`
			}
			if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
				// Set disabled to true
				fmData.Disabled = true

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

				// Write back in place
				if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
					log.Fatalf("failed to write task file: %v", err)
				}

				fmt.Printf("paused: %s\n", args[0])
				return
			}
		}
	}

	// No front-matter exists, create one with disabled: true
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("disabled: true\n")
	sb.WriteString("---\n")
	sb.WriteString(contentStr)

	if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
		log.Fatalf("failed to write task file: %v", err)
	}

	fmt.Printf("paused: %s\n", args[0])
}

func taskResumeCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task resume <name>\n")
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

	if !todo.Disabled {
		fmt.Printf("task %s is not paused\n", args[0])
		return
	}

	// Read current file content
	raw, err := os.ReadFile(todo.Path)
	if err != nil {
		log.Fatalf("failed to read task file: %v", err)
	}

	// Parse and update front-matter to remove disabled field
	contentStr := string(raw)
	body := contentStr

	if strings.HasPrefix(contentStr, "---\n") {
		parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
		if len(parts) == 2 {
			fm := parts[0]
			body = parts[1]

			var fmData struct {
				Schedule            string   `yaml:"schedule"`
				ID                  string   `yaml:"id"`
				Resume              *bool    `yaml:"resume"`
				MaxConcurrent       int      `yaml:"max_concurrent"`
				SkipPermissions     bool     `yaml:"skip_permissions"`
				AllowedTools        []string `yaml:"allowed_tools"`
				PreCheck            string   `yaml:"pre_check"`
				OnSuccess           string   `yaml:"on_success"`
				OnFailure           string   `yaml:"on_failure"`
				Disabled            bool     `yaml:"disabled"`
				Timeout             string   `yaml:"timeout"`
				Retry               int      `yaml:"retry"`
				RetryDelay          string   `yaml:"retry_delay"`
				PersistentCooldown  string   `yaml:"persistent_cooldown"`
				PersistentMaxRuntime string `yaml:"persistent_max_runtime"`
			}
			if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
				// Set disabled to false (YAML will omit if false when marshaling)
				fmData.Disabled = false

				// Marshal back
				fmBytes, err := yaml.Marshal(fmData)
				if err != nil {
					log.Fatalf("failed to marshal front-matter: %v", err)
				}

				// Build new file content (without the disabled line if it's false)
				fmStr := string(fmBytes)
				// Remove "disabled: false\n" if present
				fmStr = strings.ReplaceAll(fmStr, "disabled: false\n", "")
				fmStr = strings.ReplaceAll(fmStr, "disabled: false", "")

				var sb strings.Builder
				sb.WriteString("---\n")
				sb.WriteString(fmStr)
				sb.WriteString("---\n")
				sb.WriteString(body)

				// Write back in place
				if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
					log.Fatalf("failed to write task file: %v", err)
				}

				fmt.Printf("resumed: %s\n", args[0])
				return
			}
		}
	}

	fmt.Printf("task %s has no front-matter to resume\n", args[0])
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

	// Refresh skills in all watched directories
	watched, err := loadAllWatched()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load watched directories: %v\n", err)
	} else if len(watched) > 0 {
		fmt.Printf("refreshing skills in %d watched director%s...\n", len(watched), map[bool]string{true: "ies", false: "y"}[len(watched) > 1])
		for _, w := range watched {
			// Check if the directory still exists
			if _, err := os.Stat(w.Path); os.IsNotExist(err) {
				fmt.Printf("  skipping %s: directory no longer exists\n", w.Path)
				continue
			}
			if err := project.Init(w.Path, tools.FS); err != nil {
				fmt.Printf("  failed to refresh %s: %v\n", w.Path, err)
				continue
			}
			fmt.Printf("  refreshed %s\n", w.Path)
		}
	}
}

func usageCmd(args []string) {
	var projectFilter string
	var taskFilter string
	var sinceStr string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				projectFilter = args[i+1]
				i += 2
			} else {
				fmt.Fprintln(os.Stderr, "--project requires a value")
				os.Exit(1)
			}
		case "--task":
			if i+1 < len(args) {
				taskFilter = args[i+1]
				i += 2
			} else {
				fmt.Fprintln(os.Stderr, "--task requires a value")
				os.Exit(1)
			}
		case "--since":
			if i+1 < len(args) {
				sinceStr = args[i+1]
				i += 2
			} else {
				fmt.Fprintln(os.Stderr, "--since requires a value")
				os.Exit(1)
			}
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `Usage: anvil usage [options]

Show LLM token usage and estimated costs across tasks and projects.

Options:
  --project <path>   Filter to a specific project (default: all watched projects)
  --task <name>      Filter to a specific task name
  --since <date>     Show usage since date (YYYY-MM-DD, default: 7 days ago)
`)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	// Default: last 7 days
	since := time.Now().AddDate(0, 0, -7)
	if sinceStr != "" {
		parsed, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid date %q (expected YYYY-MM-DD): %v\n", sinceStr, err)
			os.Exit(1)
		}
		since = parsed
	}

	// Resolve project filter to absolute path
	if projectFilter != "" {
		abs, err := filepath.Abs(projectFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad project path: %v\n", err)
			os.Exit(1)
		}
		projectFilter = abs
	}

	// Discover projects
	var projectPaths []string
	if projectFilter != "" {
		projectPaths = []string{projectFilter}
	} else {
		watched, err := loadAllWatched()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load watched projects: %v\n", err)
			os.Exit(1)
		}
		for _, w := range watched {
			projectPaths = append(projectPaths, w.Path)
		}
	}

	if len(projectPaths) == 0 {
		fmt.Println("No watched projects found.")
		return
	}

	// Load global config for cost rates
	cfg, _ := config.Load()
	inputRate := cfg.InputTokenRate
	outputRate := cfg.OutputTokenRate
	if inputRate <= 0 {
		inputRate = 3.0
	}
	if outputRate <= 0 {
		outputRate = 15.0
	}

	type taskUsage struct {
		Project      string
		TaskName     string
		Runs         int
		InputTokens  int
		OutputTokens int
		Cost         float64
	}

	var allUsage []taskUsage
	var totalRuns int
	var totalInput, totalOutput int
	var totalCost float64

	for _, projPath := range projectPaths {
		proj, err := project.Load(projPath)
		if err != nil {
			continue
		}
		todos, err := proj.LoadTodos()
		if err != nil {
			continue
		}
		projName := filepath.Base(projPath)

		for _, todo := range todos {
			if taskFilter != "" && todo.Name != taskFilter {
				continue
			}
			records, err := project.ReadAllRunRecords(projPath, todo.ID)
			if err != nil {
				continue
			}

			var tu taskUsage
			tu.Project = projName
			tu.TaskName = todo.Name

			for _, rec := range records {
				if rec.Started.Before(since) {
					continue
				}
				tu.Runs++
				tu.InputTokens += rec.InputTokens
				tu.OutputTokens += rec.OutputTokens
				tu.Cost += rec.EstimatedCostUSD
			}

			// Recalculate cost if records had zero cost but had tokens
			// (handles old records that may have tokens but no cost)
			if tu.Cost == 0 && (tu.InputTokens > 0 || tu.OutputTokens > 0) {
				tu.Cost = float64(tu.InputTokens)/1_000_000*inputRate +
					float64(tu.OutputTokens)/1_000_000*outputRate
			}

			if tu.Runs > 0 {
				allUsage = append(allUsage, tu)
				totalRuns += tu.Runs
				totalInput += tu.InputTokens
				totalOutput += tu.OutputTokens
				totalCost += tu.Cost
			}
		}
	}

	if totalRuns == 0 {
		fmt.Printf("No runs found since %s.\n", since.Format("2006-01-02"))
		return
	}

	// Sort by cost descending
	sort.Slice(allUsage, func(i, j int) bool {
		return allUsage[i].Cost > allUsage[j].Cost
	})

	// Print summary
	fmt.Printf("Token usage since %s\n", since.Format("2006-01-02"))
	fmt.Printf("Rates: $%.2f/1M input, $%.2f/1M output\n\n", inputRate, outputRate)

	fmt.Printf("%-30s %-6s %12s %12s %10s\n", "TASK", "RUNS", "INPUT", "OUTPUT", "COST")
	fmt.Printf("%-30s %-6s %12s %12s %10s\n", "----", "----", "-----", "------", "----")

	limit := len(allUsage)
	if limit > 15 {
		limit = 15
	}
	for _, tu := range allUsage[:limit] {
		name := tu.TaskName
		if len(name) > 27 {
			name = name[:27] + "..."
		}
		if len(projectPaths) > 1 {
			label := tu.Project + "/" + name
			if len(label) > 30 {
				label = label[:27] + "..."
			}
			name = label
		}
		fmt.Printf("%-30s %-6d %12s %12s %10s\n",
			name,
			tu.Runs,
			formatTokens(tu.InputTokens),
			formatTokens(tu.OutputTokens),
			formatCost(tu.Cost))
	}
	if len(allUsage) > limit {
		fmt.Printf("  ... and %d more tasks\n", len(allUsage)-limit)
	}

	fmt.Printf("\n%-30s %-6d %12s %12s %10s\n",
		"TOTAL",
		totalRuns,
		formatTokens(totalInput),
		formatTokens(totalOutput),
		formatCost(totalCost))
}

func formatTokens(n int) string {
	if n == 0 {
		return "N/A"
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCost(c float64) string {
	if c == 0 {
		return "N/A"
	}
	if c < 0.01 {
		return fmt.Sprintf("$%.4f", c)
	}
	return fmt.Sprintf("$%.2f", c)
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
