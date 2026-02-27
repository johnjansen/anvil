package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/internal/service"
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
		statusCmd(os.Args[2:])
	case "cleanup":
		cleanupCmd(os.Args[2:])
	case "ps":
		psCmd(os.Args[2:])
	case "init":
		initCmd(os.Args[2:])
	case "register":
		registerCmd(os.Args[2:])
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
	case "prompt":
		promptCmd(os.Args[2:])
	case "project":
		projectCmd(os.Args[2:])
	case "template":
		templateCmd(os.Args[2:])
	case "usage":
		usageCmd(os.Args[2:])
	case "update":
		updateCmd(os.Args[2:])
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
  init [--force] [path]    Initialize a project and register it for watching
  register [path]          Register a project for watching (without full init)
  watch [-d|--daemonize]   Start the daemon (press 'd' to detach to background)
  watch --install          Install as system service (auto-start on boot)
  watch --uninstall        Remove the system service
  watch --status           Show system service status
  watch --stop [--graceful] Stop the daemon (--graceful waits for tasks, --force kills)
  add [options] <task>     Add a task to the current project
  logs [<name>]            Raw worker output (all tasks if no name given)
  ps [--json] [-w|--watch] Show running tasks (--watch for live dashboard)
  status [--json]          Show watched projects and daemon status
  project <subcommand>     Project management commands
  daemon <subcommand>      Daemon management commands
  prompt <subcommand>    Prompt testing and validation tools
  update [--check]         Update anvil to the latest release
  reload [--graceful]       Reload daemon configuration (--graceful waits for tasks)
  version                  Show version

Add options:
  -p, --priority int          Task priority 0-9 (default 1)
  -s, --schedule string      Cron schedule (e.g., "*/15 * * * *"), "" for one-shot
  -o, --once                 Create a one-shot task (no schedule)
  -n, --dry-run              Validate schedule without creating task
  -f, --file path            Read task content from a file
  -t, --template name        Use a template for task configuration
  -                          Read task content from stdin
      --pre-check string    Shell command to skip task if non-zero exit
      --allowed-tools string  Comma-separated tool allowlist (e.g. "Bash,Read") or scoped (e.g. "Bash(gh:*)")
      --max-concurrent int    Max parallel instances (default 1)
      --skip-permissions     Bypass all tool permission prompts
      --strict               Fail if schedule conflicts with existing tasks
      --no-overlap-check    Skip schedule overlap detection

Task subcommands:
  create [options] <task>   Create a new task
  ls [-a|--all] [--json] [--label L]  List tasks (--all for all projects, --label to filter)
  find <pattern>            Find tasks by name pattern (alias for ls --match)
  get <name> [--json]       Show task details including run status
  log [-f] <name>           Show execution log (-f to follow)
  history <name> [--json]    Show run history
  rm <name>                 Remove a task (kills if running)
  run <name>                Trigger immediate execution (bypass cron)
  kill <name>               Kill a running task (persistent tasks auto-restart)
  stop <name>               Stop a persistent task permanently (kill + prevent restart)
  start <name>              Start a stopped persistent task (dispatches on next tick)
  stop-on-idle <name>       Finish current run then stop rescheduling task
  unlock <name>             Remove stale lock file to allow retry
  queue [--json]            Show daemon queue status and skip reasons
  pause <name>              Pause a task (sets disabled: true)
  resume <name>             Resume a paused task (sets disabled: false)
  edit <name>               Edit task (schedule, priority, content, labels, or --remove field)
  timeout [name]            Show task timeout progress (--all for all tasks)
  extend-timeout <name> <dur>  Extend a running task's timeout by the given duration
  next [name]              Show next scheduled run time (--all for all projects)
  wait <name> [--timeout D]  Block until a running task completes (exit 0=ok, 1=fail, 2=timeout)
  analyze [-a|--all]         Detect scheduling conflicts and overlapping tasks
  pipeline [--dot|--verbose] [--all]  Visualize task dependency pipelines
  reset-budget <name>        Reset persistent task budget consumption
  state <name>              View, export, import, or clear task state
  dry-run <name> [options]   Validate and preview task config without executing
  sla [--verbose] [--reset] [--json]  Show SLA violations (--verbose for all, --reset to clear)
  export [names...] [-a] [-o file]  Export tasks to JSON for sharing or backup
  import <file> [options]   Import tasks from a JSON export file
  predict <name>            Show failure prediction analysis based on historical runs
  tree <name> [--depth N] [--json]  Show parent-child relationships for spawned tasks

Project subcommands:
  create [path]            Initialize and watch a project in one step
  ls [-a|--all] [--json]   List watched projects
  get [path]               Show project details and running tasks
  rm [path] [--clean]      Unwatch a project (--clean removes .anvil/ too)

Daemon subcommands:
  log [-f] [-n lines] [--level LEVEL] [--match PATTERN] [--since TIME] [--until TIME]
                      View daemon log (filtering options for level, match, since, until)
  config-validate [--show]  Validate config file (--show to display parsed config)

Configuration:
  ~/.anvil/config.yaml   Daemon config
  <project>/.anvil/      Project config and todos
`)
}

func initCmd(args []string) {
	path := "."
	force := false
	var filtered []string
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
		} else {
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

	result, err := project.Init(abs, tools.FS, force)
	if err != nil {
		log.Fatalf("failed to init project: %v", err)
	}

	if result.AlreadyExists {
		if result.BackupPath != "" {
			fmt.Printf("backed up %d existing task(s) to %s\n", result.TaskCount, result.BackupPath)
		} else {
			fmt.Printf("existing project with %d task(s) preserved\n", result.TaskCount)
		}
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

func registerCmd(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Verify the directory exists
	info, err := os.Stat(abs)
	if err != nil {
		log.Fatalf("path does not exist: %s", abs)
	}
	if !info.IsDir() {
		log.Fatalf("not a directory: %s", abs)
	}

	registerProject(abs)

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

	// Enter raw terminal mode BEFORE starting the daemon so that all output
	// uses \r\n from the very first line. Raw mode disables OPOST, which means
	// bare \n no longer returns the cursor to column 0.
	fd := int(os.Stdin.Fd())
	oldTermState, termErr := term.MakeRaw(fd)
	if termErr == nil {
		origWriter := log.Writer()
		log.SetOutput(&rawLineWriter{w: origWriter})
		daemon.SetRawMode(true)
		defer func() {
			term.Restore(fd, oldTermState)
			log.SetOutput(origWriter)
			daemon.SetRawMode(false)
		}()
	}

	go d.Run()

	// Start hot-daemonize listener in background
	detachCh := make(chan struct{})
	go func() {
		if termErr != nil {
			// Not a terminal, skip hot-daemonize
			return
		}

		buf := make([]byte, 1)
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		// 'd' key or Ctrl+D (ASCII 4) triggers detach
		if buf[0] == 'd' || buf[0] == 4 {
			close(detachCh)
		}
	}()

	select {
	case sig := <-sigCh:
		log.Printf("received %v, shutting down", sig)
		d.Stop()
	case <-d.Done():
		// daemon stopped itself (e.g. stop-on-idle drain completed)
	case <-detachCh:
		// User pressed 'd' or Ctrl+D to hot-daemonize
		log.Printf("detaching to background...")
		detachToBackground(d)
	}
}

// watchCmd2 handles "anvil watch" with optional --daemonize/-d, --stop, --restart, and --child flags.
func watchCmd2(args []string) {
	daemonize := false
	stop := false
	restart := false
	child := false
	install := false
	uninstall := false
	status := false
	graceful := false
	force := false
	gracefulTimeout := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--daemonize", "-d":
			daemonize = true
		case "--stop":
			stop = true
		case "--restart":
			restart = true
		case "--child":
			child = true
		case "--install":
			install = true
		case "--uninstall":
			uninstall = true
		case "--status":
			status = true
		case "--graceful", "-g":
			graceful = true
		case "--force":
			force = true
		case "--timeout":
			if i+1 < len(args) {
				i++
				gracefulTimeout = args[i]
			}
		default:
			if strings.HasPrefix(args[i], "--graceful=") {
				graceful = true
				gracefulTimeout = strings.TrimPrefix(args[i], "--graceful=")
			} else if strings.HasPrefix(args[i], "--timeout=") {
				gracefulTimeout = strings.TrimPrefix(args[i], "--timeout=")
			}
		}
	}

	if install {
		watchInstall()
		return
	}

	if uninstall {
		watchUninstall()
		return
	}

	if status {
		watchStatus()
		return
	}

	if stop {
		if graceful {
			stopDaemonGraceful(gracefulTimeout)
		} else {
			stopDaemon(force)
		}
		return
	}

	if restart {
		restartDaemon(gracefulTimeout)
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

func watchInstall() {
	svc, err := service.New()
	if err != nil {
		log.Fatalf("failed to initialize service manager: %v", err)
	}

	binaryPath, err := os.Executable()
	if err != nil {
		log.Fatalf("cannot determine binary path: %v", err)
	}
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		log.Fatalf("cannot resolve binary path: %v", err)
	}

	if err := svc.Install(binaryPath); err != nil {
		log.Fatalf("failed to install service: %v", err)
	}

	fmt.Printf("service installed and started\n")
	fmt.Printf("  binary: %s\n", binaryPath)
	fmt.Printf("  anvil watch will now auto-start on boot and restart on crash\n")
}

func watchUninstall() {
	svc, err := service.New()
	if err != nil {
		log.Fatalf("failed to initialize service manager: %v", err)
	}

	if err := svc.Uninstall(); err != nil {
		log.Fatalf("failed to uninstall service: %v", err)
	}

	fmt.Println("service uninstalled")
}

func watchStatus() {
	svc, err := service.New()
	if err != nil {
		log.Fatalf("failed to initialize service manager: %v", err)
	}

	st, err := svc.Status()
	if err != nil {
		log.Fatalf("failed to get service status: %v", err)
	}

	fmt.Printf("service: %s\n", st.Message)
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

// detachToBackground starts a new daemon in the background, then stops the foreground daemon.
// This allows the foreground daemon to drain its work queue before exiting.
// The new daemon continues accepting new tasks.
func detachToBackground(d *daemon.Daemon) {
	// First, start the background daemon
	if err := config.EnsureDir(); err != nil {
		log.Printf("failed to create ~/.anvil: %v", err)
		return
	}

	// Get the executable path
	exe, err := os.Executable()
	if err != nil {
		log.Printf("failed to find executable path: %v", err)
		return
	}

	// Open the log file for the detached daemon
	logFile, err := os.OpenFile(config.DaemonLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("failed to open daemon log: %v", err)
		return
	}

	// Start the new daemon process in a new session
	// Use --child flag which runs the daemon without writing PID file (daemon.Run does that)
	cmd := exec.Command(exe, "watch", "--child")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		log.Printf("failed to start background daemon: %v", err)
		return
	}

	newPID := cmd.Process.Pid
	logFile.Close()

	// Now stop the foreground daemon gracefully (allows draining work queue)
	log.Printf("stopping foreground daemon to detach (background PID: %d)...", newPID)
	d.Stop()

	// Print detach message to stderr
	fmt.Fprintf(os.Stderr, "Detached to background (PID %d). Use 'anvil ps' to monitor.\n", newPID)
}

// runDaemonChild is the internal entry point for the detached child process.
func runDaemonChild() {
	if err := config.EnsureDir(); err != nil {
		log.Fatalf("failed to create ~/.anvil: %v", err)
	}

	// PID file is written by daemon.Run() via checkAndWritePID().
	// Do NOT write it here — that causes Run() to detect our own PID
	// as an already-running daemon and exit without starting the socket server.

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
func stopDaemon(force bool) {
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

	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}

	if err := proc.Signal(sig); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop daemon: %v\n", err)
		return
	}

	if force {
		fmt.Fprintf(os.Stderr, "daemon force-killed (PID %d)\n", pid)
		os.Remove(config.PidFile())
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

func stopDaemonGraceful(timeoutStr string) {
	pid := readDaemonPID()
	if pid == 0 {
		fmt.Fprintln(os.Stderr, "no daemon running")
		return
	}

	timeoutDur := 5 * time.Minute
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			log.Fatalf("invalid timeout: %s (use format like 5m, 30s, 1h)", timeoutStr)
		}
		timeoutDur = d
	}

	// Check running tasks before stopping
	if daemon.IsDaemonRunning() {
		tasks, err := daemon.SendPsRequest()
		if err == nil && len(tasks) == 0 {
			// No running tasks — stop immediately
			stopDaemon(false)
			return
		}

		if err == nil && len(tasks) > 0 {
			fmt.Fprintf(os.Stderr, "Gracefully stopping daemon...\nWaiting for %d running task(s) to complete:\n", len(tasks))
			for _, t := range tasks {
				fmt.Fprintf(os.Stderr, "  - %s/%s (running %s)\n", t.Project, t.Name, t.Elapsed)
			}
			fmt.Fprintf(os.Stderr, "Press Ctrl+C to force stop\n\n")
		}
	}

	// Send SIGTERM — daemon will do graceful shutdown internally
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no daemon running")
		return
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop daemon: %v\n", err)
		return
	}

	// Poll until daemon stops or timeout expires
	deadline := time.After(timeoutDur + 10*time.Second) // extra 10s buffer over daemon's own timeout
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	startTime := time.Now()

	for {
		select {
		case <-deadline:
			fmt.Fprintf(os.Stderr, "\nTimeout reached. Force-killing daemon (PID %d)...\n", pid)
			proc.Signal(syscall.SIGKILL)
			os.Remove(config.PidFile())
			return
		case <-ticker.C:
			if _, err := os.Stat(config.PidFile()); os.IsNotExist(err) {
				elapsed := time.Since(startTime).Round(time.Second)
				fmt.Fprintf(os.Stderr, "\ndaemon stopped gracefully (PID %d, waited %s)\n", pid, elapsed)
				return
			}
			// Show progress
			if daemon.IsDaemonRunning() {
				tasks, err := daemon.SendPsRequest()
				if err == nil {
					elapsed := time.Since(startTime).Round(time.Second)
					remaining := timeoutDur - time.Since(startTime)
					if remaining < 0 {
						remaining = 0
					}
					fmt.Fprintf(os.Stderr, "\r  %d task(s) still running... (%s elapsed, %s remaining)  ", len(tasks), elapsed, remaining.Round(time.Second))
				}
			}
		}
	}
}

func restartDaemon(timeoutStr string) {
	pid := readDaemonPID()
	if pid == 0 {
		fmt.Fprintln(os.Stderr, "no daemon running, starting fresh...")
		daemonizeProcess()
		return
	}

	fmt.Println("Restarting daemon...")

	// Use graceful stop if daemon is running
	stopDaemonGraceful(timeoutStr)

	// Small delay to ensure clean shutdown
	time.Sleep(500 * time.Millisecond)

	// Start the daemon again
	fmt.Println("Starting daemon...")
	daemonizeProcess()
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
		if _, err := project.Init(abs, tools.FS, false); err != nil {
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

func statusCmd(args []string) {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		}
	}

	watched, err := loadAllWatched()
	if err != nil {
		log.Fatalf("failed to read watched: %v", err)
	}

	daemonRunning := daemon.IsDaemonRunning()
	draining := false
	if daemonRunning {
		if status, err := daemon.SendStatusRequest(); err == nil && status.Draining {
			draining = true
		}
	}

	if jsonOutput {
		type projectStatusJSON struct {
			Path  string `json:"path"`
			Tasks int    `json:"tasks"`
			Error string `json:"error,omitempty"`
		}
		type statusJSON struct {
			DaemonRunning bool                `json:"daemon_running"`
			Draining      bool                `json:"draining"`
			Projects      []projectStatusJSON `json:"projects"`
		}
		st := statusJSON{
			DaemonRunning: daemonRunning,
			Draining:      draining,
			Projects:      []projectStatusJSON{},
		}
		for _, w := range watched {
			proj, err := project.Load(w.Path)
			if err != nil {
				st.Projects = append(st.Projects, projectStatusJSON{
					Path:  w.Path,
					Error: err.Error(),
				})
				continue
			}
			todos, _ := proj.LoadTodos()
			st.Projects = append(st.Projects, projectStatusJSON{
				Path:  w.Path,
				Tasks: len(todos),
			})
		}
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Show daemon drain state if running
	if draining {
		fmt.Println("daemon: draining (stop-on-idle active)")
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
	graceful := false
	timeoutStr := ""

	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil reload [--graceful] [--timeout duration]

Reload the daemon configuration without restarting.

Sends SIGHUP to the daemon to reload ~/.anvil/config.yaml.
New tasks will use the updated config; running tasks are unaffected.

Options:
  --graceful           Wait for running tasks to complete before reloading
  --timeout duration   Max time to wait for tasks (default: 5m). Forces reload after timeout.
`)
			return
		case "--graceful":
			graceful = true
		default:
			if strings.HasPrefix(a, "--timeout=") {
				timeoutStr = strings.TrimPrefix(a, "--timeout=")
			} else if strings.HasPrefix(a, "--timeout") {
				// handled below with next arg
			}
		}
	}

	// Parse --timeout value (may be space-separated)
	for i := 0; i < len(args); i++ {
		if args[i] == "--timeout" && i+1 < len(args) {
			timeoutStr = args[i+1]
			break
		}
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	if !graceful {
		// Immediate reload (original behavior)
		if err := daemon.SendReloadRequest(); err != nil {
			fmt.Printf("failed to reload config: %v\n", err)
			return
		}
		fmt.Println("config reload triggered")
		return
	}

	// Graceful reload: wait for running tasks to complete first
	timeoutDur := 5 * time.Minute
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			log.Fatalf("invalid timeout duration: %s (use format like 5m, 30s, 1h)", timeoutStr)
		}
		timeoutDur = d
	}

	// Check for running tasks
	tasks, err := daemon.SendPsRequest()
	if err != nil {
		log.Fatalf("failed to check running tasks: %v", err)
	}

	if len(tasks) == 0 {
		// No running tasks, reload immediately
		if err := daemon.SendReloadRequest(); err != nil {
			fmt.Printf("failed to reload config: %v\n", err)
			return
		}
		fmt.Println("no running tasks — config reload triggered")
		return
	}

	fmt.Printf("Waiting for %d running task(s) to complete (timeout: %s)...\n", len(tasks), timeoutDur)
	for _, t := range tasks {
		fmt.Printf("  - %s/%s (running %s)\n", t.Project, t.Name, t.Elapsed)
	}

	deadline := time.After(timeoutDur)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	startTime := time.Now()

	for {
		select {
		case <-deadline:
			fmt.Fprintf(os.Stderr, "\nTimeout reached (%s). Force-reloading daemon...\n", timeoutDur)
			if err := daemon.SendReloadRequest(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to reload config: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("config reload triggered (forced after timeout)")
			return
		case <-ticker.C:
			tasks, err := daemon.SendPsRequest()
			if err != nil {
				fmt.Fprintf(os.Stderr, "daemon unreachable: %v\n", err)
				os.Exit(1)
			}
			if len(tasks) == 0 {
				// All tasks finished
				if err := daemon.SendReloadRequest(); err != nil {
					fmt.Fprintf(os.Stderr, "failed to reload config: %v\n", err)
					os.Exit(1)
				}
				elapsed := time.Since(startTime).Round(time.Second)
				fmt.Printf("\nAll tasks completed (%s). Config reload triggered.\n", elapsed)
				return
			}
			elapsed := time.Since(startTime).Round(time.Second)
			remaining := timeoutDur - time.Since(startTime)
			if remaining < 0 {
				remaining = 0
			}
			fmt.Printf("\r  %d task(s) still running... (%s elapsed, %s remaining)  ", len(tasks), elapsed, remaining.Round(time.Second))
		}
	}
}

