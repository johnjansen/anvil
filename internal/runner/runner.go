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
// and any error.
func (r *Runner) Run(ctx context.Context, dir string, sessionID string, resume bool, skipPermissions bool, allowedTools []string, content string, taskLabel string, logDir string, onStart func(pid int, logPath string, sessionID string)) (usedSessionID string, logPath string, err error) {
	var lastErr error
	var lastStderr string

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

	for i, command := range r.Commands {
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

		var stdout, stderr bytes.Buffer
		if logFile != nil {
			cmd.Stdout = io.MultiWriter(&stdout, logFile)
			cmd.Stderr = io.MultiWriter(&stderr, logFile)
		} else {
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
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
		if waitErr != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return "", logPath, fmt.Errorf("timed out after %s", r.Timeout)
			}
			lastErr = waitErr
			lastStderr = stderr.String()
			log.Printf("runner[%d] [%s] failed: %v", i, taskLabel, waitErr)
			continue
		}

		log.Printf("runner[%d] [%s] succeeded: %s", i, taskLabel, command)
		return actualSessionID, logPath, nil
	}

	return "", logPath, fmt.Errorf("all runners failed: last exit error: %w\nstderr: %s", lastErr, lastStderr)
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

// shellEscape wraps content in single quotes with proper escaping.
// This is the standard POSIX approach: replace ' with '\" and wrap in '...'
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
