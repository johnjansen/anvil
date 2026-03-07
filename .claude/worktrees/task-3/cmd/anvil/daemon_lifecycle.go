package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/internal/service"
	"github.com/johnjansen/anvil/tools"
	"golang.org/x/term"
)

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