func cleanupCmd(args []string) {
	olderThan := ""
	dryRun := false

	for _, a := range args {
		if a == "--dry-run" || a == "-n" {
			dryRun = true
		} else if strings.HasPrefix(a, "--older-than=") {
			olderThan = strings.TrimPrefix(a, "--older-than=")
		} else if strings.HasPrefix(a, "-o=") {
			olderThan = strings.TrimPrefix(a, "-o=")
		}
	}

	var maxAge time.Duration
	if olderThan != "" {
		var err error
		maxAge, err = time.ParseDuration(olderThan)
		if err != nil {
			log.Fatalf("invalid duration: %s (use format like 7d, 24h)", olderThan)
		}
	}

	watched, err := loadAllWatched()
	if err != nil {
		log.Fatalf("failed to read watched: %v", err)
	}

	if len(watched) == 0 {
		fmt.Println("no watched projects")
		return
	}

	// If no retention config and no --older-than, show current config
	cfg, _ := config.Load()
	if maxAge == 0 && cfg.Retention.MaxAge == 0 && cfg.Retention.MaxRuns == 0 {
		fmt.Println("No retention policy configured. Set in ~/.anvil/config.yaml:")
		fmt.Println("  retention:")
		fmt.Println("    max_age: 7d")
		fmt.Println("    max_runs: 50")
		fmt.Println("")
		fmt.Println("Or use --older-than to prune manually:")
		fmt.Println("  anvil cleanup --older-than=3d")
		return
	}

	action := "Would prune"
	if dryRun {
		action = "Would prune"
	} else {
		action = "Pruned"
	}

	totalFreed := 0

	for _, w := range watched {
		// Prune logs
		logsDir := filepath.Join(w.Path, ".anvil", "logs")
		if _, err := os.Stat(logsDir); err == nil {
			count, freed := pruneDir(logsDir, maxAge, 0, dryRun)
			totalFreed += freed
			if count > 0 {
				fmt.Printf("%s %d log files from %s\n", action, count, w.Path)
			}
		}

		// Prune runs
		runsDir := filepath.Join(w.Path, ".anvil", "runs")
		if _, err := os.Stat(runsDir); err == nil {
			count, freed := pruneDir(runsDir, maxAge, cfg.Retention.MaxRuns, dryRun)
			totalFreed += freed
			if count > 0 {
				fmt.Printf("%s %d run files from %s\n", action, count, w.Path)
			}
		}
	}

	if dryRun {
		fmt.Printf("Total space that would be freed: %d bytes\n", totalFreed)
		fmt.Println("(use without --dry-run to actually delete)")
	} else {
		fmt.Printf("Total space freed: %d bytes\n", totalFreed)
	}
}

func pruneDir(dir string, maxAge time.Duration, maxRuns int, dryRun bool) (int, int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}

	// Collect task directories
	var taskDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			taskDirs = append(taskDirs, filepath.Join(dir, entry.Name()))
		}
	}

	deleted := 0
	freed := 0

	for _, taskDir := range taskDirs {
		taskEntries, err := os.ReadDir(taskDir)
		if err != nil {
			continue
		}

		// Collect files with modification times
		type fileInfo struct {
			name    string
			path    string
			size    int64
			modTime time.Time
		}

		var files []fileInfo
		for _, entry := range taskEntries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{
				name:    entry.Name(),
				path:    filepath.Join(taskDir, entry.Name()),
				size:    info.Size(),
				modTime: info.ModTime(),
			})
		}

		if len(files) == 0 {
			continue
		}

		// Sort by modification time (oldest first)
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime.Before(files[j].modTime)
		})

		now := time.Now()
		cutoff := now.Add(-maxAge)

		// Mark files to delete by age
		toDelete := make(map[string]fileInfo)
		if maxAge > 0 {
			for _, f := range files {
				if f.modTime.Before(cutoff) {
					toDelete[f.path] = f
				}
			}
		}

		// Mark files to delete by count (keep maxRuns newest)
		if maxRuns > 0 && len(files) > maxRuns {
			for i := 0; i < len(files)-maxRuns; i++ {
				f := files[i]
				toDelete[f.path] = f
			}
		}

		// Delete marked files
		for _, f := range toDelete {
			if !dryRun {
				if err := os.Remove(f.path); err == nil {
					deleted++
					freed += int(f.size)
				}
			} else {
				deleted++
				freed += int(f.size)
			}
		}
	}

	return deleted, freed
}

func psCmd(args []string) {
	jsonOutput := false
	watchMode := false
	for _, a := range args {
		switch a {
		case "--json":
			jsonOutput = true
		case "--watch", "-f":
			watchMode = true
		}
	}

	if watchMode {
		psWatch()
		return
	}


	if !daemon.IsDaemonRunning() {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("daemon not running")
		}
		return
	}

	// Show drain status header if applicable (text mode only)
	if !jsonOutput {
		if status, err := daemon.SendStatusRequest(); err == nil {
			if status.Draining {
				fmt.Println("(draining — no new tasks will be dispatched)")
			}
			// Show rate limit status if configured
			if status.RateLimited {
				slots := status.RateLimitSlots
				inUse := status.RateInUse
				pct := 0
				if slots > 0 {
					pct = (inUse * 100) / slots
				}
				bar := strings.Repeat("█", pct/5) + strings.Repeat("░", 20-pct/5)
				fmt.Printf("Rate limit: [%s] %d/%d\n", bar, inUse, slots)
			}
		}
	}

	tasks, err := daemon.SendPsRequest()
	if err != nil {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Printf("failed to get tasks: %v\n", err)
		}
		return
	}

	if len(tasks) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("no running tasks")
		}
		return
	}

	if jsonOutput {
		data, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
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

// psWatch runs a live-updating dashboard that refreshes every 3 seconds.
// Press q or Ctrl+C to exit.
func psWatch() {
	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		return
	}

	// Read daemon PID
	daemonPID := 0
	if data, err := os.ReadFile(config.PidFile()); err == nil {
		pidStr := strings.TrimSpace(string(data))
		daemonPID, _ = strconv.Atoi(pidStr)
	}

	// Load config for max_workers
	cfg, _ := config.Load()
	maxWorkers := 4
	if cfg != nil && cfg.MaxWorkers > 0 {
		maxWorkers = cfg.MaxWorkers
	}

	// Set up terminal raw mode for key detection
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fall back to non-interactive mode
		psWatchLoop(daemonPID, maxWorkers, nil)
		return
	}
	defer term.Restore(fd, oldState)

	// Channel for quit signal
	quit := make(chan struct{})

	// Read keypresses in background
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				close(quit)
				return
			}
			if buf[0] == 'q' || buf[0] == 'Q' || buf[0] == 3 { // q, Q, or Ctrl+C
				close(quit)
				return
			}
		}
	}()

	psWatchLoop(daemonPID, maxWorkers, quit)
}

func psWatchLoop(daemonPID, maxWorkers int, quit <-chan struct{}) {
	// Handle Ctrl+C via signal if no quit channel
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Render immediately, then on each tick
	for {
		psRenderDashboard(daemonPID, maxWorkers)

		select {
		case <-ticker.C:
			continue
		case <-sigCh:
			fmt.Print("\033[?25h") // show cursor
			return
		case <-quit:
			fmt.Print("\033[?25h") // show cursor
			return
		}
	}
}

func psRenderDashboard(daemonPID, maxWorkers int) {
	// Use a buffer so we can translate \n → \r\n for raw terminal mode.
	var buf bytes.Buffer
	w := &buf

	// Clear screen and move to top
	fmt.Fprint(w, "\033[2J\033[H")
	fmt.Fprint(w, "\033[?25l") // hide cursor

	// Check daemon is still running
	if !daemon.IsDaemonRunning() {
		fmt.Fprintln(w, "daemon not running")
		writeRaw(buf.Bytes())
		return
	}

	// Gather data
	tasks, _ := daemon.SendPsRequest()
	queue, _ := daemon.SendQueueRequest()
	status, _ := daemon.SendStatusRequest()
	watched, _ := loadAllWatched()

	// Count total todos across projects
	totalTasks := 0
	for _, wp := range watched {
		proj, err := project.Load(wp.Path)
		if err != nil {
			continue
		}
		todos, _ := proj.LoadTodos()
		totalTasks += len(todos)
	}

	// Header line
	drainNote := ""
	if status != nil && status.Draining {
		drainNote = " (draining)"
	}
	fmt.Fprintf(w, "anvil daemon (PID %d), %d projects, %d tasks — %d workers%s\n",
		daemonPID, len(watched), totalTasks, maxWorkers, drainNote)
	fmt.Fprintln(w)

	// RUNNING section
	runningTasks := tasks
	fmt.Fprintf(w, "RUNNING (%d)\n", len(runningTasks))
	if len(runningTasks) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, t := range runningTasks {
		projectName := filepath.Base(t.Project)
		statusStr := ""
		if t.Status != "" {
			statusStr = "  " + t.Status
		}
		fmt.Fprintf(w, "  %-40s %8s%s\n",
			truncate(projectName+"/"+t.Name, 40),
			t.Elapsed,
			statusStr)
	}
	fmt.Fprintln(w)

	// IDLE workers
	idleCount := maxWorkers - len(runningTasks)
	if idleCount < 0 {
		idleCount = 0
	}
	fmt.Fprintf(w, "IDLE (%d)\n", idleCount)
	fmt.Fprintln(w)

	// PENDING / SKIPPED from queue
	var pending []daemon.TaskQueueInfo
	var skipped []daemon.TaskQueueInfo
	for _, q := range queue {
		switch q.Status {
		case "pending":
			pending = append(pending, q)
		case "skipped":
			skipped = append(skipped, q)
		}
	}

	if len(pending) > 0 {
		fmt.Fprintf(w, "PENDING (%d)\n", len(pending))
		for _, q := range pending {
			projectName := filepath.Base(q.Project)
			fmt.Fprintf(w, "  %-40s %s\n", truncate(projectName+"/"+q.Name, 40), q.Schedule)
		}
		fmt.Fprintln(w)
	}

	if len(skipped) > 0 {
		fmt.Fprintf(w, "SKIPPED (%d)\n", len(skipped))
		for _, q := range skipped {
			projectName := filepath.Base(q.Project)
			reason := q.SkipReason
			if reason == "" {
				reason = "unknown"
			}
			fmt.Fprintf(w, "  %-40s %s\n", truncate(projectName+"/"+q.Name, 40), reason)
		}
		fmt.Fprintln(w)
	}

	// NEXT SCHEDULED — compute from watched projects
	type nextTask struct {
		project  string
		name     string
		schedule string
		nextRun  time.Time
	}
	var upcoming []nextTask
	now := time.Now()
	for _, wp := range watched {
		proj, err := project.Load(wp.Path)
		if err != nil {
			continue
		}
		todos, _ := proj.LoadTodos()
		for _, t := range todos {
			if t.Schedule == "" || t.Schedule == "once" {
				continue
			}
			p, err := cron.Parse(t.Schedule)
			if err != nil {
				continue
			}
			next, err := p.Next(now)
			if err != nil {
				continue
			}
			upcoming = append(upcoming, nextTask{
				project:  filepath.Base(wp.Path),
				name:     t.Name,
				schedule: t.Schedule,
				nextRun:  next,
			})
		}
	}
	// Sort by next run time
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].nextRun.Before(upcoming[j].nextRun)
	})
	// Show up to 5
	if len(upcoming) > 5 {
		upcoming = upcoming[:5]
	}
	if len(upcoming) > 0 {
		fmt.Fprintf(w, "NEXT SCHEDULED\n")
		for _, u := range upcoming {
			until := time.Until(u.nextRun)
			var untilStr string
			if until < time.Minute {
				untilStr = fmt.Sprintf("in %ds", int(until.Seconds()))
			} else if until < time.Hour {
				untilStr = fmt.Sprintf("in %dm%ds", int(until.Minutes()), int(until.Seconds())%60)
			} else {
				untilStr = fmt.Sprintf("in %dh%dm", int(until.Hours()), int(until.Minutes())%60)
			}
			fmt.Fprintf(w, "  %-6s %-34s %s\n",
				u.schedule,
				truncate(u.project+"/"+u.name, 34),
				untilStr)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Press q to quit")
	writeRaw(buf.Bytes())
}

// writeRaw writes data to stdout, translating bare \n to \r\n for raw terminal mode.
func writeRaw(data []byte) {
	out := bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
	os.Stdout.Write(out)
}


// truncate shortens a string to the specified length, collapsing any
// embedded newlines/whitespace runs into single spaces for table display.
func truncate(s string, maxLen int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// rawLineWriter translates \n to \r\n for raw terminal mode where OPOST is disabled.
type rawLineWriter struct {
	w io.Writer
}

func (r *rawLineWriter) Write(p []byte) (n int, err error) {
	out := bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))
	_, err = r.w.Write(out)
	return len(p), err
}

