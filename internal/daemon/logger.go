package daemon

import (
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/johnjansen/anvil/internal/config"
)

// ANSI escape sequences
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiWhite   = "\033[37m"
)

// workerColors cycles through a palette by worker ID
var workerColors = []string{ansiCyan, ansiYellow, ansiGreen, ansiMagenta, ansiBlue}

// priorityColors maps priority level to ANSI code; p3+ uses empty (default)
var priorityColors = []string{ansiRed, ansiYellow, ansiCyan, ""}

// ansiPattern matches ANSI escape sequences for stripping from log file output.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// maxLogSize is the size threshold at which the daemon log is rotated.
const maxLogSize = 1 << 20 // 1 MB

type daemonLogger struct {
	isTTY   bool
	rawMode bool // when true, println writes \r\n instead of \n for raw terminal mode

	mu      sync.Mutex
	logFile *os.File
	logSize int64
}

// dlog is the package-level structured logger for the daemon.
var dlog = newDaemonLogger()

func newDaemonLogger() *daemonLogger {
	fi, err := os.Stdout.Stat()
	tty := err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	l := &daemonLogger{isTTY: tty}
	l.openLogFile()
	return l
}

// openLogFile opens (or re-opens) the daemon log file for appending.
// It skips opening if stdout is already pointing at daemon.log (daemonized mode),
// since the content would be duplicated.
func (l *daemonLogger) openLogFile() {
	if err := config.EnsureDir(); err != nil {
		return
	}
	path := config.DaemonLogPath()

	// When daemonized, stdout is redirected to daemon.log by the parent process.
	// Detect this to avoid writing each line twice.
	if stdoutInfo, err := os.Stdout.Stat(); err == nil {
		if logInfo, err := os.Stat(path); err == nil {
			if os.SameFile(stdoutInfo, logInfo) {
				return
			}
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return
	}
	l.logFile = f
	l.logSize = info.Size()
}

// rotateIfNeeded rotates the log file when it exceeds maxLogSize.
// Caller must hold l.mu.
func (l *daemonLogger) rotateIfNeeded() {
	if l.logFile == nil || l.logSize < maxLogSize {
		return
	}
	l.logFile.Close()
	path := config.DaemonLogPath()
	_ = os.Rename(path, path+".1")
	l.logFile = nil
	l.logSize = 0
	l.openLogFile()
}

// c wraps text in an ANSI color code, stripping it when not a TTY.
func (l *daemonLogger) c(code, text string) string {
	if !l.isTTY || code == "" {
		return text
	}
	return code + text + ansiReset
}

func (l *daemonLogger) priorityStr(p int) string {
	s := fmt.Sprintf("p%d", p)
	idx := p
	if idx >= len(priorityColors) {
		idx = len(priorityColors) - 1
	}
	return l.c(priorityColors[idx], s)
}

func (l *daemonLogger) workerStr(id int) string {
	code := workerColors[id%len(workerColors)]
	return l.c(code, fmt.Sprintf("worker[%d]", id))
}

func (l *daemonLogger) schedulerStr() string {
	return l.c(ansiBold+ansiWhite, "[scheduler]")
}

func (l *daemonLogger) ts() string {
	return time.Now().Format("15:04:05")
}

// SetRawMode toggles raw terminal mode output. When enabled, println writes
// \r\n instead of bare \n so that lines start at column 0 when OPOST is disabled.
func SetRawMode(enabled bool) {
	dlog.mu.Lock()
	dlog.rawMode = enabled
	dlog.mu.Unlock()
}

func (l *daemonLogger) println(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Serialize stdout writes to prevent interleaved output from concurrent workers.
	// In raw terminal mode, \n alone only moves the cursor down without returning
	// to column 0, so we must write \r\n explicitly.
	if l.rawMode {
		fmt.Fprint(os.Stdout, line+"\r\n")
	} else {
		fmt.Fprintln(os.Stdout, line)
	}

	// Write a plain (no ANSI) copy to the log file.
	if l.logFile != nil {
		plain := ansiPattern.ReplaceAllString(line, "") + "\n"
		n, _ := l.logFile.WriteString(plain)
		l.logSize += int64(n)
		l.rotateIfNeeded()
	}
}

// --- Daemon lifecycle ---

func (l *daemonLogger) Startup(tick, runner string, workers int) {
	l.println(fmt.Sprintf("%s %s  daemon started (tick=%s, runner=%q, workers=%d)",
		l.ts(), l.schedulerStr(), tick, runner, workers))
}

func (l *daemonLogger) Stopping() {
	l.println(fmt.Sprintf("%s %s  daemon stopping", l.ts(), l.schedulerStr()))
}

func (l *daemonLogger) SocketListening(path string) {
	l.println(fmt.Sprintf("%s %s  socket listening on %s", l.ts(), l.schedulerStr(), path))
}

func (l *daemonLogger) SocketError(err error) {
	l.println(fmt.Sprintf("%s %s  socket error: %v", l.ts(), l.c(ansiRed, "[error]"), err))
}

func (l *daemonLogger) SocketStartFailed(err error) {
	l.println(fmt.Sprintf("%s %s  failed to start socket server: %v", l.ts(), l.c(ansiRed, "[error]"), err))
}

// --- Worker lifecycle ---

func (l *daemonLogger) WorkerStarted(id int) {
	l.println(fmt.Sprintf("%s %s  started", l.ts(), l.workerStr(id)))
}

func (l *daemonLogger) WorkerStopped(id int) {
	l.println(fmt.Sprintf("%s %s  stopped", l.ts(), l.workerStr(id)))
}

func (l *daemonLogger) WorkerPickup(id int, projName, name string, priority int) {
	label := projName + "/" + name
	l.println(fmt.Sprintf("%s %s %s  picked up %s (%s)",
		l.ts(), l.workerStr(id), l.c(ansiBlue, "▶"), l.c(ansiBold, label), l.priorityStr(priority)))
}

func (l *daemonLogger) WorkerDone(id int, projName, name string, elapsed time.Duration) {
	label := projName + "/" + name
	l.println(fmt.Sprintf("%s %s %s  done %s (%s)",
		l.ts(), l.workerStr(id), l.c(ansiGreen, "✓"), l.c(ansiBold, label),
		l.c(ansiDim, elapsed.Round(time.Second).String())))
}

func (l *daemonLogger) WorkerFail(id int, projName, name string, err error) {
	label := projName + "/" + name
	l.println(fmt.Sprintf("%s %s %s  fail %s: %v",
		l.ts(), l.workerStr(id), l.c(ansiRed, "✗"), l.c(ansiBold, label), err))
}

func (l *daemonLogger) WorkerIdle(id int) {
	l.println(fmt.Sprintf("%s %s %s  idle",
		l.ts(), l.workerStr(id), l.c(ansiDim, "—")))
}

// --- Scheduler/tick ---

func (l *daemonLogger) Dispatch(projName, name string, priority int, schedule string) {
	l.println(fmt.Sprintf("%s %s  dispatch %s/%s (%s, schedule=%q)",
		l.ts(), l.schedulerStr(), projName, l.c(ansiBold, name), l.priorityStr(priority), schedule))
}

func (l *daemonLogger) TickSummary(t time.Time, projects, tasks, matched, dispatched int) {
	l.println(fmt.Sprintf("%s %s  tick %s — %d projects, %d tasks, %d matched, %d dispatched",
		l.ts(), l.schedulerStr(), t.Format("15:04:05"), projects, tasks, matched, dispatched))
}

func (l *daemonLogger) TickIdle(t time.Time) {
	l.println(fmt.Sprintf("%s %s  tick %s — %s",
		l.ts(), l.schedulerStr(), t.Format("15:04:05"), l.c(ansiDim, "idle")))
}

func (l *daemonLogger) TickRunning(t time.Time, tasks string) {
	l.println(fmt.Sprintf("%s %s  tick %s — running: %s",
		l.ts(), l.schedulerStr(), t.Format("15:04:05"), tasks))
}

func (l *daemonLogger) TickNoProjects(t time.Time) {
	l.println(fmt.Sprintf("%s %s  tick %s — no watched projects",
		l.ts(), l.schedulerStr(), t.Format("15:04:05")))
}

// --- Warnings and misc ---

func (l *daemonLogger) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.println(fmt.Sprintf("%s %s  %s", l.ts(), l.c(ansiYellow, "[warn]"), msg))
}

func (l *daemonLogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.println(fmt.Sprintf("%s %s  %s", l.ts(), l.schedulerStr(), msg))
}

func (l *daemonLogger) Fatal(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.println(fmt.Sprintf("%s %s  %s", l.ts(), l.c(ansiRed, "[fatal]"), msg))
	os.Exit(1)
}
