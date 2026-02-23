package daemon

import (
	"fmt"
	"os"
	"time"
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

type daemonLogger struct {
	isTTY bool
}

// dlog is the package-level structured logger for the daemon.
var dlog = newDaemonLogger()

func newDaemonLogger() *daemonLogger {
	fi, err := os.Stdout.Stat()
	tty := err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	return &daemonLogger{isTTY: tty}
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

func (l *daemonLogger) println(line string) {
	fmt.Fprintln(os.Stdout, line)
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