func addCmd(args []string) {
	// Handle -h/--help before creating task
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintf(os.Stderr, `usage: anvil add [-p priority] [-s schedule | --once] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [-f file | -] <task text>

Add a new task to the project.

Options:
  -p, --priority n        Priority 0-9 (default: 1)
  -s, --schedule cron     Cron schedule (e.g., "*/15 * * * *")
  -t, --template name    Use a template for task configuration
  -o, --once              Create a one-shot task (no schedule)
  -n, --dry-run          Validate schedule without creating task
  --pre-check cmd        Command to run before task execution
  --allowed-tools tools  Comma-separated list of allowed tools (e.g. "Bash,Read" or scoped "Bash(gh:*)", "Read(.claude/commands/*)")
  --max-concurrent n     Max concurrent runs (default: 1)
  --skip-permissions     Skip permission checks
  --strict               Fail if schedule conflicts with existing tasks
  --no-overlap-check     Skip schedule overlap detection
  --depends-on dep       Task dependency (repeatable; use project:task for cross-project)
  -f, --file path        Read task content from a file
  -                      Read task content from stdin

Frontmatter in file/stdin input is merged with CLI flags (CLI flags take precedence).

Examples:
  anvil add "Review pull requests"
  anvil add --once "Migrate the database schema"
  anvil add -p 2 -s "0 9 * * *" "Daily standup notes"
  anvil add --pre-check "git diff --quiet" "Sync documentation"
  anvil add --depends-on setup-db "Run migrations"
  anvil add --depends-on other-project:build-step "Deploy after build"
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
		preview = strings.Join(strings.Fields(preview), " ")
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
	case "extend-timeout":
		taskExtendTimeoutCmd(args[1:])
	case "start":
		taskStartCmd(args[1:])
	case "stop":
		taskStopCmd(args[1:])
	case "next":
		taskNextCmd(args[1:])
	case "export":
		taskExportCmd(args[1:])
	case "import":
		taskImportCmd(args[1:])
	case "wait":
		taskWaitCmd(args[1:])
	case "analyze":
		taskAnalyzeCmd(args[1:])
	case "reset-budget":
		taskResetBudgetCmd(args[1:])
	case "state":
		taskStateCmd(args[1:])
	case "pipeline":
		taskPipelineCmd(args[1:])
	case "sla":
		taskSlaCmd(args[1:])
	case "runbook":
		taskRunbookCmd(args[1:])
	case "find":
		// "find" is an alias for "ls --match" - inject the pattern as --match flag
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: anvil task find <pattern>\n")
			os.Exit(1)
		}
		taskLsCmd([]string{"--match", args[1]})
	case "dry-run":
		taskDryRunCmd(args[1:])
	case "overlaps":
		taskOverlapsCmd(args[1:])
	case "predict":
		taskPredictCmd(args[1:])
	case "tree":
		taskTreeCmd(args[1:])
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
	onceFlag := false
	dryRun := false
	strict := false
	noOverlapCheck := false
	templateName := ""
	var dependsOn []string

	// Track which flags were explicitly set on the CLI so they take precedence over frontmatter/template.
	prioritySet := false
	scheduleSet := false
	preCheckSet := false
	allowedToolsSet := false
	maxConcurrentSet := false

	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t", "--template":
			if i+1 >= len(args) {
				log.Fatal("missing value for -t/--template")
			}
			i++
			templateName = args[i]
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
		case "-o", "--once":
			onceFlag = true
		case "-n", "--dry-run":
			dryRun = true
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
		case "--depends-on":
			if i+1 >= len(args) {
				log.Fatal("missing value for --depends-on")
			}
			i++
			dependsOn = append(dependsOn, args[i])
		case "--strict":
			strict = true
		case "--no-overlap-check":
			noOverlapCheck = true
		case "-":
			readStdin = true
		default:
			rest = append(rest, args[i])
		}
	}

	// Validate --once and --schedule are not both set.
	if onceFlag && scheduleSet {
		log.Fatal("cannot use both --once and --schedule")
	}

	// --once explicitly sets an empty schedule (one-shot task).
	if onceFlag {
		schedule = ""
		scheduleSet = true
	}

	// Handle --dry-run: validate schedule without creating task.
	if dryRun {
		if schedule != "" {
			if _, err := cron.Parse(schedule); err != nil {
				log.Fatalf("invalid cron expression: %s (%v)", schedule, err)
			}
			// Show next run time
			if p, err := cron.Parse(schedule); err == nil {
				if next, err := p.Next(time.Now()); err == nil {
					until := time.Until(next).Round(time.Minute)
					fmt.Printf("Schedule is valid. Next run: %s (%s from now)\n", next.Format("Mon Jan 2 15:04:05"), until)
				}
			}
		} else {
			fmt.Println("No schedule specified (one-shot task)")
		}
		return
	}

	// Load template if specified and apply its values (CLI flags take precedence).
	if templateName != "" {
		abs, err := filepath.Abs(".")
		if err != nil {
			log.Fatalf("bad path: %v", err)
		}
		tmpl, err := project.LoadTemplate(abs, templateName)
		if err != nil {
			log.Fatalf("failed to load template: %v", err)
		}
		// Apply template values only if not explicitly set via CLI flags
		if !prioritySet && tmpl.Spec.Priority > 0 {
			priority = tmpl.Spec.Priority
		}
		if !scheduleSet && tmpl.Spec.Schedule != "" {
			schedule = tmpl.Spec.Schedule
		}
		if !preCheckSet && tmpl.Spec.PreCheck != "" {
			preCheck = tmpl.Spec.PreCheck
		}
		if !allowedToolsSet && len(tmpl.Spec.AllowedTools) > 0 {
			allowedTools = strings.Join(tmpl.Spec.AllowedTools, ",")
		}
		if !maxConcurrentSet && tmpl.Spec.MaxConcurrent > 0 {
			maxConcurrent = tmpl.Spec.MaxConcurrent
		}
		if !skipPermissions && tmpl.Spec.SkipPermissions {
			skipPermissions = true
		}
		// Store template labels for later use (in AddTodo)
		// These will be used when creating the task
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
			log.Fatal("usage: anvil add [-p priority] [-s schedule | --once] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [-f file | -] <task text>")
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
		if _, err := project.Init(abs, tools.FS, false); err != nil {
			log.Fatalf("failed to init project: %v", err)
		}
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	// Check for schedule overlaps with existing tasks.
	if schedule != "" && !noOverlapCheck {
		if newParser, parseErr := cron.Parse(schedule); parseErr == nil {
			todos, _ := proj.LoadTodos()
			now := time.Now().Truncate(time.Minute)
			var overlapping []string
			for _, t := range todos {
				if t.Schedule == "" || t.Schedule == "persistent" || t.Disabled {
					continue
				}
				existParser, err := cron.Parse(t.Schedule)
				if err != nil {
					continue
				}
				// Check next 30 occurrences for overlap within 1 minute
				cur := now
				for k := 0; k < 30; k++ {
					newNext, e1 := newParser.Next(cur)
					existNext, e2 := existParser.Next(cur)
					if e1 != nil || e2 != nil {
						break
					}
					diff := newNext.Sub(existNext)
					if diff < 0 {
						diff = -diff
					}
					if diff <= time.Minute {
						name := strings.TrimSuffix(t.Name, ".md")
						overlapping = append(overlapping, fmt.Sprintf("%s (%s)", name, t.Schedule))
						break
					}
					if newNext.Before(existNext) {
						cur = newNext
					} else {
						cur = existNext
					}
				}
			}
			if len(overlapping) > 0 {
				if strict {
					fmt.Fprintf(os.Stderr, "Error: Schedule overlaps with %d existing task(s):\n", len(overlapping))
					for _, o := range overlapping {
						fmt.Fprintf(os.Stderr, "  - %s\n", o)
					}
					fmt.Fprintf(os.Stderr, "\nRun 'anvil task overlaps' for full conflict report.\n")
					suggestStagger(schedule, len(overlapping))
					os.Exit(1)
				}
				// Non-strict: warn but continue
				severity := "Note"
				if len(overlapping) >= 3 {
					severity = "Warning"
				}
				fmt.Fprintf(os.Stderr, "%s: This schedule overlaps with %d existing task(s) that run at similar times:\n", severity, len(overlapping))
				for _, o := range overlapping {
					fmt.Fprintf(os.Stderr, "  - %s\n", o)
				}
				suggestStagger(schedule, len(overlapping))
				fmt.Fprintln(os.Stderr)
			}
		}
	}

	// Validate dependencies before creating the task.
	if len(dependsOn) > 0 {
		for _, dep := range dependsOn {
			parsed := project.ParseDependency(dep)
			if err := project.ValidateDependency(parsed, abs); err != nil {
				log.Fatalf("invalid dependency %q: %v", dep, err)
			}
		}
	}

	relPath, err := proj.AddTodo(priority, schedule, taskText, preCheck, allowedTools, maxConcurrent, skipPermissions, "", dependsOn)
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
	jsonOutput := false
	matchPattern := ""
	labelFilter := ""

	// Parse flags: --match/-m for pattern, --all/-a for all projects, --json for JSON output, --label/-l for label filter
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--all" || a == "-a" {
			allProjects = true
		} else if a == "--json" {
			jsonOutput = true
		} else if a == "--match" || a == "-m" {
			if i+1 >= len(args) {
				log.Fatal("missing value for --match/-m")
			}
			i++
			matchPattern = args[i]
		} else if strings.HasPrefix(a, "--match=") || strings.HasPrefix(a, "-m=") {
			matchPattern = strings.TrimPrefix(strings.TrimPrefix(a, "--match="), "-m=")
		} else if a == "--label" || a == "-l" {
			if i+1 >= len(args) {
				log.Fatal("missing value for --label/-l")
			}
			i++
			labelFilter = strings.ToLower(args[i])
		} else if strings.HasPrefix(a, "--label=") || strings.HasPrefix(a, "-l=") {
			labelFilter = strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(a, "--label="), "-l="))
		} else {
			filteredArgs = append(filteredArgs, a)
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

	// Filter by match pattern if specified (regex or case-insensitive substring match)
	if matchPattern != "" {
		// Check if it's a valid regex
		var regex *regexp.Regexp
		isRegex := false
		if re, err := regexp.Compile("(?i)" + matchPattern); err == nil {
			regex = re
			isRegex = true
		}

		var filtered []projectTodos
		for _, p := range projects {
			var projectFiltered []project.Todo
			for _, t := range p.todos {
				matched := false
				if isRegex {
					matched = regex.MatchString(t.Name)
				} else {
					matched = strings.Contains(strings.ToLower(t.Name), strings.ToLower(matchPattern))
				}
				if matched {
					projectFiltered = append(projectFiltered, t)
				}
			}
			if len(projectFiltered) > 0 {
				filtered = append(filtered, projectTodos{path: p.path, todos: projectFiltered})
			}
		}
		projects = filtered
	}

	// Filter by label if specified (case-insensitive match)
	if labelFilter != "" {
		var filtered []projectTodos
		for _, p := range projects {
			var projectFiltered []project.Todo
			for _, t := range p.todos {
				for _, lbl := range t.Labels {
					if strings.ToLower(lbl) == labelFilter {
						projectFiltered = append(projectFiltered, t)
						break
					}
				}
			}
			if len(projectFiltered) > 0 {
				filtered = append(filtered, projectTodos{path: p.path, todos: projectFiltered})
			}
		}
		projects = filtered
	}

	total := 0
	for _, p := range projects {
		total += len(p.todos)
	}
	if total == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("no tasks")
		}
		if matchPattern != "" || labelFilter != "" {
			os.Exit(1)
		}
		return
	}

	if jsonOutput {
		type taskJSON struct {
			Project  string   `json:"project"`
			Name     string   `json:"name"`
			Priority int      `json:"priority"`
			Schedule string   `json:"schedule"`
			Status   string   `json:"status"`
			Disabled bool     `json:"disabled"`
			Content  string   `json:"content"`
			ID       string   `json:"id,omitempty"`
			Labels   []string `json:"labels,omitempty"`
		}
		var items []taskJSON
		for _, p := range projects {
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
				items = append(items, taskJSON{
					Project:  p.path,
					Name:     t.Name,
					Priority: t.Priority,
					Schedule: t.Schedule,
					Status:   status,
					Disabled: t.Disabled,
					Content:  strings.TrimSpace(t.Content),
					ID:       t.ID,
					Labels:   t.Labels,
				})
			}
		}
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Fetch budget data for persistent tasks
	var budgetMap map[string]float64
	if daemon.IsDaemonRunning() {
		budgetMap, _ = daemon.SendBudgetRequest()
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
			preview = strings.Join(strings.Fields(preview), " ")
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			labelStr := ""
			if len(t.Labels) > 0 {
				labelStr = "  [" + strings.Join(t.Labels, ", ") + "]"
			}
			budgetStr := ""
			if t.IsPersistent() && t.PersistentBudget > 0 {
				budgetUsed := time.Duration(0)
				if secs, ok := budgetMap[taskKey]; ok {
					budgetUsed = time.Duration(secs * float64(time.Second))
				}
				pct := float64(budgetUsed) / float64(t.PersistentBudget) * 100
				filled := int(pct / 10)
				if filled > 10 {
					filled = 10
				}
				bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", 10-filled)
				budgetStr = fmt.Sprintf("  %s %.0f%%", bar, pct)
			}
			fmt.Printf("p%d  %-14s  %-10s  %-35s  %s%s%s\n", t.Priority, t.Schedule, status, t.Name, preview, labelStr, budgetStr)
		}
	}
}

func taskGetCmd(args []string) {
	jsonOutput := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		} else {
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task get <name> [--json]\n")
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

	todo := findTodo(todos, rest[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", rest[0])
		os.Exit(1)
	}

	// Check if task is running
	runStatus := "idle"
	var runPID int
	var runElapsed string
	if daemon.IsDaemonRunning() {
		runningTasks, err := daemon.SendPsRequest()
		if err == nil {
			taskName := fmt.Sprintf("%s/%s", abs, todo.Name)
			for _, t := range runningTasks {
				if t.Name == taskName {
					runStatus = "running"
					runPID = t.PID
					runElapsed = t.Elapsed
					break
				}
			}
		}
	}

	if jsonOutput {
		type depStatusJSON struct {
			Name    string `json:"name"`
			Status  string `json:"status"` // "success", "failed", "not_run", "stale"
			Error   string `json:"error,omitempty"`
			LastRun string `json:"last_run,omitempty"`
		}
		type taskDetailJSON struct {
			File            string          `json:"file"`
			ID              string          `json:"id"`
			Name            string          `json:"name"`
			Schedule        string          `json:"schedule"`
			Priority        int             `json:"priority"`
			Disabled        bool            `json:"disabled"`
			Status          string          `json:"status"`
			PID             int             `json:"pid,omitempty"`
			Elapsed         string          `json:"elapsed,omitempty"`
			Content         string          `json:"content"`
			PreCheck        string          `json:"pre_check,omitempty"`
			OnSuccess       string          `json:"on_success,omitempty"`
			OnFailure       string          `json:"on_failure,omitempty"`
			AllowedTools    []string        `json:"allowed_tools,omitempty"`
			MaxConcurrent   int             `json:"max_concurrent,omitempty"`
			SkipPermissions bool            `json:"skip_permissions,omitempty"`
			Runner          string          `json:"runner,omitempty"`
			RunnerChain     []string        `json:"runner_chain,omitempty"`
			RunnerOnTimeout string          `json:"runner_on_timeout,omitempty"`
			LastRunnerUsed  string          `json:"last_runner_used,omitempty"`
			DependsOn       []string        `json:"depends_on,omitempty"`
			Dependencies    []depStatusJSON `json:"dependencies,omitempty"`
			Retry           int             `json:"retry,omitempty"`
			RetryDelay      string          `json:"retry_delay,omitempty"`
			LastAttempt     int             `json:"last_attempt,omitempty"`
			LastMaxRetries  int             `json:"last_max_retries,omitempty"`
			LastAttemptStatus string        `json:"last_attempt_status,omitempty"`
			BudgetTotal     string          `json:"budget_total,omitempty"`
			BudgetUsed      string          `json:"budget_used,omitempty"`
			BudgetRemaining string          `json:"budget_remaining,omitempty"`
			BudgetPercent   float64         `json:"budget_percent,omitempty"`
			BudgetExhausted bool            `json:"budget_exhausted,omitempty"`
			SLAMaxDelay     string          `json:"sla_max_delay,omitempty"`
			SLAStrict       bool            `json:"sla_strict,omitempty"`
			SLALastDelay    string          `json:"sla_last_delay,omitempty"`
			SLALastViolation bool           `json:"sla_last_violation,omitempty"`
		}
		detail := taskDetailJSON{
			File:            todo.Path,
			ID:              todo.ID,
			Name:            todo.Name,
			Schedule:        todo.Schedule,
			Priority:        todo.Priority,
			Disabled:        todo.Disabled,
			Status:          runStatus,
			PID:             runPID,
			Elapsed:         runElapsed,
			Content:         strings.TrimSpace(todo.Content),
			PreCheck:        todo.PreCheck,
			OnSuccess:       todo.OnSuccess,
			OnFailure:       todo.OnFailure,
			AllowedTools:    todo.AllowedTools,
			MaxConcurrent:   todo.MaxConcurrent,
			SkipPermissions: todo.SkipPermissions,
			Runner:          todo.Runner,
			RunnerChain:     todo.RunnerChain,
			RunnerOnTimeout: todo.RunnerOnTimeout,
			DependsOn:       todo.DependsOn,
		}
		// Add last runner used from most recent run record
		if lastRec, recErr := project.ReadCurrentRunRecord(abs, todo.ID); recErr == nil && lastRec.RunnerCommand != "" {
			detail.LastRunnerUsed = lastRec.RunnerCommand
		}
		// Add dependency status info
		if len(todo.DependsOn) > 0 {
			for _, dep := range todo.DependsOn {
				ds := depStatusJSON{Name: dep}
				rec, err := project.ReadCurrentRunRecord(abs, dep)
				if err != nil {
					ds.Status = "not_run"
				} else if !rec.Success {
					ds.Status = "failed"
					ds.Error = rec.Error
					if !rec.Finished.IsZero() {
						ds.LastRun = rec.Finished.Format(time.RFC3339)
					}
				} else {
					ds.Status = "success"
					if !rec.Finished.IsZero() {
						ds.LastRun = rec.Finished.Format(time.RFC3339)
					}
				}
				detail.Dependencies = append(detail.Dependencies, ds)
			}
		}
		// Add retry configuration and last run attempt info
		if todo.Retry > 0 {
			detail.Retry = todo.Retry
			delayStr := todo.RetryDelay.String()
			if todo.RetryDelay <= 0 {
				delayStr = "1m0s"
			}
			detail.RetryDelay = delayStr
			rec, recErr := project.ReadCurrentRunRecord(abs, todo.ID)
			if recErr == nil && rec.MaxRetries > 0 {
				detail.LastAttempt = rec.Attempt
				detail.LastMaxRetries = rec.MaxRetries
				if !rec.Success {
					if rec.Attempt >= rec.MaxRetries {
						detail.LastAttemptStatus = "failed (retries exhausted)"
					} else {
						detail.LastAttemptStatus = "failed"
					}
				} else if rec.Attempt > 1 {
					detail.LastAttemptStatus = "succeeded (after retry)"
				} else {
					detail.LastAttemptStatus = "succeeded"
				}
			}
		}
		// Add budget info for persistent tasks
		if todo.IsPersistent() && todo.PersistentBudget > 0 {
			budgetUsed := time.Duration(0)
			if daemon.IsDaemonRunning() {
				budgetMap, err := daemon.SendBudgetRequest()
				if err == nil {
					taskKey := fmt.Sprintf("%s/%s", abs, todo.Name)
					if secs, ok := budgetMap[taskKey]; ok {
						budgetUsed = time.Duration(secs * float64(time.Second))
					}
				}
			}
			remaining := todo.PersistentBudget - budgetUsed
			if remaining < 0 {
				remaining = 0
			}
			pct := float64(budgetUsed) / float64(todo.PersistentBudget) * 100
			detail.BudgetTotal = todo.PersistentBudget.String()
			detail.BudgetUsed = budgetUsed.Round(time.Second).String()
			detail.BudgetRemaining = remaining.Round(time.Second).String()
			detail.BudgetPercent = pct
			detail.BudgetExhausted = budgetUsed >= todo.PersistentBudget
		}
		// Add SLA info
		if todo.SLA.MaxDelay > 0 {
			detail.SLAMaxDelay = todo.SLA.MaxDelay.String()
			detail.SLAStrict = todo.SLA.Strict
			rec, recErr := project.ReadCurrentRunRecord(abs, todo.ID)
			if recErr == nil && rec.SLAMaxDelay > 0 {
				detail.SLALastDelay = rec.DispatchDelay.String()
				detail.SLALastViolation = rec.SLAViolation
			}
		}
		data, err := json.MarshalIndent(detail, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	fmt.Printf("File:     %s\n", todo.Path)
	fmt.Printf("ID:       %s\n", todo.ID)
	fmt.Printf("Schedule: %s\n", todo.Schedule)
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
	if runPID > 0 {
		fmt.Printf("Status:   running (PID %d, elapsed %s)\n", runPID, runElapsed)
	} else {
		fmt.Printf("Status:   %s\n", runStatus)
	}
	// Show dependency status
	if len(todo.DependsOn) > 0 {
		fmt.Printf("Deps:     %s\n", strings.Join(todo.DependsOn, ", "))
		for _, dep := range todo.DependsOn {
			rec, err := project.ReadCurrentRunRecord(abs, dep)
			if err != nil {
				fmt.Printf("  %-30s not_run\n", dep)
			} else if !rec.Success {
				lastRun := ""
				if !rec.Finished.IsZero() {
					lastRun = fmt.Sprintf(" (last run: %s)", rec.Finished.Format("15:04"))
				}
				errMsg := ""
				if rec.Error != "" {
					errMsg = fmt.Sprintf(" — %s", rec.Error)
				}
				fmt.Printf("  %-30s failed%s%s\n", dep, lastRun, errMsg)
			} else {
				lastRun := ""
				if !rec.Finished.IsZero() {
					lastRun = fmt.Sprintf(" (last run: %s)", rec.Finished.Format("15:04"))
				}
				fmt.Printf("  %-30s success%s\n", dep, lastRun)
			}
		}
	}
	// Show runner chain configuration
	if len(todo.RunnerChain) > 0 {
		fmt.Printf("Chain:    %d runners\n", len(todo.RunnerChain))
		for i, cmd := range todo.RunnerChain {
			fmt.Printf("  [%d] %s\n", i, cmd)
		}
		if todo.RunnerOnTimeout != "" {
			fmt.Printf("  on_timeout: %s\n", todo.RunnerOnTimeout)
		}
		// Show which runner was last used
		rec, recErr := project.ReadCurrentRunRecord(abs, todo.ID)
		if recErr == nil && rec.RunnerCommand != "" {
			fmt.Printf("          last used: %s\n", rec.RunnerCommand)
		}
	} else if todo.RunnerOnTimeout != "" {
		fmt.Printf("Timeout:  fallback runner: %s\n", todo.RunnerOnTimeout)
	}
	// Show retry configuration and last run attempt info
	if todo.Retry > 0 {
		delayStr := todo.RetryDelay.String()
		if todo.RetryDelay <= 0 {
			delayStr = "1m0s (default)"
		}
		fmt.Printf("Retry:    %d retries, delay %s\n", todo.Retry, delayStr)
		// Show last run attempt info
		rec, err := project.ReadCurrentRunRecord(abs, todo.ID)
		if err == nil && rec.MaxRetries > 0 {
			attemptStatus := "succeeded"
			if !rec.Success {
				if rec.Attempt >= rec.MaxRetries {
					attemptStatus = "failed (retries exhausted)"
				} else {
					attemptStatus = "failed"
				}
			} else if rec.Attempt > 1 {
				attemptStatus = "succeeded (after retry)"
			}
			fmt.Printf("          last run: attempt %d/%d — %s\n", rec.Attempt, rec.MaxRetries, attemptStatus)
		}
	}
	// Show budget info for persistent tasks with a budget
	if todo.IsPersistent() && todo.PersistentBudget > 0 {
		budgetUsed := time.Duration(0)
		if daemon.IsDaemonRunning() {
			budgetMap, err := daemon.SendBudgetRequest()
			if err == nil {
				taskKey := fmt.Sprintf("%s/%s", abs, todo.Name)
				if secs, ok := budgetMap[taskKey]; ok {
					budgetUsed = time.Duration(secs * float64(time.Second))
				}
			}
		}
		remaining := todo.PersistentBudget - budgetUsed
		if remaining < 0 {
			remaining = 0
		}
		pct := float64(budgetUsed) / float64(todo.PersistentBudget) * 100
		fmt.Printf("Budget:   %v / %v used (%.0f%%)\n", budgetUsed.Round(time.Second), todo.PersistentBudget, pct)
		if remaining > 0 {
			fmt.Printf("          %v remaining\n", remaining.Round(time.Second))
		} else {
			fmt.Printf("          EXHAUSTED\n")
		}
	}
	// Show SLA configuration and last run status
	if todo.SLA.MaxDelay > 0 {
		strictStr := ""
		if todo.SLA.Strict {
			strictStr = " (strict)"
		}
		fmt.Printf("SLA:      %v max delay%s\n", todo.SLA.MaxDelay, strictStr)
		rec, err := project.ReadCurrentRunRecord(abs, todo.ID)
		if err == nil && rec.SLAMaxDelay > 0 {
			if rec.SLAViolation {
				fmt.Printf("          last run: %v late — SLA VIOLATION\n", rec.DispatchDelay.Round(time.Second))
			} else {
				fmt.Printf("          last run: on time (%v delay)\n", rec.DispatchDelay.Round(time.Second))
			}
		}
	}
	fmt.Printf("\n%s", todo.Content)
}

func taskResetBudgetCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task reset-budget <name>\n")
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Fprintf(os.Stderr, "daemon is not running\n")
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

	if !todo.IsPersistent() {
		fmt.Fprintf(os.Stderr, "task %s is not a persistent task\n", todo.Name)
		os.Exit(1)
	}

	taskKey := fmt.Sprintf("%s/%s", abs, todo.Name)
	if err := daemon.SendResetBudgetRequest(taskKey); err != nil {
		log.Fatalf("failed to reset budget: %v", err)
	}

	fmt.Printf("Budget reset for %s\n", todo.Name)
}

func taskRunbookCmd(args []string) {
	openInBrowser := false
	var rest []string
	for _, a := range args {
		if a == "--open" || a == "-o" {
			openInBrowser = true
		} else {
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task runbook <name> [--open|-o]\n")
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

	todo := findTodo(todos, rest[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", rest[0])
		os.Exit(1)
	}

	runbook := todo.Runbook
	if runbook == "" {
		fmt.Printf("No runbook defined for task %s\n", todo.Name)
		fmt.Printf("Add a runbook in the task's frontmatter:\n")
		fmt.Printf("  ---\n")
		fmt.Printf("  runbook: https://example.com/runbook\n")
		fmt.Printf("  ---\n")
		os.Exit(0)
	}

	// Check if it's a URL
	if strings.HasPrefix(runbook, "http://") || strings.HasPrefix(runbook, "https://") {
		fmt.Printf("Runbook: %s\n", runbook)
		if openInBrowser {
			fmt.Printf("Opening in browser...\n")
			exec.Command("open", runbook).Start()
		}
	} else {
		// Inline markdown - display it
		fmt.Printf("Runbook for %s:\n", todo.Name)
		fmt.Println(strings.Repeat("-", 50))
		fmt.Println(runbook)
	}
}

func taskSlaCmd(args []string) {
	verbose := false
	reset := false
	jsonOutput := false
	for _, a := range args {
		switch a {
		case "--verbose", "-v":
			verbose = true
		case "--reset":
			reset = true
		case "--json":
			jsonOutput = true
		}
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

	// Filter tasks with SLA configured
	type slaEntry struct {
		Name         string `json:"name"`
		MaxDelay     string `json:"max_delay"`
		Strict       bool   `json:"strict,omitempty"`
		LastDelay    string `json:"last_delay,omitempty"`
		Violation    bool   `json:"violation"`
		ScheduledAt  string `json:"scheduled_at,omitempty"`
		DispatchedAt string `json:"dispatched_at,omitempty"`
	}

	var entries []slaEntry
	for _, t := range todos {
		if t.SLA.MaxDelay <= 0 {
			continue
		}

		entry := slaEntry{
			Name:     t.Name,
			MaxDelay: t.SLA.MaxDelay.String(),
			Strict:   t.SLA.Strict,
		}

		rec, recErr := project.ReadCurrentRunRecord(abs, t.ID)
		if recErr == nil && rec.SLAMaxDelay > 0 {
			entry.LastDelay = rec.DispatchDelay.String()
			entry.Violation = rec.SLAViolation
			if !rec.ScheduledTime.IsZero() {
				entry.ScheduledAt = rec.ScheduledTime.Format(time.RFC3339)
			}
			if !rec.Started.IsZero() {
				entry.DispatchedAt = rec.Started.Format(time.RFC3339)
			}
		}

		if verbose || entry.Violation {
			entries = append(entries, entry)
		}
	}

	if reset {
		resetCount := 0
		for _, t := range todos {
			if t.SLA.MaxDelay <= 0 {
				continue
			}
			records, err := project.ReadAllRunRecords(abs, t.ID)
			if err != nil {
				continue
			}
			for _, rec := range records {
				if rec.SLAViolation {
					rec.SLAViolation = false
					if writeErr := project.WriteRunRecord(abs, rec); writeErr == nil {
						resetCount++
					}
				}
			}
		}
		fmt.Printf("Reset %d SLA violation(s)\n", resetCount)
		return
	}

	if len(entries) == 0 {
		if verbose {
			fmt.Println("No tasks have SLA tracking enabled.")
		} else {
			fmt.Println("No SLA violations found.")
		}
		return
	}

	if jsonOutput {
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	for _, e := range entries {
		status := "OK"
		if e.Violation {
			status = "VIOLATION"
		}
		fmt.Printf("%-30s  SLA: %s max delay  %s", e.Name, e.MaxDelay, status)
		if e.LastDelay != "" {
			fmt.Printf("  (last: %s delay)", e.LastDelay)
		}
		fmt.Println()
	}
}

func taskStateCmd(args []string) {
	// Handle subcommands: get, export, import, clear
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task state <name> [subcommand]\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  (none)       Show current state\n")
		fmt.Fprintf(os.Stderr, "  --export FILE  Export state to file\n")
		fmt.Fprintf(os.Stderr, "  --import FILE  Import state from file\n")
		fmt.Fprintf(os.Stderr, "  --clear        Clear state\n")
		os.Exit(1)
	}

	// Check for flags first
	var exportFile, importFile string
	var clearState bool
	var taskName string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--export":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --export requires a file argument\n")
				os.Exit(1)
			}
			exportFile = args[i+1]
			i++
		case "--import":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --import requires a file argument\n")
				os.Exit(1)
			}
			importFile = args[i+1]
			i++
		case "--clear":
			clearState = true
		default:
			if taskName == "" {
				taskName = args[i]
			}
		}
	}

	if taskName == "" {
		fmt.Fprintf(os.Stderr, "error: task name required\n")
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

	// Check if task has state configured
	if todo.State == nil {
		fmt.Fprintf(os.Stderr, "task %s does not have state configured\n", taskName)
		fmt.Fprintf(os.Stderr, "Add 'state' configuration to task frontmatter:\n")
		fmt.Fprintf(os.Stderr, "  state:\n")
		fmt.Fprintf(os.Stderr, "    bucket: \"my-bucket\"\n")
		fmt.Fprintf(os.Stderr, "    key: \"{{ .TaskID }}\"\n")
		os.Exit(1)
	}

	// Resolve the key template
	stateKey := strings.ReplaceAll(todo.State.Key, "{{ .TaskID }}", todo.ID)

	if importFile != "" {
		// Import state from file
		data, err := os.ReadFile(importFile)
		if err != nil {
			log.Fatalf("failed to read import file: %v", err)
		}
		var state map[string]interface{}
		if err := json.Unmarshal(data, &state); err != nil {
			log.Fatalf("failed to parse import file: %v", err)
		}
		if err := project.WriteTaskState(abs, todo.State.Bucket, stateKey, state); err != nil {
			log.Fatalf("failed to write state: %v", err)
		}
		fmt.Printf("State imported from %s\n", importFile)
		return
	}

	if clearState {
		if err := project.DeleteTaskState(abs, todo.State.Bucket, stateKey); err != nil {
			log.Fatalf("failed to clear state: %v", err)
		}
		fmt.Printf("State cleared for %s\n", taskName)
		return
	}

	if exportFile != "" {
		// Export state to file
		state, err := project.ReadTaskState(abs, todo.State.Bucket, stateKey)
		if err != nil {
			log.Fatalf("failed to read state: %v", err)
		}
		if state == nil {
			state = map[string]interface{}{}
		}
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal state: %v", err)
		}
		if err := os.WriteFile(exportFile, data, 0644); err != nil {
			log.Fatalf("failed to write export file: %v", err)
		}
		fmt.Printf("State exported to %s\n", exportFile)
		return
	}

	// Default: show current state
	state, err := project.ReadTaskState(abs, todo.State.Bucket, stateKey)
	if err != nil {
		log.Fatalf("failed to read state: %v", err)
	}
	if state == nil {
		fmt.Printf("No state found for %s (bucket: %s, key: %s)\n", taskName, todo.State.Bucket, stateKey)
		return
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal state: %v", err)
	}
	fmt.Print(string(data))
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
		fmt.Fprintf(os.Stderr, "usage: anvil task run <name> [--force]\n")
		os.Exit(1)
	}

	// Parse --force flag
	force := false
	var filtered []string
	for _, a := range args {
		if a == "--force" {
			force = true
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task run <name> [--force]\n")
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

	todo := findTodo(todos, filtered[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", filtered[0])
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Fprintln(os.Stderr, "daemon not running — start it with: anvil watch")
		os.Exit(1)
	}

	if err := daemon.SendRunRequest(abs, todo.ID, todo.Name, force); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run task: %v\n", err)
		os.Exit(1)
	}

	msg := "▶ Dispatched %s for immediate execution\n"
	if force {
		msg = "▶ Dispatched %s for immediate execution (bypassing time windows)\n"
	}
	fmt.Printf(msg, todo.Name)
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

func taskStopCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task stop <name>\n")
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

	if err := daemon.SendStopRequest(todo.ID); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("stopped %s — will not be re-dispatched until started\n", todo.Name)
}

func taskStartCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task start <name>\n")
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

	if err := daemon.SendStartRequest(abs, todo.ID, todo.Name); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("started %s — will be dispatched on next tick\n", todo.Name)
}

func taskHistoryCmd(args []string) {
	limit := 10
	showFailuresOnly := false
	showRetriedOnly := false
	showStats := false
	jsonOutput := false
	followMode := false
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-n", "--limit":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "usage: anvil task history <name> [-n limit] [-f] [--failures] [--retried] [--stats] [--json]\n")
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
		case "--retried":
			showRetriedOnly = true
			i++
		case "--stats":
			showStats = true
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
		fmt.Fprintf(os.Stderr, "usage: anvil task history <name> [-n limit] [-f] [--failures] [--retried] [--stats] [--json]\n")
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

	// Filter to only retried runs (attempt > 1)
	if showRetriedOnly {
		var filtered []project.RunRecord
		for _, rec := range records {
			if rec.Attempt > 1 {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	// Show retry statistics
	if showStats {
		total := len(records)
		succeeded := 0
		failed := 0
		retried := 0
		for _, rec := range records {
			if rec.Success {
				succeeded++
			} else {
				failed++
			}
			if rec.Attempt > 1 {
				retried++
			}
		}
		retriedPct := 0.0
		if total > 0 {
			retriedPct = float64(retried) / float64(total) * 100
		}
		fmt.Printf("Total: %d, Succeeded: %d, Failed: %d, Retried: %d (%.0f%%)\n", total, succeeded, failed, retried, retriedPct)
		return
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
	fmt.Printf("%-20s %10s %10s %-12s %10s\n", "STARTED", "DURATION", "ATTEMPTS", "RUNNER", "STATUS")
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
				// Truncate error for display, collapse newlines
				errMsg := strings.Join(strings.Fields(rec.Error), " ")
				if len(errMsg) > 20 {
					errMsg = errMsg[:20] + "..."
				}
				status = errMsg
			}
			// Annotate if all retries were exhausted
			if rec.MaxRetries > 0 && rec.Attempt >= rec.MaxRetries {
				status += " (retries exhausted)"
			}
		} else if rec.Attempt > 1 {
			// Succeeded after retries
			status = "ok (retry succeeded)"
		}

		// Format attempts column (e.g., "1/3" or "-" if no retries configured)
		attempts := "-"
		if rec.MaxRetries > 0 {
			attempts = fmt.Sprintf("%d/%d", rec.Attempt, rec.MaxRetries)
		} else if rec.Attempt > 0 {
			attempts = fmt.Sprintf("%d", rec.Attempt)
		}

		// Format runner column
		runnerLabel := "-"
		if rec.RunnerCommand != "" {
			runnerLabel = rec.RunnerCommand
			if len(runnerLabel) > 12 {
				runnerLabel = runnerLabel[:12]
			}
		} else if rec.RunnerIndex >= 100 {
			runnerLabel = "timeout-fb"
		} else if rec.RunnerIndex > 0 {
			runnerLabel = fmt.Sprintf("runner[%d]", rec.RunnerIndex)
		}

		fmt.Printf("%-20s %10s %10s %-12s %10s\n", rec.Started.Format("2006-01-02 15:04"), duration, attempts, runnerLabel, status)

		// Print output summary if available
		if rec.OutputSummary != "" {
			summaryLines := strings.Split(rec.OutputSummary, "\n")
			for _, line := range summaryLines {
				fmt.Printf("  %s\n", line)
			}
		}

		// Print checkpoint data if available
		if rec.CheckpointData != "" {
			cpPreview := rec.CheckpointData
			if len(cpPreview) > 80 {
				cpPreview = cpPreview[:80] + "..."
			}
			fmt.Printf("  checkpoint: %s\n", cpPreview)
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

func taskQueueCmd(args []string) {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		}
	}

	if !daemon.IsDaemonRunning() {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("daemon not running")
		}
		return
	}

	tasks, err := daemon.SendQueueRequest()
	if err != nil {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Fprintf(os.Stderr, "failed to get queue status: %v\n", err)
		}
		os.Exit(1)
	}

	if len(tasks) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("no tasks in queue")
		}
		return
	}

	if jsonOutput {
		data, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
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
	var newContent *string
	var contentFile *string
	var removeField *string
	var bulkPattern string
	var dryRun bool
	var setDisabled *bool
	var addLabel string
	var removeLabel string

	// Fields that can be cleared with --remove
	removableFields := map[string]bool{
		"schedule": true, "allowed_tools": true, "pre_check": true,
		"on_success": true, "on_failure": true, "timeout": true,
		"persistent_cooldown": true, "persistent_max_runtime": true,
		"persistent_max_failures": true, "persistent_budget": true,
	}

	var nameArgs []string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--all":
			if i+1 >= len(args) {
				// --all without pattern matches all tasks
				bulkPattern = "*"
			} else if strings.HasPrefix(args[i+1], "-") {
				// Next arg is a flag, match all tasks
				bulkPattern = "*"
			} else {
				i++
				bulkPattern = args[i]
			}
		case "--dry-run":
			dryRun = true
		case "--disabled":
			v := true
			setDisabled = &v
		case "--enabled":
			v := false
			setDisabled = &v
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
		case "--content":
			if i+1 >= len(args) {
				log.Fatal("missing value for --content")
			}
			i++
			newContent = &args[i]
		case "--content-file":
			if i+1 >= len(args) {
				log.Fatal("missing value for --content-file")
			}
			i++
			contentFile = &args[i]
		case "--remove", "--clear":
			if i+1 >= len(args) {
				log.Fatal("missing field name for --remove")
			}
			i++
			removeField = &args[i]
		case "--add-label":
			if i+1 >= len(args) {
				log.Fatal("missing value for --add-label")
			}
			i++
			addLabel = args[i]
		case "--remove-label":
			if i+1 >= len(args) {
				log.Fatal("missing value for --remove-label")
			}
			i++
			removeLabel = args[i]
		default:
			nameArgs = append(nameArgs, args[i])
		}
		i++
	}

	if newContent != nil && contentFile != nil {
		log.Fatal("cannot use both --content and --content-file")
	}

	if removeField != nil && (newSchedule != nil || newPriority != nil || newContent != nil || contentFile != nil) {
		log.Fatal("cannot combine --remove with other edit flags")
	}

	if removeField != nil && !removableFields[*removeField] {
		var fields []string
		for k := range removableFields {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		log.Fatalf("invalid field %q for --remove. Valid fields: %s", *removeField, strings.Join(fields, ", "))
	}

	// Read content from file if --content-file was provided
	if contentFile != nil {
		data, err := os.ReadFile(*contentFile)
		if err != nil {
			log.Fatalf("failed to read content file %q: %v", *contentFile, err)
		}
		s := string(data)
		newContent = &s
	}

	// Bulk edit mode: --all [pattern]
	if bulkPattern != "" {
		if newSchedule == nil && newPriority == nil && setDisabled == nil {
			log.Fatal("--all requires at least one edit flag: -s/--schedule, -p/--priority, --disabled, or --enabled")
		}
		if newContent != nil || contentFile != nil || removeField != nil {
			log.Fatal("--all does not support --content, --content-file, or --remove")
		}
		if newSchedule != nil && *newSchedule != "" && *newSchedule != "persistent" {
			if _, err := cron.Parse(*newSchedule); err != nil {
				log.Fatalf("invalid schedule %q: %v", *newSchedule, err)
			}
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

		// Ensure pattern has .md suffix for matching
		matchPattern := bulkPattern
		if !strings.HasSuffix(matchPattern, ".md") && !strings.HasSuffix(matchPattern, "*") {
			matchPattern += ".md"
		}

		var matched []project.Todo
		for _, t := range todos {
			ok, err := filepath.Match(matchPattern, t.Name)
			if err != nil {
				log.Fatalf("invalid pattern %q: %v", bulkPattern, err)
			}
			if ok {
				matched = append(matched, t)
			}
		}

		if len(matched) == 0 {
			fmt.Fprintf(os.Stderr, "no tasks match pattern: %s\n", bulkPattern)
			os.Exit(1)
		}

		if dryRun {
			fmt.Printf("dry run: would update %d task(s):\n", len(matched))
			for _, t := range matched {
				changes := []string{}
				if newSchedule != nil {
					changes = append(changes, fmt.Sprintf("schedule: %s -> %s", t.Schedule, *newSchedule))
				}
				if newPriority != nil {
					changes = append(changes, fmt.Sprintf("priority: p%d -> p%d", t.Priority, *newPriority))
				}
				if setDisabled != nil {
					changes = append(changes, fmt.Sprintf("disabled: %t -> %t", t.Disabled, *setDisabled))
				}
				fmt.Printf("  %s (%s)\n", t.Name, strings.Join(changes, ", "))
			}
			return
		}

		updated := 0
		for _, t := range matched {
			if err := taskEditApply(&t, abs, newSchedule, newPriority, setDisabled); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update %s: %v\n", t.Name, err)
				continue
			}
			updated++
		}
		fmt.Printf("updated %d task(s)\n", updated)
		return
	}

	if len(nameArgs) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task edit <name> [-s schedule] [-p priority] [--content text] [--content-file path] [--remove field] [--add-label L] [--remove-label L]\n")
		fmt.Fprintf(os.Stderr, "       anvil task edit --all [pattern] [-s schedule] [-p priority] [--disabled|--enabled] [--dry-run]\n")
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

	// Single-task --disabled/--enabled support
	if setDisabled != nil && len(nameArgs) > 0 {
		todo := findTodo(todos, nameArgs[0])
		if todo == nil {
			fmt.Fprintf(os.Stderr, "task not found: %s\n", nameArgs[0])
			os.Exit(1)
		}
		if err := taskEditApply(todo, abs, newSchedule, newPriority, setDisabled); err != nil {
			log.Fatalf("failed to update task: %v", err)
		}
		return
	}

	todo := findTodo(todos, nameArgs[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", nameArgs[0])
		os.Exit(1)
	}

	// Handle --add-label / --remove-label
	if addLabel != "" || removeLabel != "" {
		raw, err := os.ReadFile(todo.Path)
		if err != nil {
			log.Fatalf("failed to read task file: %v", err)
		}
		contentStr := string(raw)
		var fmMap map[string]interface{}
		body := contentStr

		if strings.HasPrefix(contentStr, "---\n") {
			parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
			if len(parts) == 2 {
				body = parts[1]
				if err := yaml.Unmarshal([]byte(parts[0]), &fmMap); err != nil {
					log.Fatalf("failed to parse front-matter: %v", err)
				}
			}
		}
		if fmMap == nil {
			fmMap = make(map[string]interface{})
		}

		// Get current labels
		var labels []string
		if raw, ok := fmMap["labels"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						labels = append(labels, s)
					}
				}
			}
		}

		if addLabel != "" {
			// Check for duplicate (case-insensitive)
			found := false
			for _, l := range labels {
				if strings.EqualFold(l, addLabel) {
					found = true
					break
				}
			}
			if !found {
				labels = append(labels, addLabel)
				fmt.Printf("added label %q to %s\n", addLabel, nameArgs[0])
			} else {
				fmt.Printf("task %s already has label %q\n", nameArgs[0], addLabel)
				return
			}
		}

		if removeLabel != "" {
			var newLabels []string
			found := false
			for _, l := range labels {
				if strings.EqualFold(l, removeLabel) {
					found = true
				} else {
					newLabels = append(newLabels, l)
				}
			}
			if !found {
				fmt.Printf("task %s does not have label %q\n", nameArgs[0], removeLabel)
				return
			}
			labels = newLabels
			fmt.Printf("removed label %q from %s\n", removeLabel, nameArgs[0])
		}

		if len(labels) > 0 {
			fmMap["labels"] = labels
		} else {
			delete(fmMap, "labels")
		}

		fmBytes, err := yaml.Marshal(fmMap)
		if err != nil {
			log.Fatalf("failed to marshal front-matter: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(string(fmBytes))
		sb.WriteString("---\n")
		sb.WriteString(body)
		if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
			log.Fatalf("failed to write task file: %v", err)
		}
		return
	}

	// Handle --remove flag: clear a field from frontmatter
	if removeField != nil {
		raw, err := os.ReadFile(todo.Path)
		if err != nil {
			log.Fatalf("failed to read task file: %v", err)
		}
		contentStr := string(raw)
		if !strings.HasPrefix(contentStr, "---\n") {
			log.Fatal("task has no front-matter to modify")
		}
		parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
		if len(parts) != 2 {
			log.Fatal("failed to parse task front-matter")
		}
		var fmMap map[string]interface{}
		if err := yaml.Unmarshal([]byte(parts[0]), &fmMap); err != nil {
			log.Fatalf("failed to parse front-matter: %v", err)
		}
		if _, exists := fmMap[*removeField]; !exists {
			fmt.Printf("field %q is not set — nothing to remove\n", *removeField)
			return
		}
		delete(fmMap, *removeField)
		fmBytes, err := yaml.Marshal(fmMap)
		if err != nil {
			log.Fatalf("failed to marshal front-matter: %v", err)
		}
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(string(fmBytes))
		sb.WriteString("---\n")
		sb.WriteString(parts[1])
		if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
			log.Fatalf("failed to write task file: %v", err)
		}
		fmt.Printf("removed %s from %s\n", *removeField, todo.Name)
		return
	}

	// If targeted flags provided, apply them without opening editor
	if newSchedule != nil || newPriority != nil || newContent != nil {
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

					// Replace body content if --content was provided
					if newContent != nil {
						body = *newContent
						if !strings.HasSuffix(body, "\n") {
							body += "\n"
						}
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
					if newContent != nil {
						fmt.Printf("updated content\n")
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
				if err := yaml.Unmarshal([]byte(parts[0]), &fmData); err != nil {
					log.Printf("WARN: failed to parse frontmatter after edit: %v", err)
				} else if fmData.Schedule != "" && fmData.Schedule != "persistent" {
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
		fmt.Fprintln(os.Stderr, "  log [-f] [-n lines] [--level LEVEL] [--match PATTERN] [--since TIME] [--until TIME]")
		fmt.Fprintln(os.Stderr, "  config-validate           Validate config file")
		os.Exit(1)
	}
	switch args[0] {
	case "log":
		daemonLogCmd(args[1:])
	case "config-validate":
		daemonConfigValidateCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

// logLevelFromLine extracts the log level from a daemon log line.
// Lines contain markers like [warn], [error], [fatal], or are info-level by default.
func logLevelFromLine(line string) string {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "[error]") {
		return "error"
	}
	if strings.Contains(lower, "[fatal]") {
		return "error" // treat fatal as error level
	}
	if strings.Contains(lower, "[warn]") {
		return "warn"
	}
	// Worker and scheduler lines without explicit level markers are info.
	return "info"
}

// logLevelRank returns a numeric rank for level comparison (higher = more severe).
func logLevelRank(level string) int {
	switch strings.ToLower(level) {
	case "debug":
		return 0
	case "info":
		return 1
	case "warn":
		return 2
	case "error":
		return 3
	default:
		return 1
	}
}

// parseLogTimestamp extracts the HH:MM:SS timestamp from the start of a log line
// and returns it as a time.Time on today's date.
func parseLogTimestamp(line string) (time.Time, bool) {
	if len(line) < 8 {
		return time.Time{}, false
	}
	ts := line[:8]
	t, err := time.Parse("15:04:05", ts)
	if err != nil {
		return time.Time{}, false
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), true
}

// parseTimeArg parses a time argument that is either a Go duration ("1h", "30m")
// or a time-of-day ("10:00", "10:00:00").
func parseTimeArg(s string) (time.Time, error) {
	// Try Go duration first (e.g., "1h", "30m").
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	// Try time-of-day HH:MM:SS.
	if t, err := time.Parse("15:04:05", s); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
	}
	// Try time-of-day HH:MM.
	if t, err := time.Parse("15:04", s); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
	}
	return time.Time{}, fmt.Errorf("invalid time/duration: %s (use e.g. \"1h\", \"30m\", or \"10:00\")", s)
}

type logFilter struct {
	minLevel int    // minimum log level rank to show
	match    string // substring to match (case-insensitive)
	since    time.Time
	until    time.Time
}

func (lf *logFilter) matches(line string) bool {
	if line == "" {
		return false
	}
	if lf.minLevel > 0 {
		level := logLevelFromLine(line)
		if logLevelRank(level) < lf.minLevel {
			return false
		}
	}
	if lf.match != "" {
		if !strings.Contains(strings.ToLower(line), strings.ToLower(lf.match)) {
			return false
		}
	}
	if !lf.since.IsZero() || !lf.until.IsZero() {
		ts, ok := parseLogTimestamp(line)
		if !ok {
			return false // skip lines without timestamps when time filtering
		}
		if !lf.since.IsZero() && ts.Before(lf.since) {
			return false
		}
		if !lf.until.IsZero() && ts.After(lf.until) {
			return false
		}
	}
	return true
}

func (lf *logFilter) active() bool {
	return lf.minLevel > 0 || lf.match != "" || !lf.since.IsZero() || !lf.until.IsZero()
}

func daemonLogCmd(args []string) {
	follow := false
	numLines := 50
	filter := logFilter{}

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
		case "--level":
			if i+1 >= len(args) {
				log.Fatal("missing value for --level")
			}
			i++
			// Accept the minimum level; show that level and above.
			level := strings.ToLower(args[i])
			rank := logLevelRank(level)
			if level != "debug" && level != "info" && level != "warn" && level != "error" {
				log.Fatalf("invalid log level: %s (use debug, info, warn, or error)", args[i])
			}
			filter.minLevel = rank
		case "--match":
			if i+1 >= len(args) {
				log.Fatal("missing value for --match")
			}
			i++
			filter.match = args[i]
		case "--since":
			if i+1 >= len(args) {
				log.Fatal("missing value for --since")
			}
			i++
			t, err := parseTimeArg(args[i])
			if err != nil {
				log.Fatalf("--since: %v", err)
			}
			filter.since = t
		case "--until":
			if i+1 >= len(args) {
				log.Fatal("missing value for --until")
			}
			i++
			t, err := parseTimeArg(args[i])
			if err != nil {
				log.Fatalf("--until: %v", err)
			}
			filter.until = t
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil daemon log [-f] [-n lines] [--level LEVEL] [--match PATTERN]
                        [--since TIME] [--until TIME]

View the daemon log file (~/.anvil/daemon.log).

Options:
  -f, --follow     Follow the log (like tail -f)
  -n lines         Show last N lines (default 50)
  --level LEVEL    Minimum log level to show: debug, info, warn, error
                   Shows the specified level and above (e.g. --level warn shows warn+error)
  --match PATTERN  Show only lines containing PATTERN (case-insensitive)
  --since TIME     Show entries after TIME (duration like "1h" or time like "10:00")
  --until TIME     Show entries before TIME (duration like "1h" or time like "10:00")
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

	// Apply filters, then take the last N matching lines.
	if filter.active() {
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if filter.matches(line) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
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

	remainder := ""
	buf := make([]byte, 4096)
	for {
		select {
		case <-sigCh:
			return
		default:
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			chunk := remainder + string(buf[:n])
			chunkLines := strings.Split(chunk, "\n")
			// Last element may be incomplete; save it.
			remainder = chunkLines[len(chunkLines)-1]
			for _, line := range chunkLines[:len(chunkLines)-1] {
				if !filter.active() || filter.matches(line) {
					fmt.Println(line)
				}
			}
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

// daemonConfigValidateCmd validates the config file.
func daemonConfigValidateCmd(args []string) {
	showConfig := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--show" {
			showConfig = true
		} else if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: anvil daemon config-validate [options]")
			fmt.Println("")
			fmt.Println("Validate the daemon config file (~/.anvil/config.yaml).")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --show    Show parsed config in YAML format")
			fmt.Println("  -h        Show this help")
			return
		}
	}

	// Load config (validates YAML syntax)
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Validate durations
	if cfg.Timeout <= 0 {
		fmt.Fprintf(os.Stderr, "Error: invalid timeout %v (must be > 0)\n", cfg.Timeout)
		os.Exit(1)
	}
	if cfg.TickInterval <= 0 {
		fmt.Fprintf(os.Stderr, "Error: invalid tick_interval %v (must be > 0)\n", cfg.TickInterval)
		os.Exit(1)
	}

	// Validate runners
	if len(cfg.Runners) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no runners configured (must have at least one)\n")
		os.Exit(1)
	}

	// Validate webhooks if present
	if cfg.Webhooks != nil {
		for name, webhook := range cfg.Webhooks {
			if webhook.URL != "" {
				u, err := url.Parse(webhook.URL)
				if err != nil || !u.IsAbs() {
					fmt.Fprintf(os.Stderr, "Error: invalid webhook URL for %s: %s\n", name, webhook.URL)
					os.Exit(1)
				}
			}
		}
	}

	// Show warnings for deprecated fields
	// (currently no deprecated fields in config, but structure is here)

	// If --show, output config
	if showConfig {
		fmt.Println("tick_interval:", cfg.TickInterval)
		fmt.Println("max_workers:", cfg.MaxWorkers)
		fmt.Println("timeout:", cfg.Timeout)
		fmt.Println("runners:")
		for _, r := range cfg.Runners {
			fmt.Println("  -", r)
		}
		if cfg.Hooks.OnSuccess != "" || cfg.Hooks.OnFailure != "" {
			fmt.Println("hooks:")
			fmt.Println("  on_success:", cfg.Hooks.OnSuccess)
			fmt.Println("  on_failure:", cfg.Hooks.OnFailure)
		}
		if cfg.Webhooks != nil {
			fmt.Println("webhooks:")
			for name, wh := range cfg.Webhooks {
				fmt.Printf("  %s:\n", name)
				fmt.Printf("    url: %s\n", wh.URL)
				if len(wh.Events) > 0 {
					fmt.Printf("    events:\n")
					for _, e := range wh.Events {
						fmt.Printf("      - %s\n", e)
					}
				}
			}
		}
		return
	}

	// Config is valid
	fmt.Println("✓ Config is valid")
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

// taskNextCmd shows the next scheduled run time for tasks.
func taskNextCmd(args []string) {
	jsonOutput := false
	allProjects := false
	verbose := false
	var taskName string

	for _, a := range args {
		switch a {
		case "--json":
			jsonOutput = true
		case "--verbose", "-v":
			verbose = true
		case "--all":
			allProjects = true
		case "-h", "--help":
			fmt.Println("Usage: anvil task next [options] [task-name]")
			fmt.Println("")
			fmt.Println("Show the next scheduled run time for tasks.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --json      Output as JSON")
			fmt.Println("  --all       Show tasks from all watched projects")
			fmt.Println("  --verbose   Show time window constraints and quiet hours info")
			fmt.Println("  -h          Show this help")
			fmt.Println("")
			fmt.Println("If task-name is omitted, shows all tasks in the current project.")
			return
		default:
			taskName = a
		}
	}

	now := time.Now()

	if allProjects {
		// Load all watched projects
		watched, err := loadAllWatched()
		if err != nil {
			log.Fatalf("failed to read watched: %v", err)
		}
		if len(watched) == 0 {
			if jsonOutput {
				fmt.Println("[]")
			} else {
				fmt.Println("no watched projects")
			}
			return
		}

		type nextResult struct {
			Project string `json:"project"`
			Name    string `json:"name"`
			NextRun string `json:"next_run,omitempty"`
			In      string `json:"in,omitempty"`
			Error   string `json:"error,omitempty"`
		}
		var results []nextResult

		for _, w := range watched {
			proj, err := project.Load(w.Path)
			if err != nil {
				continue
			}
			todos, _ := proj.LoadTodos()
			for _, t := range todos {
				r := nextResult{
					Project: filepath.Base(w.Path),
					Name:    t.Name,
				}
				if t.Schedule == "" || t.Schedule == "once" {
					r.NextRun = "never (one-shot)"
				} else {
					p, err := cron.Parse(t.Schedule)
					if err != nil {
						r.Error = err.Error()
					} else {
						next, err := p.Next(now)
						if err != nil {
							r.Error = err.Error()
						} else {
							r.NextRun = next.Format("Mon Jan 2 15:04:05")
							r.In = formatDurationShort(time.Until(next))
						}
					}
				}
				results = append(results, r)
			}
		}

		if jsonOutput {
			data, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, r := range results {
				if r.Error != "" {
					fmt.Printf("%s/%s: error: %s\n", r.Project, r.Name, r.Error)
				} else {
					fmt.Printf("%s/%s: %s (%s)\n", r.Project, r.Name, r.NextRun, r.In)
				}
			}
		}
		return
	}

	// Single project mode
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

	if taskName != "" {
		// Show specific task
		todo := findTodo(todos, taskName)
		if todo == nil {
			fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
			os.Exit(1)
		}

		if todo.Schedule == "" || todo.Schedule == "once" {
			if jsonOutput {
				data, _ := json.MarshalIndent(map[string]string{
					"name": todo.Name, "next_run": "never", "note": "one-shot task"}, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("%s: never (one-shot task)\n", todo.Name)
			}
			return
		}

		// Load global config for quiet hours
		cfg, _ := config.Load()

		// Calculate window-aware next run
		next, err := daemon.NextAllowedRun(todo.Schedule, todo.Window, cfg.QuietHours, todo.Priority, now)
		if err != nil {
			if jsonOutput {
				data, _ := json.MarshalIndent(map[string]string{
					"name": todo.Name, "error": err.Error()}, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Fprintf(os.Stderr, "failed to calculate next run: %v\n", err)
			}
			os.Exit(1)
		}

		if jsonOutput {
			result := map[string]string{
				"name":     todo.Name,
				"schedule": todo.Schedule,
				"next_run": next.Format(time.RFC3339),
				"in":       formatDurationShort(time.Until(next)),
			}
			if todo.Window.Start != "" {
				result["window_start"] = todo.Window.Start
				result["window_end"] = todo.Window.End
				result["window_days"] = todo.Window.Days
			}
			if cfg.QuietHours.Enabled {
				result["quiet_hours"] = cfg.QuietHours.Start + "-" + cfg.QuietHours.End
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("%s: next run at %s (%s)\n", todo.Name, next.Format("Mon Jan 2 15:04:05 MST"), formatDurationShort(time.Until(next)))
			if verbose {
				if todo.Window.Start != "" {
					days := todo.Window.Days
					if days == "" {
						days = "all"
					}
					fmt.Printf("  window: %s–%s (days: %s)\n", todo.Window.Start, todo.Window.End, days)
				}
				if cfg.QuietHours.Enabled {
					fmt.Printf("  quiet hours: %s–%s (exempt: p%d and above)\n", cfg.QuietHours.Start, cfg.QuietHours.End, cfg.QuietHours.ExcludePriority)
				}
				if todo.Window.Start == "" && !cfg.QuietHours.Enabled {
					fmt.Println("  no time window constraints active")
				}
			}
		}
		return
	}

	// Show all tasks in current project
	type nextResult struct {
		Name    string `json:"name"`
		NextRun string `json:"next_run,omitempty"`
		In      string `json:"in,omitempty"`
		Error   string `json:"error,omitempty"`
	}
	var results []nextResult

	for _, t := range todos {
		r := nextResult{Name: t.Name}
		if t.Schedule == "" || t.Schedule == "once" {
			r.NextRun = "never (one-shot)"
		} else {
			p, err := cron.Parse(t.Schedule)
			if err != nil {
				r.Error = err.Error()
			} else {
				next, err := p.Next(now)
				if err != nil {
					r.Error = err.Error()
				} else {
					r.NextRun = next.Format("Mon Jan 2 15:04:05")
					r.In = formatDurationShort(time.Until(next))
				}
			}
		}
		results = append(results, r)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	} else {
		for _, r := range results {
			if r.Error != "" {
				fmt.Printf("%s: error: %s\n", r.Name, r.Error)
			} else {
				fmt.Printf("%s: %s (%s)\n", r.Name, r.NextRun, r.In)
			}
		}
	}
}

// taskExportCmd exports tasks to a JSON file.
func taskExportCmd(args []string) {
	exportAll := false
	outputFile := ""

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--all" || arg == "-a" {
			exportAll = true
		} else if arg == "-o" || arg == "--output" {
			if i+1 >= len(args) {
				log.Fatal("missing value for -o/--output")
			}
			i++
			outputFile = args[i]
		} else if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: anvil task export [options] [task-names...]")
			fmt.Println("")
			fmt.Println("Export tasks to a JSON file for sharing or backup.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --all, -a      Export all tasks from current project")
			fmt.Println("  -o, --output   Output file (default: stdout)")
			fmt.Println("  -h             Show this help")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  anvil task export task1.md task2.md -o backup.json")
			fmt.Println("  anvil task export --all -o all-tasks.json")
			return
		} else if strings.HasPrefix(arg, "-") {
			log.Fatalf("unknown flag: %s", arg)
		} else {
			break
		}
		i++
	}

	taskNames := args[i:]

	if !exportAll && len(taskNames) == 0 {
		log.Fatal("specify task names or use --all")
	}

	// Load current project
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

	// Filter tasks
	var toExport []project.Todo
	if exportAll {
		toExport = todos
	} else {
		for _, name := range taskNames {
			found := false
			for _, t := range todos {
				if t.Name == name {
					toExport = append(toExport, t)
					found = true
					break
				}
			}
			if !found {
				log.Fatalf("task not found: %s", name)
			}
		}
	}

	// Build export data
	type ExportTask struct {
		Name         string `json:"name"`
		Content      string `json:"content"`
		ProjectPath  string `json:"project_path"`
		Schedule     string `json:"schedule,omitempty"`
		Priority     int    `json:"priority"`
		Timeout      int    `json:"timeout,omitempty"`
		Retry        int    `json:"retry,omitempty"`
		RetryDelay   string `json:"retry_delay,omitempty"`
		Runner       string `json:"runner,omitempty"`
		Webhook      string `json:"webhook,omitempty"`
		PreCheck     string `json:"pre_check,omitempty"`
		OnSuccess    string `json:"on_success,omitempty"`
		OnFailure    string `json:"on_failure,omitempty"`
		Disabled     bool   `json:"disabled"`
		MaxLogSize   int64  `json:"max_log_size,omitempty"`
		SkipPerms    bool   `json:"skip_permissions"`
		AllowedTools string `json:"allowed_tools,omitempty"`
	}

	exportTasks := make([]ExportTask, len(toExport))
	for i, t := range toExport {
		var allowedToolsStr string
		if len(t.AllowedTools) > 0 {
			allowedToolsStr = strings.Join(t.AllowedTools, ",")
		}
		exportTasks[i] = ExportTask{
			Name:         t.Name,
			Content:      t.Content,
			ProjectPath:  abs,
			Schedule:     t.Schedule,
			Priority:     t.Priority,
			Timeout:      int(t.Timeout.Seconds()),
			Retry:        t.Retry,
			RetryDelay:   t.RetryDelay.String(),
			Runner:       t.Runner,
			Webhook:      t.Webhook,
			PreCheck:     t.PreCheck,
			OnSuccess:    t.OnSuccess,
			OnFailure:    t.OnFailure,
			Disabled:     t.Disabled,
			MaxLogSize:   t.MaxLogSize,
			SkipPerms:    t.SkipPermissions,
			AllowedTools: allowedToolsStr,
		}
	}

	type ExportData struct {
		Version    string        `json:"version"`
		ExportedAt time.Time     `json:"exported_at"`
		Tasks      []ExportTask  `json:"tasks"`
	}

	data := ExportData{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Tasks:      exportTasks,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal export: %v", err)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			log.Fatalf("failed to write export file: %v", err)
		}
		fmt.Printf("Exported %d tasks to %s\n", len(exportTasks), outputFile)
	} else {
		fmt.Println(string(jsonData))
	}
}

// taskImportCmd imports tasks from a JSON file.
func taskImportCmd(args []string) {
	inputFile := ""
	basePath := ""
	dryRun := false
	force := false

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--base-path" {
			if i+1 >= len(args) {
				log.Fatal("missing value for --base-path")
			}
			i++
			basePath = args[i]
		} else if arg == "--dry-run" || arg == "-n" {
			dryRun = true
		} else if arg == "--force" || arg == "-f" {
			force = true
		} else if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: anvil task import <file> [options]")
			fmt.Println("")
			fmt.Println("Import tasks from a JSON export file.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --base-path    Remap project paths during import")
			fmt.Println("  --dry-run, -n  Preview without creating tasks")
			fmt.Println("  --force, -f   Overwrite existing tasks")
			fmt.Println("  -h            Show this help")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  anvil task import backup.json")
			fmt.Println("  anvil task import backup.json --dry-run")
			fmt.Println("  anvil task import backup.json --base-path /new/project/path")
			return
		} else if strings.HasPrefix(arg, "-") {
			log.Fatalf("unknown flag: %s", arg)
		} else {
			if inputFile != "" {
				log.Fatal("multiple input files specified")
			}
			inputFile = arg
		}
		i++
	}

	if inputFile == "" {
		log.Fatal("missing input file")
	}

	data, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("failed to read import file: %v", err)
	}

	type ExportTask struct {
		Name         string `json:"name"`
		Content      string `json:"content"`
		ProjectPath  string `json:"project_path"`
		Schedule     string `json:"schedule,omitempty"`
		Priority     int    `json:"priority"`
		Timeout      int    `json:"timeout,omitempty"`
		Retry        int    `json:"retry,omitempty"`
		RetryDelay   string `json:"retry_delay,omitempty"`
		Runner       string `json:"runner,omitempty"`
		Webhook      string `json:"webhook,omitempty"`
		PreCheck     string `json:"pre_check,omitempty"`
		OnSuccess    string `json:"on_success,omitempty"`
		OnFailure    string `json:"on_failure,omitempty"`
		Disabled     bool   `json:"disabled"`
		MaxLogSize   int64  `json:"max_log_size,omitempty"`
		SkipPerms    bool   `json:"skip_permissions"`
		AllowedTools string `json:"allowed_tools,omitempty"`
	}

	type ExportData struct {
		Version    string       `json:"version"`
		ExportedAt time.Time    `json:"exported_at"`
		Tasks      []ExportTask `json:"tasks"`
	}

	var export ExportData
	if err := json.Unmarshal(data, &export); err != nil {
		log.Fatalf("failed to parse import file: %v", err)
	}

	if len(export.Tasks) == 0 {
		log.Fatal("no tasks found in import file")
	}

	// Determine target project path
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}
	targetPath := abs
	if basePath != "" {
		targetPath = basePath
	}

	proj, err := project.Load(targetPath)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	// Check for existing tasks
	existingTodos, _ := proj.LoadTodos()
	existingNames := make(map[string]bool)
	for _, t := range existingTodos {
		existingNames[t.Name] = true
	}

	importCount := 0
	skipCount := 0

	for _, et := range export.Tasks {
		// Check if task exists
		if existingNames[et.Name] && !force {
			fmt.Printf("Skipping %s (already exists, use --force to overwrite)\n", et.Name)
			skipCount++
			continue
		}

		if dryRun {
			fmt.Printf("Would import: %s\n", et.Name)
			importCount++
			continue
		}

		// Build frontmatter and content
		var frontmatter []string
		if et.Schedule != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("schedule: %q", et.Schedule))
		}
		if et.Priority != 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("priority: %d", et.Priority))
		}
		if et.Timeout > 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("timeout: %ds", et.Timeout))
		}
		if et.Retry > 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("retry: %d", et.Retry))
		}
		if et.RetryDelay != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("retry_delay: %q", et.RetryDelay))
		}
		if et.Runner != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("runner: %q", et.Runner))
		}
		if et.Webhook != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("webhook: %q", et.Webhook))
		}
		if et.PreCheck != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("pre_check: %q", et.PreCheck))
		}
		if et.OnSuccess != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("on_success: %q", et.OnSuccess))
		}
		if et.OnFailure != "" {
			frontmatter = append(frontmatter, fmt.Sprintf("on_failure: %q", et.OnFailure))
		}
		if et.Disabled {
			frontmatter = append(frontmatter, "disabled: true")
		}
		if et.MaxLogSize > 0 {
			frontmatter = append(frontmatter, fmt.Sprintf("max_log_size: %d", et.MaxLogSize))
		}
		if et.SkipPerms {
			frontmatter = append(frontmatter, "skip_permissions: true")
		}
		if et.AllowedTools != "" {
			tools := strings.Split(et.AllowedTools, ",")
			for i := range tools {
				tools[i] = strings.TrimSpace(tools[i])
			}
			frontmatter = append(frontmatter, fmt.Sprintf("allowed_tools: [%s]", strings.Join(tools, ", ")))
		}

		// Build full content with frontmatter
		var content string
		if len(frontmatter) > 0 {
			content = "---\n" + strings.Join(frontmatter, "\n") + "\n---\n\n" + et.Content
		} else {
			content = et.Content
		}

		// Write task file
		todoDir := filepath.Join(proj.Path, ".anvil", "todos")
		if err := os.MkdirAll(todoDir, 0755); err != nil {
			log.Fatalf("failed to create todos directory: %v", err)
		}

		taskFile := filepath.Join(todoDir, et.Name)
		if err := os.WriteFile(taskFile, []byte(content), 0644); err != nil {
			log.Fatalf("failed to write task file: %v", err)
		}

		fmt.Printf("Imported: %s\n", et.Name)
		importCount++
	}

	if dryRun {
		fmt.Printf("\nDry run: would import %d task(s), skip %d\n", importCount, skipCount)
	} else {
		fmt.Printf("\nImported %d task(s) from %s\n", importCount, inputFile)
	}
}

// Conflict represents a scheduling conflict between two tasks
type Conflict struct {
	Task1    string
	Task2    string
	Schedule1 string
	Schedule2 string
	Reason   string
}

// detectConflicts analyzes tasks for scheduling conflicts
func detectConflicts(todos []project.Todo) []Conflict {
	var conflicts []Conflict

	// Build a map of task name to schedule
	taskSchedules := make(map[string]string)
	for _, t := range todos {
		if t.Schedule != "" {
			taskSchedules[t.Name] = t.Schedule
		}
	}

	// Check for frequency-based conflicts
	for i := range todos {
		for j := i + 1; j < len(todos); j++ {
			t1, t2 := todos[i], todos[j]
			if t1.Schedule == "" || t2.Schedule == "" {
				continue
			}

			// Both have schedules - check for conflicts
			// High frequency: both run every minute
			if t1.Schedule == "*/1 * * * *" && t2.Schedule == "*/1 * * * *" {
				conflicts = append(conflicts, Conflict{
					Task1: t1.Name, Task2: t2.Name,
					Schedule1: t1.Schedule, Schedule2: t2.Schedule,
					Reason: "Both tasks run every minute - may compete for resources",
				})
				continue
			}

			// Check for same minute patterns that could overlap
			// e.g., */5 and */10 at minute 0
			if strings.Contains(t1.Schedule, "*/5") && strings.Contains(t2.Schedule, "*/") {
				conflicts = append(conflicts, Conflict{
					Task1: t1.Name, Task2: t2.Name,
					Schedule1: t1.Schedule, Schedule2: t2.Schedule,
					Reason: "Tasks may run at the same time - check schedules",
				})
			}
		}
	}

	return conflicts
}

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

// taskPipelineCmd visualizes task dependency pipelines as ASCII trees, DOT graphs, or verbose output.
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

func taskWaitCmd(args []string) {
	var timeoutDur time.Duration
	matchPattern := ""
	var nameArgs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--timeout", "-t":
			if i+1 >= len(args) {
				log.Fatal("missing value for --timeout")
			}
			i++
			var err error
			timeoutDur, err = time.ParseDuration(args[i])
			if err != nil {
				log.Fatalf("invalid timeout duration: %v", err)
			}
		case "--match", "-m":
			if i+1 >= len(args) {
				log.Fatal("missing value for --match")
			}
			i++
			matchPattern = strings.ToLower(args[i])
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil task wait <name> [--timeout DURATION]
       anvil task wait --match <pattern> [--timeout DURATION]

Block until a running task completes.

Options:
  --timeout, -t DUR   Cancel wait after duration (e.g., 5m, 1h)
  --match, -m PAT     Wait for first task matching pattern (case-insensitive)

Exit codes:
  0  Task completed successfully
  1  Task failed
  2  Wait timed out
`)
			os.Exit(0)
		default:
			nameArgs = append(nameArgs, args[i])
		}
	}

	if len(nameArgs) == 0 && matchPattern == "" {
		fmt.Fprintf(os.Stderr, "usage: anvil task wait <name> [--timeout DURATION]\n")
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Fprintln(os.Stderr, "daemon is not running")
		os.Exit(1)
	}

	// Build the target name for matching
	targetName := ""
	if len(nameArgs) > 0 {
		targetName = nameArgs[0]
		if !strings.HasSuffix(targetName, ".md") {
			targetName += ".md"
		}
	}

	// Check that the task is actually running before we start waiting
	tasks, err := daemon.SendPsRequest()
	if err != nil {
		log.Fatalf("failed to query daemon: %v", err)
	}

	found := findRunningTask(tasks, targetName, matchPattern)
	if found == nil {
		if targetName != "" {
			fmt.Fprintf(os.Stderr, "task not currently running: %s\n", targetName)
		} else {
			fmt.Fprintf(os.Stderr, "no running task matches pattern: %s\n", matchPattern)
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "waiting for %s ...\n", found.Name)

	// Set up timeout if specified
	var deadline <-chan time.Time
	if timeoutDur > 0 {
		deadline = time.After(timeoutDur)
	}

	// Poll until the task is no longer running
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	waitingFor := found.Name
	for {
		select {
		case <-deadline:
			fmt.Fprintf(os.Stderr, "timed out after %s\n", timeoutDur)
			os.Exit(2)
		case <-ticker.C:
			tasks, err := daemon.SendPsRequest()
			if err != nil {
				// Daemon may have gone away — treat as task completed
				fmt.Fprintf(os.Stderr, "daemon unreachable, assuming task completed\n")
				os.Exit(0)
			}
			still := false
			for _, t := range tasks {
				if t.Name == waitingFor {
					still = true
					break
				}
			}
			if !still {
				// Task is no longer running — check last run record for exit status
				exitCode := checkTaskResult(waitingFor)
				if exitCode == 0 {
					fmt.Fprintf(os.Stderr, "task completed successfully: %s\n", waitingFor)
				} else {
					fmt.Fprintf(os.Stderr, "task failed: %s\n", waitingFor)
				}
				os.Exit(exitCode)
			}
		}
	}
}

