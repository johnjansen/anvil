package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Runner shells out the configured command with todo content as an argument.
type Runner struct {
	Commands []string
	Timeout  time.Duration
}

// New creates a runner with the given command template and timeout.
func New(commands []string, timeout time.Duration) *Runner {
	return &Runner{Commands: commands, Timeout: timeout}
}

// Run executes the configured commands with the todo content as an argument.
// The content is shell-escaped and appended to the command string.
// The command runs in the specified directory.
// If resume is true, uses --resume to continue a previous session.
// Otherwise, uses --session-id to start a new session.
// Tries each command in order until one succeeds.
//
// skipIndices, if non-nil, is a set of runner indices to skip (e.g., runners
// in cooldown after recent failures). These runners are omitted from the attempt loop.
//
// taskLabel is a human-readable "project/task" string used in log output.
//
// logDir, if non-empty, is the directory where a raw log file will be written
// (stdout+stderr teed in real time). The file is named <runID>.log. Pass empty
// string to skip log file creation (behaviour identical to before).
//
// onStart, if non-nil, is called right after the child process starts
// (before waiting for it to complete). It receives the child PID, the log
// file path (empty if no log was written), and the session ID being used for
// this execution. This allows callers to record metadata while the task runs.
//
// Returns the actual session ID used (either the passed-in sessionID for resume,
// or a freshly generated one), the log file path (empty if no log was written),
// the index of the runner that was used (last attempted, -1 if none), the stderr
// output from the last runner attempt (used for token usage parsing), and any error.
// runnerChainOverride, if non-empty, overrides the default Commands for this execution.
// runnerOnTimeout is tried if the first runner in the chain times out.
func (r *Runner) Run(ctx context.Context, dir string, sessionID string, resume bool, skipPermissions bool, allowedTools []string, content string, taskLabel string, logDir string, skipIndices map[int]bool, extraEnv map[string]string, runnerChainOverride []string, runnerOnTimeout string, onStart func(pid int, logPath string, sessionID string), onStatus func(status string), onCheckpoint func(data string)) (usedSessionID string, logPath string, usedRunnerIndex int, stderrOutput string, err error) {
	var lastErr error
	var lastStderr string
	var lastRunnerIndex int

	// Safety guard: never pass --resume with an empty session ID
	if resume && sessionID == "" {
		resume = false
	}

	// One log file is shared across all runner attempts for this Run() call.
	// It is opened before the first attempt so that output from every attempt
	// is captured, and closed via defer when Run() returns.
	var logFile *os.File
	if logDir != "" {
		if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
			log.Printf("runner [%s] failed to create log dir %s: %v", taskLabel, logDir, mkErr)
		} else {
			// Use a random run ID for the log filename since a stable name is
			// not required at this layer.
			var b [8]byte
			rand.Read(b[:])
			runID := fmt.Sprintf("%x", b[:])
			lp := filepath.Join(logDir, runID+".log")
			f, fErr := os.Create(lp)
			if fErr != nil {
				log.Printf("runner [%s] failed to create log file %s: %v", taskLabel, lp, fErr)
			} else {
				logFile = f
				logPath = lp
				defer logFile.Close()
			}
		}
	}

	// Use task-specific runner chain if provided, otherwise fall back to default
	commands := r.Commands
	if len(runnerChainOverride) > 0 {
		commands = runnerChainOverride
	}

	for i, command := range commands {
		lastRunnerIndex = i
		// Skip runners that are in cooldown
		if skipIndices != nil && skipIndices[i] {
			continue
		}
		actualSessionID := sessionID
		cmdStr := command
		// Append --dangerously-skip-permissions if the task opts in and the
		// runner command does not already include it (backwards compatibility:
		// if the runner includes the flag globally it applies unconditionally).
		if skipPermissions && !strings.Contains(cmdStr, "--dangerously-skip-permissions") {
			cmdStr += " --dangerously-skip-permissions"
		}
		if len(allowedTools) > 0 {
			quoted := make([]string, len(allowedTools))
			for i, t := range allowedTools {
				quoted[i] = shellEscape(t)
			}
			cmdStr += " --allowedTools " + strings.Join(quoted, " ")
		}
		if resume {
			cmdStr += " --resume " + sessionID
		} else {
			// Generate a fresh session ID so we never collide with existing sessions
			var b [16]byte
			rand.Read(b[:])
			freshID := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
			actualSessionID = freshID
			cmdStr += " --session-id " + freshID
		}

		escaped := shellEscape(content)
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr+" "+escaped)
		cmd.Dir = dir
		cmd.Env = cleanEnv()
		// Append task-specific environment variables (overrides any existing values)
		for k, v := range extraEnv {
			cmd.Env = append(cmd.Env, k+"="+v)
		}

		var stdout, stderr bytes.Buffer
		var sw *statusWriter
		var stderrSw *statusWriter
		if logFile != nil {
			stdoutBase := io.MultiWriter(&stdout, logFile)
			sw = newStatusWriter(stdoutBase, onStatus, onCheckpoint)
			cmd.Stdout = sw
			stderrBase := io.MultiWriter(&stderr, logFile)
			stderrSw = newStatusWriter(stderrBase, onStatus, onCheckpoint)
			cmd.Stderr = stderrSw
		} else {
			sw = newStatusWriter(&stdout, onStatus, onCheckpoint)
			cmd.Stdout = sw
			stderrSw = newStatusWriter(&stderr, onStatus, onCheckpoint)
			cmd.Stderr = stderrSw
		}

		if err := cmd.Start(); err != nil {
			lastErr = err
			lastStderr = err.Error()
			log.Printf("runner[%d] [%s] start failed: %v", i, taskLabel, err)
			continue
		}

		// Report child PID, log path, and session ID to caller while the process is running
		if onStart != nil {
			onStart(cmd.Process.Pid, logPath, actualSessionID)
		}

		waitErr := cmd.Wait()
		if sw != nil {
			sw.Flush()
		}
		if stderrSw != nil {
			stderrSw.Flush()
		}
		if waitErr != nil {
			if ctx.Err() == context.DeadlineExceeded {
				// If runner_on_timeout is configured and we haven't tried it yet, try it now
				if runnerOnTimeout != "" && i == 0 {
					log.Printf("runner[%d] [%s] timed out, trying runner_on_timeout: %s", i, taskLabel, runnerOnTimeout)
					// Run the timeout fallback (without context timeout since we're already in timeout)
					timeoutCtx, cancel := context.WithTimeout(context.Background(), r.Timeout)
					defer cancel()
					timeoutResult, timeoutLogPath, timeoutIdx, timeoutStderr, timeoutErr := runSingleRunner(timeoutCtx, dir, sessionID, resume, skipPermissions, allowedTools, content, taskLabel, logFile, runnerOnTimeout, extraEnv)
					if timeoutErr == nil {
						// Timeout fallback succeeded - use index 100+ to indicate fallback
						return timeoutResult, timeoutLogPath, 100 + timeoutIdx, timeoutStderr, nil
					}
					// Timeout fallback also failed, return original timeout error
					return "", logPath, i, stderr.String(), fmt.Errorf("timed out after %s (runner_on_timeout also failed: %v)", r.Timeout, timeoutErr)
				}
				return "", logPath, i, stderr.String(), fmt.Errorf("timed out after %s", r.Timeout)
			}
			lastErr = waitErr
			lastStderr = stderr.String()
			lastRunnerIndex = i
			log.Printf("runner[%d] [%s] failed: %v", i, taskLabel, waitErr)
			continue
		}

		log.Printf("runner[%d] [%s] succeeded: %s", i, taskLabel, command)
		return actualSessionID, logPath, i, stderr.String(), nil
	}

	return "", logPath, lastRunnerIndex, lastStderr, fmt.Errorf("all runners failed: last exit error: %w\nstderr: %s", lastErr, lastStderr)
}

