package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"os/exec"
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
// onStart, if non-nil, is called with the child process PID right after the
// process starts (before waiting for it to complete). This allows callers to
// record the PID while the task is running.
//
// Returns the actual session ID used (either the passed-in sessionID for resume,
// or a freshly generated one) and any error.
func (r *Runner) Run(ctx context.Context, dir string, sessionID string, resume bool, content string, onStart func(pid int)) (usedSessionID string, err error) {
	var lastErr error
	var lastStderr string

	for i, command := range r.Commands {
		actualSessionID := sessionID
		cmdStr := command
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
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			lastErr = err
			lastStderr = err.Error()
			log.Printf("runner[%d] start failed: %v", i, err)
			continue
		}

		// Report child PID to caller while the process is running
		if onStart != nil {
			onStart(cmd.Process.Pid)
		}

		waitErr := cmd.Wait()
		if waitErr != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return "", fmt.Errorf("timed out after %s", r.Timeout)
			}
			lastErr = waitErr
			lastStderr = stderr.String()
			log.Printf("runner[%d] failed: %v", i, waitErr)
			continue
		}

		if out := stdout.String(); out != "" {
			log.Printf("runner[%d] output: %s", i, out)
		}
		log.Printf("runner[%d] succeeded: %s", i, command)
		return actualSessionID, nil
	}

	return "", fmt.Errorf("all runners failed: last exit error: %w\nstderr: %s", lastErr, lastStderr)
}

// cleanEnv returns the current environment with Claude-nesting vars removed.
func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") {
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