// findRunningTask finds a running task by exact name or pattern match.
func findRunningTask(tasks []daemon.TaskInfo, targetName, matchPattern string) *daemon.TaskInfo {
	for i := range tasks {
		if targetName != "" && tasks[i].Name == targetName {
			return &tasks[i]
		}
		if matchPattern != "" && strings.Contains(strings.ToLower(tasks[i].Name), matchPattern) {
			return &tasks[i]
		}
	}
	return nil
}

// checkTaskResult checks the most recent run record for a task to determine if it succeeded.
// Returns 0 for success, 1 for failure.
func checkTaskResult(taskName string) int {
	abs, err := filepath.Abs(".")
	if err != nil {
		return 1
	}
	proj, err := project.Load(abs)
	if err != nil {
		return 1
	}
	todos, err := proj.LoadTodos()
	if err != nil {
		return 1
	}
	todo := findTodo(todos, taskName)
	if todo == nil {
		// Task may have been a one-shot that was removed on success
		return 0
	}
	if todo.ID == "" {
		return 0
	}
	rec, err := project.ReadCurrentRunRecord(abs, todo.ID)
	if err != nil {
		// No record found — assume success
		return 0
	}
	if rec.Success {
		return 0
	}

	// Task failed - show runbook if available
	showRunbookHint(todo)

	return 1
}