// runSingleRunner executes a single runner command (used for runner_on_timeout fallback).
// It runs the given command with content as an argument.
func runSingleRunner(ctx context.Context, dir string, sessionID string, resume bool, skipPermissions bool, allowedTools []string, content string, taskLabel string, logFile *os.File, cmdStr string, extraEnv map[string]string) (usedSessionID string, logPath string, runnerIndex int, stderrOutput string, err error) {
	actualSessionID := sessionID

	// Append --dangerously-skip-permissions if needed
	if skipPermissions && !strings.Contains(cmdStr, "--dangerously-skip-permissions") {
		cmdStr += " --dangerously-skip-permissions"
	}
	if len(allowedTools) > 0 {
		quoted := make([]string, len(allowedTools))
		for i, t := range allowedTools {
			quoted[i] = shellEscape(t)
		}
		cmdStr += " --allowedTools " + strings.Join(quoted, " ")
	}
	if resume {
		cmdStr += " --resume " + sessionID
	} else {
		var b [16]byte
		rand.Read(b[:])
		freshID := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
		actualSessionID = freshID
		cmdStr += " --session-id " + freshID
	}

	escaped := shellEscape(content)
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr+" "+escaped)
	cmd.Dir = dir
	cmd.Env = cleanEnv()
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	var sw *statusWriter
	var stderrSw *statusWriter
	if logFile != nil {
		stdoutBase := io.MultiWriter(&stdout, logFile)
		sw = newStatusWriter(stdoutBase, nil, nil)
		cmd.Stdout = sw
		stderrBase := io.MultiWriter(&stderr, logFile)
		stderrSw = newStatusWriter(stderrBase, nil, nil)
		cmd.Stderr = stderrSw
	} else {
		sw = newStatusWriter(&stdout, nil, nil)
		cmd.Stdout = sw
		stderrSw = newStatusWriter(&stderr, nil, nil)
		cmd.Stderr = stderrSw
	}

	if err := cmd.Start(); err != nil {
		return "", "", 0, err.Error(), err
	}

	if err := cmd.Wait(); err != nil {
		if sw != nil {
			sw.Flush()
		}
		if stderrSw != nil {
			stderrSw.Flush()
		}
		return "", "", 0, stderr.String(), err
	}

	if sw != nil {
		sw.Flush()
	}
	if stderrSw != nil {
		stderrSw.Flush()
	}
	return actualSessionID, "", 0, stderr.String(), nil
}