// showRunbookHint displays runbook information when a task fails.
func showRunbookHint(todo *project.Todo) {
	if todo.Runbook == "" {
		return
	}

	fmt.Fprintf(os.Stderr, "\nRunbook available for this task:\n")
	if strings.HasPrefix(todo.Runbook, "http://") || strings.HasPrefix(todo.Runbook, "https://") {
		fmt.Fprintf(os.Stderr, "  %s\n", todo.Runbook)
		fmt.Fprintf(os.Stderr, "  View: anvil task runbook %s\n", todo.Name)
	} else {
		// Show inline runbook preview
		lines := strings.Split(todo.Runbook, "\n")
		preview := strings.Join(lines[:min(5, len(lines))], "\n")
		if len(lines) > 5 {
			preview += "\n  ..."
		}
		fmt.Fprintf(os.Stderr, "  (inline runbook)\n%s\n", preview)
	}
	fmt.Fprintf(os.Stderr, "\n")
}

// suggestStagger prints a hint for staggering overlapping schedules.
func suggestStagger(schedule string, overlapCount int) {
	// Parse the cron fields to offer meaningful suggestions
	fields := strings.Fields(schedule)
	if len(fields) < 5 {
		return
	}
	minute := fields[0]
	// If it's a fixed minute (e.g. "0" or "30"), suggest offsetting
	if _, err := fmt.Sscanf(minute, "%d", new(int)); err == nil {
		offset := (overlapCount + 1) * 5 // suggest 5-min offsets
		if offset >= 60 {
			offset = offset % 60
		}
		suggested := make([]string, len(fields))
		copy(suggested, fields)
		suggested[0] = fmt.Sprintf("%d", offset)
		fmt.Fprintf(os.Stderr, "Hint: Consider staggering with schedule %q to avoid overlap\n", strings.Join(suggested, " "))
	} else if strings.HasPrefix(minute, "*/") {
		// Interval-based, suggest a different offset start
		fmt.Fprintf(os.Stderr, "Hint: Consider staggering schedules or increasing max_workers in .anvil/config.yaml\n")
	}
}

func taskOverlapsCmd(args []string) {
	allProjects := false
	for _, a := range args {
		switch a {
		case "-a", "--all":
			allProjects = true
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil task overlaps [-a|--all]

Show all schedule conflicts and overlapping task runs.

Groups tasks by time slot to identify scheduling bottlenecks.

Options:
  -a, --all    Check across all watched projects
`)
			os.Exit(0)
		}
	}

	type parsedTask struct {
		name     string
		project  string
		schedule string
		parser   *cron.Parser
	}

	var tasks []parsedTask

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

	// Load max_workers for display
	maxWorkers := 4
	cfg, _ := config.Load()
	if cfg != nil && cfg.MaxWorkers > 0 {
		maxWorkers = cfg.MaxWorkers
	}

	// Parse schedules
	for _, p := range projects {
		projName := filepath.Base(p.path)
		for _, t := range p.todos {
			if t.Schedule == "" || t.Schedule == "persistent" || t.Disabled {
				continue
			}
			parser, err := cron.Parse(t.Schedule)
			if err != nil {
				continue
			}
			name := strings.TrimSuffix(t.Name, ".md")
			tasks = append(tasks, parsedTask{
				name:     name,
				project:  projName,
				schedule: t.Schedule,
				parser:   parser,
			})
		}
	}

	if len(tasks) == 0 {
		fmt.Println("no scheduled tasks to analyze")
		return
	}

	// Generate next 60 minutes of run times and group by minute
	now := time.Now().Truncate(time.Minute)
	window := 24 * time.Hour
	minuteMap := make(map[time.Time][]string) // minute -> task names
	for _, t := range tasks {
		cur := now
		for i := 0; i < 1440; i++ { // up to 1440 minutes in a day
			next, err := t.parser.Next(cur)
			if err != nil {
				break
			}
			if next.After(now.Add(window)) {
				break
			}
			key := next.Truncate(time.Minute)
			minuteMap[key] = append(minuteMap[key], t.name)
			cur = next
		}
	}

	// Collect conflicts (minutes with >1 task), sorted by time
	type conflict struct {
		minute time.Time
		tasks  []string
	}
	var conflicts []conflict
	for minute, names := range minuteMap {
		if len(names) > 1 {
			conflicts = append(conflicts, conflict{minute: minute, tasks: names})
		}
	}

	// Sort by time
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].minute.Before(conflicts[j].minute)
	})

	// Deduplicate by task group (same set of tasks at different times)
	type groupKey struct {
		key   string
		times []string
		tasks []string
		count int
	}
	groups := make(map[string]*groupKey)
	for _, c := range conflicts {
		sorted := make([]string, len(c.tasks))
		copy(sorted, c.tasks)
		sort.Strings(sorted)
		key := strings.Join(sorted, "+")
		if g, ok := groups[key]; ok {
			g.count++
			if len(g.times) < 3 {
				g.times = append(g.times, c.minute.Format("15:04"))
			}
		} else {
			groups[key] = &groupKey{
				key:   key,
				times: []string{c.minute.Format("15:04")},
				tasks: sorted,
				count: 1,
			}
		}
	}

	if len(groups) == 0 {
		fmt.Printf("OK: %d scheduled task(s) — no overlapping runs detected in the next 24h\n", len(tasks))
		return
	}

	// Print results in table format
	fmt.Printf("Schedule overlaps found (next 24h, %d worker slots):\n\n", maxWorkers)
	fmt.Printf("%-12s  %-6s  %s\n", "TIME", "TASKS", "NAMES")
	fmt.Printf("%-12s  %-6s  %s\n", "----", "-----", "-----")

	// Sort groups by first occurrence time
	var groupList []*groupKey
	for _, g := range groups {
		groupList = append(groupList, g)
	}
	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].times[0] < groupList[j].times[0]
	})

	totalConflicts := 0
	for _, g := range groupList {
		timeStr := strings.Join(g.times, ", ")
		if g.count > len(g.times) {
			timeStr += fmt.Sprintf(" (+%d more)", g.count-len(g.times))
		}
		severity := "Note"
		if len(g.tasks) >= maxWorkers {
			severity = "WARNING"
		} else if len(g.tasks) >= 3 {
			severity = "Warning"
		}

		fmt.Printf("%-12s  %-6d  %s", g.times[0], len(g.tasks), strings.Join(g.tasks, ", "))
		if len(g.tasks) >= maxWorkers {
			fmt.Printf(" (%d tasks >= %d workers)", len(g.tasks), maxWorkers)
		}
		fmt.Println()

		if g.count > 1 {
			fmt.Printf("  %s: repeats %d times in 24h (%s)\n", severity, g.count, timeStr)
		} else {
			if severity != "Note" {
				fmt.Printf("  %s: %d tasks competing for %d worker slots\n", severity, len(g.tasks), maxWorkers)
			}
		}
		totalConflicts += g.count
	}

	fmt.Printf("\n%d overlap(s) across %d time group(s). Consider staggering schedules or increasing max_workers.\n", totalConflicts, len(groupList))
	os.Exit(1)
}

// formatDurationShort formats a duration as short human-readable relative time.
func formatDurationShort(d time.Duration) string {
	if d < 0 {
		d = -d
		if d < time.Minute {
			return "now"
		}
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours()/24), int(d.Hours())%24)
}

// taskPredictCmd shows failure prediction analysis for a task.
func taskPredictCmd(args []string) {
	flags := flag.NewFlagSet("task predict", flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: anvil task predict <task-name>\n\n")
		fmt.Fprintf(os.Stderr, "Show failure prediction analysis for a task based on historical runs.\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		os.Exit(1)
	}

	if flags.NArg() != 1 {
		flags.Usage()
		os.Exit(1)
	}

	taskName := flags.Arg(0)

	// Load project
	proj, err := project.Load(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load todos
	todos, err := proj.LoadTodos()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Find the task
	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	// Load run records
	runs, err := project.ReadAllRunRecords(proj.Path, todo.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading run records: %v\n", err)
		os.Exit(1)
	}

	if len(runs) < 5 {
		fmt.Printf("Insufficient data for prediction (need at least 5 runs, have %d)\n", len(runs))
		os.Exit(0)
	}

	// Get thresholds
	thresholds := todo.RiskThreshold
	if thresholds.HighThreshold == 0 {
		thresholds = project.GetDefaultRiskThresholds()
	}

	// Analyze
	analyzer := project.NewRiskAnalyzer(proj.Path, todo.ID, thresholds)
	riskState, err := analyzer.AnalyzeTask(runs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error analyzing risk: %v\n", err)
		os.Exit(1)
	}

	// Print analysis
	fmt.Printf("Analysis of last %d runs:\n", len(runs))
	fmt.Println(strings.Repeat("─", 44))
	fmt.Printf("Success rate: %.0f%% (%d/%d)\n", riskState.HistoricalStats.SuccessRate*100,
		int(riskState.HistoricalStats.SuccessRate*float64(riskState.HistoricalStats.TotalRuns)),
		riskState.HistoricalStats.TotalRuns)
	fmt.Printf("Recent failures: %d of last 10 runs\n", riskState.HistoricalStats.RecentFailures)
	fmt.Printf("Trend: %s\n", riskState.HistoricalStats.TrendDirection)
	fmt.Println(strings.Repeat("─", 44))

	if len(riskState.RiskFactors) > 0 {
		fmt.Println("Risk Factors:")
		for _, f := range riskState.RiskFactors {
			fmt.Printf("  • %s: %s\n", f.Type, f.Value)
		}
		fmt.Println(strings.Repeat("─", 44))
	}

	riskLabel := string(riskState.CurrentRisk)
	fmt.Printf("Risk Score: %.2f (%s)\n", riskState.RiskScore, riskLabel)
	fmt.Printf("Prediction: %s\n", analyzer.GetPrediction(riskState.RiskScore))
	fmt.Println(strings.Repeat("─", 44))

	// Recommendations
	if riskState.CurrentRisk == project.RiskLevelHigh {
		fmt.Println("Recommendation: Task is at high risk. Consider investigating the risk factors above.")
	} else if riskState.CurrentRisk == project.RiskLevelMedium {
		fmt.Println("Recommendation: Task shows elevated risk. Monitor closely.")
	} else {
		fmt.Println("Recommendation: Task is healthy. Continue monitoring.")
	}
}

func templateCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil template <subcommand> [options]\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  ls           List available templates\n")
		fmt.Fprintf(os.Stderr, "  get <name>   Show template details\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	switch args[0] {
	case "ls":
		templateListCmd(args[1:])
	case "get":
		templateGetCmd(args[1:])
	case "-h", "--help":
		fmt.Fprintf(os.Stderr, "usage: anvil template <subcommand> [options]\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  ls           List available templates\n")
		fmt.Fprintf(os.Stderr, "  get <name>   Show template details\n")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown template subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func templateListCmd(_ []string) {
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	templates, err := project.ListTemplates(abs)
	if err != nil {
		log.Fatalf("failed to list templates: %v", err)
	}

	if len(templates) == 0 {
		fmt.Println("No templates found")
		fmt.Println("\nCreate templates in:")
		fmt.Println("  .anvil/templates/   (project-specific)")
		fmt.Println("  ~/.anvil/templates/ (global)")
		return
	}

	fmt.Println("Available templates:")
	for _, t := range templates {
		fmt.Printf("  %s\n", t.Name)
	}
}

func templateGetCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil template get <name>\n")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	tmpl, err := project.LoadTemplate(abs, args[0])
	if err != nil {
		log.Fatalf("template not found: %s\n", args[0])
	}

	fmt.Printf("Template: %s\n", tmpl.Name)
	// Display key template fields
	spec := tmpl.Spec
	if spec.Schedule != "" {
		fmt.Printf("Schedule: %s\n", spec.Schedule)
	}
	if spec.Priority > 0 {
		fmt.Printf("Priority: %d\n", spec.Priority)
	}
	if len(spec.AllowedTools) > 0 {
		fmt.Printf("AllowedTools: %s\n", strings.Join(spec.AllowedTools, ", "))
	}
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
			Path       string `json:"path"`
			Tasks      int    `json:"tasks"`
			Running    int    `json:"running"`
			Status     string `json:"status"`
			WatchedAt  string `json:"watched_at,omitempty"`
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

	// Parse --runs N flag
	runsCount := 1
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--runs" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "invalid --runs value: must be a positive integer")
				os.Exit(1)
			}
			runsCount = n
			i++ // skip the value
		} else if strings.HasPrefix(args[i], "--runs=") {
			val := strings.TrimPrefix(args[i], "--runs=")
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "invalid --runs value: must be a positive integer")
				os.Exit(1)
			}
			runsCount = n
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	if len(filteredArgs) == 0 {
		logsMultiplex()
		return
	}

	// Single task mode: anvil logs <name> [--runs N]
	name := filteredArgs[0]
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

	// Check if the task is currently running (only for single-run mode)
	if runsCount == 1 {
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
	}

	// Task is not running — print log(s) from completed runs
	if taskID == "" {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", name)
		os.Exit(1)
	}

	if runsCount == 1 {
		// Single run: show latest log
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
	} else {
		// Multiple runs: show last N run logs with headers
		records, err := project.ReadAllRunRecords(abs, taskID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read run records for task %s: %v\n", name, err)
			os.Exit(1)
		}
		if len(records) == 0 {
			fmt.Fprintf(os.Stderr, "no run records found for task %s\n", name)
			os.Exit(1)
		}

		// Limit to requested count
		if runsCount > len(records) {
			runsCount = len(records)
		}

		// Print runs oldest-first for chronological reading
		for i := runsCount - 1; i >= 0; i-- {
			rec := records[i]
			status := "success"
			if !rec.Success {
				status = "failure"
			}
			fmt.Printf("=== Run %s [%s] %s → %s ===\n",
				rec.RunID,
				status,
				rec.Started.Format("2006-01-02 15:04:05"),
				rec.Finished.Format("15:04:05"))

			logPath := rawLogPath(abs, rec.TaskID, rec.RunID)
			data, err := os.ReadFile(logPath)
			if err != nil {
				fmt.Printf("  (log not available: %v)\n", err)
			} else {
				fmt.Print(string(data))
				if len(data) > 0 && data[len(data)-1] != '\n' {
					fmt.Println()
				}
			}
			if i > 0 {
				fmt.Println()
			}
		}
	}
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

// taskEditApply applies schedule, priority, and/or disabled changes to a single task file.
func taskEditApply(todo *project.Todo, projectRoot string, newSchedule *string, newPriority *int, setDisabled *bool) error {
	raw, err := os.ReadFile(todo.Path)
	if err != nil {
		return fmt.Errorf("read task file: %w", err)
	}

	contentStr := string(raw)
	if !strings.HasPrefix(contentStr, "---\n") {
		return fmt.Errorf("task %s has no front-matter", todo.Name)
	}
	parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
	if len(parts) != 2 {
		return fmt.Errorf("failed to parse front-matter for %s", todo.Name)
	}

	var fmMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(parts[0]), &fmMap); err != nil {
		return fmt.Errorf("parse front-matter for %s: %w", todo.Name, err)
	}

	changes := []string{}

	if newSchedule != nil {
		fmMap["schedule"] = *newSchedule
		changes = append(changes, fmt.Sprintf("schedule: %s", *newSchedule))
	}

	if setDisabled != nil {
		if *setDisabled {
			fmMap["disabled"] = true
		} else {
			delete(fmMap, "disabled")
		}
		changes = append(changes, fmt.Sprintf("disabled: %t", *setDisabled))
	}

	priority := todo.Priority
	if newPriority != nil {
		priority = *newPriority
	}

	fmBytes, err := yaml.Marshal(fmMap)
	if err != nil {
		return fmt.Errorf("marshal front-matter for %s: %w", todo.Name, err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(string(fmBytes))
	sb.WriteString("---\n")
	sb.WriteString(parts[1])

	if priority != todo.Priority {
		newDir := filepath.Join(projectRoot, ".anvil", "todos", fmt.Sprintf("p%d", priority))
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return fmt.Errorf("create priority directory: %w", err)
		}
		newPath := filepath.Join(newDir, filepath.Base(todo.Path))
		if err := os.WriteFile(newPath, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("write task file: %w", err)
		}
		if err := os.Remove(todo.Path); err != nil {
			return fmt.Errorf("remove old task file: %w", err)
		}
		changes = append(changes, fmt.Sprintf("priority: p%d -> p%d", todo.Priority, priority))
	} else {
		if err := os.WriteFile(todo.Path, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("write task file: %w", err)
		}
	}

	fmt.Printf("  %s: %s\n", todo.Name, strings.Join(changes, ", "))
	return nil
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

func usageCmd(args []string) {
	var projectFilter string
	var taskFilter string
	var sinceStr string
	var showMetrics bool
	var topN int
	var jsonOut bool

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
		case "--metrics":
			showMetrics = true
			i++
		case "--top":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err != nil || n <= 0 {
					fmt.Fprintln(os.Stderr, "--top requires a positive number")
					os.Exit(1)
				}
				topN = n
				i += 2
			} else {
				fmt.Fprintln(os.Stderr, "--top requires a value")
				os.Exit(1)
			}
		case "--json":
			jsonOut = true
			i++
		case "-h", "--help":
			if showMetrics {
				fmt.Fprintf(os.Stderr, `Usage: anvil usage --metrics [options]

Show task runtime metrics including total runtime, average execution time,
run count, and success rates.

Options:
  --project <path>   Filter to a specific project (default: all watched projects)
  --task <name>     Filter to a specific task name
  --since <date>    Show metrics since date (YYYY-MM-DD, default: 30 days ago)
  --top <N>         Show only top N tasks by runtime
  --json            Output as JSON
`)
			} else {
				fmt.Fprintf(os.Stderr, `Usage: anvil usage [options]

Show LLM token usage and estimated costs across tasks and projects.

Options:
  --project <path>   Filter to a specific project (default: all watched projects)
  --task <name>      Filter to a specific task name
  --since <date>     Show usage since date (YYYY-MM-DD, default: 7 days ago)
  --metrics          Show task runtime metrics (total runtime, success rate, etc.)
  --top <N>          Limit output to top N tasks (use with --metrics)
  --json             Output as JSON
`)
			}
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	// Default: last 7 days for usage, 30 days for metrics
	sinceDays := 7
	if showMetrics {
		sinceDays = 30
	}
	since := time.Now().AddDate(0, 0, -sinceDays)
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

	// If --metrics requested, show runtime metrics
	if showMetrics {
		type taskMetrics struct {
			Project        string
			TaskName       string
			TotalRuntime   time.Duration
			AvgRuntime     time.Duration
			Runs           int
			Successes      int
			SuccessRate    float64
			InputTokens    int
			OutputTokens   int
			Cost           float64
		}

		var allMetrics []taskMetrics
		var projectTotals map[string]struct {
			runtime   time.Duration
			runs      int
			cost      float64
		}
		projectTotals = make(map[string]struct {
			runtime   time.Duration
			runs      int
			cost      float64
		})
		var grandTotalRuntime time.Duration

		// Reload data for metrics
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

				var tm taskMetrics
				tm.Project = projName
				tm.TaskName = todo.Name

				for _, rec := range records {
					if rec.Started.Before(since) {
						continue
					}
					if rec.Finished.IsZero() {
						continue // skip incomplete runs
					}
					tm.Runs++
					runtime := rec.Finished.Sub(rec.Started)
					tm.TotalRuntime += runtime
					if rec.Success {
						tm.Successes++
					}
					tm.InputTokens += rec.InputTokens
					tm.OutputTokens += rec.OutputTokens
					tm.Cost += rec.EstimatedCostUSD
				}

				if tm.Runs > 0 {
					tm.AvgRuntime = tm.TotalRuntime / time.Duration(tm.Runs)
					tm.SuccessRate = float64(tm.Successes) / float64(tm.Runs)

					// Recalculate cost if needed
					if tm.Cost == 0 && (tm.InputTokens > 0 || tm.OutputTokens > 0) {
						tm.Cost = float64(tm.InputTokens)/1_000_000*inputRate +
							float64(tm.OutputTokens)/1_000_000*outputRate
					}

					allMetrics = append(allMetrics, tm)
					grandTotalRuntime += tm.TotalRuntime

					// Accumulate project totals
					if pt, ok := projectTotals[projName]; ok {
						pt.runtime += tm.TotalRuntime
						pt.runs += tm.Runs
						pt.cost += tm.Cost
						projectTotals[projName] = pt
					} else {
						projectTotals[projName] = struct {
							runtime   time.Duration
							runs      int
							cost      float64
						}{runtime: tm.TotalRuntime, runs: tm.Runs, cost: tm.Cost}
					}
				}
			}
		}

		if len(allMetrics) == 0 {
			fmt.Printf("\nNo completed runs found since %s.\n", since.Format("2006-01-02"))
			return
		}

		// Sort by total runtime descending
		sort.Slice(allMetrics, func(i, j int) bool {
			return allMetrics[i].TotalRuntime > allMetrics[j].TotalRuntime
		})

		// Apply topN limit
		displayLimit := len(allMetrics)
		if topN > 0 && topN < displayLimit {
			displayLimit = topN
		}

		// Output JSON if requested
		if jsonOut {
			type jsonOutput struct {
				Period     string          `json:"period"`
				Since      string          `json:"since"`
				Tasks      []taskMetrics   `json:"tasks"`
				TotalCost  float64         `json:"total_cost"`
				TotalRuns  int             `json:"total_runs"`
			}
			var totalCost float64
			var totalRuns int
			for _, tm := range allMetrics {
				totalCost += tm.Cost
				totalRuns += tm.Runs
			}
			jo := jsonOutput{
				Period:    fmt.Sprintf("%dd", sinceDays),
				Since:     since.Format("2006-01-02"),
				Tasks:     allMetrics[:displayLimit],
				TotalCost: totalCost,
				TotalRuns: totalRuns,
			}
			data, err := json.MarshalIndent(jo, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal JSON: %v", err)
			}
			fmt.Println(string(data))
			return
		}

		// Print human-readable metrics
		fmt.Printf("\n")
		fmt.Printf("TASK RUNTIME SUMMARY\n")
		fmt.Printf("====================\n")
		fmt.Printf("Period: %s to %s\n\n", since.Format("2006-01-02"), time.Now().Format("2006-01-02"))

		fmt.Printf("%-30s %10s %10s %6s %8s\n", "TASK", "TOTAL", "AVG", "RUNS", "SUCCESS")
		fmt.Printf("%-30s %10s %10s %6s %8s\n", "----", "-----", "---", "----", "-------")

		for _, tm := range allMetrics[:displayLimit] {
			name := tm.TaskName
			if len(name) > 27 {
				name = name[:27] + "..."
			}
			if len(projectPaths) > 1 {
				label := tm.Project + "/" + name
				if len(label) > 30 {
					label = label[:27] + "..."
				}
				name = label
			}
			successPct := int(tm.SuccessRate * 100)
			fmt.Printf("%-30s %10s %10s %6d %7d%%\n",
				name,
				formatDuration(tm.TotalRuntime),
				formatDuration(tm.AvgRuntime),
				tm.Runs,
				successPct)
		}

		if len(allMetrics) > displayLimit {
			fmt.Printf("  ... and %d more tasks\n", len(allMetrics)-displayLimit)
		}

		// Project totals
		if len(projectTotals) > 1 {
			fmt.Printf("\nBY PROJECT\n")
			fmt.Printf("==========\n")
			var sortedProjects []string
			for p := range projectTotals {
				sortedProjects = append(sortedProjects, p)
			}
			sort.Slice(sortedProjects, func(i, j int) bool {
				return projectTotals[sortedProjects[i]].runtime > projectTotals[sortedProjects[j]].runtime
			})

			for _, p := range sortedProjects {
				pt := projectTotals[p]
				fmt.Printf("%-40s %10s %6d runs %10s\n",
					p,
					formatDuration(pt.runtime),
					pt.runs,
					formatCost(pt.cost))
			}
		}

		// Failure analysis
		fmt.Printf("\nRUNTIME\n")
		fmt.Printf("=======\n")
		fmt.Printf("Total: %s across %d runs\n", formatDuration(grandTotalRuntime), totalRuns)
		fmt.Printf("Cost:  %s\n", formatCost(totalCost))
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "N/A"
	}
	if d >= time.Hour {
		hours := d.Hours()
		return fmt.Sprintf("%.1fh", hours)
	}
	if d >= time.Minute {
		mins := d.Minutes()
		return fmt.Sprintf("%.1fm", mins)
	}
	secs := d.Seconds()
	return fmt.Sprintf("%.0fs", secs)
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