// cleanEnv returns the current environment with Claude-nesting guard vars removed.
// Strips all CLAUDE* and ANTHROPIC* prefixed vars to prevent recursive invocation
// detection, but preserves ANTHROPIC_API_KEY and ANTHROPIC_BASE_URL which are
// required for the runner to authenticate with the API.
func cleanEnv() []string {
	keep := map[string]bool{
		"ANTHROPIC_API_KEY":  true,
		"ANTHROPIC_BASE_URL": true,
	}
	var env []string
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if keep[key] {
			env = append(env, e)
			continue
		}
		if strings.HasPrefix(key, "CLAUDE") || strings.HasPrefix(key, "ANTHROPIC") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// statusPrefix is the magic stdout prefix tasks use to report dynamic status.
const statusPrefix = "##anvil:status "

// checkpointPrefix is the magic stdout prefix tasks use to save checkpoint data.
const checkpointPrefix = "##anvil:checkpoint "

// statusWriter wraps an io.Writer and scans for lines with the statusPrefix
// or checkpointPrefix. Status lines call onStatus; checkpoint lines call
// onCheckpoint. Both are stripped from the downstream writer.
type statusWriter struct {
	downstream   io.Writer
	onStatus     func(string)
	onCheckpoint func(string)
	buf          []byte // partial line buffer
}

func newStatusWriter(downstream io.Writer, onStatus func(string), onCheckpoint func(string)) *statusWriter {
	return &statusWriter{downstream: downstream, onStatus: onStatus, onCheckpoint: onCheckpoint}
}

func (sw *statusWriter) Write(p []byte) (int, error) {
	n := len(p)
	sw.buf = append(sw.buf, p...)

	for {
		idx := bytes.IndexByte(sw.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(sw.buf[:idx])
		sw.buf = sw.buf[idx+1:]

		if strings.HasPrefix(line, statusPrefix) {
			status := strings.TrimSpace(line[len(statusPrefix):])
			if status != "" && sw.onStatus != nil {
				sw.onStatus(status)
			}
			// Strip status lines from downstream output
			continue
		}

		if strings.HasPrefix(line, checkpointPrefix) {
			data := strings.TrimSpace(line[len(checkpointPrefix):])
			if data != "" && sw.onCheckpoint != nil {
				sw.onCheckpoint(data)
			}
			// Strip checkpoint lines from downstream output
			continue
		}

		// Pass non-status lines through
		if _, err := sw.downstream.Write([]byte(line + "\n")); err != nil {
			return n, err
		}
	}
	return n, nil
}

// Flush writes any remaining partial line to downstream.
func (sw *statusWriter) Flush() {
	if len(sw.buf) > 0 {
		line := string(sw.buf)
		if strings.HasPrefix(line, statusPrefix) {
			status := strings.TrimSpace(line[len(statusPrefix):])
			if status != "" && sw.onStatus != nil {
				sw.onStatus(status)
			}
		} else if strings.HasPrefix(line, checkpointPrefix) {
			data := strings.TrimSpace(line[len(checkpointPrefix):])
			if data != "" && sw.onCheckpoint != nil {
				sw.onCheckpoint(data)
			}
		} else {
			sw.downstream.Write(sw.buf)
		}
		sw.buf = nil
	}
}

// TokenUsage holds parsed token counts from Claude CLI stderr output.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// ParseTokenUsage extracts token usage from Claude CLI stderr output.
// Claude CLI prints usage stats like:
//
//	Total input tokens: 12345
//	Total output tokens: 6789
//
// or in cost format:
//
//	Input tokens: 12345
//	Output tokens: 6789
//
// Returns zero values if parsing fails (caller should treat as "N/A").
func ParseTokenUsage(stderr string) TokenUsage {
	var usage TokenUsage
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if n, ok := extractTokenCount(line, "input"); ok {
			usage.InputTokens = n
		}
		if n, ok := extractTokenCount(line, "output"); ok {
			usage.OutputTokens = n
		}
	}
	return usage
}

// extractTokenCount tries to extract a token count from a line containing
// the given direction ("input" or "output"). It handles various formats
// that Claude CLI may use to report token counts.
func extractTokenCount(line, direction string) (int, bool) {
	lower := strings.ToLower(line)
	if !strings.Contains(lower, direction) || !strings.Contains(lower, "token") {
		return 0, false
	}
	// Extract the last number on the line (the token count)
	parts := strings.Fields(line)
	for i := len(parts) - 1; i >= 0; i-- {
		// Strip commas from numbers like "12,345"
		cleaned := strings.ReplaceAll(parts[i], ",", "")
		if n, err := strconv.Atoi(cleaned); err == nil && n >= 0 {
			return n, true
		}
	}
	return 0, false
}

// shellEscape wraps content in single quotes with proper escaping.
// This is the standard POSIX approach: replace ' with '\" and wrap in '...'
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
