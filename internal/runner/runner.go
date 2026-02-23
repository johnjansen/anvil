package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner shells out the configured command with todo content as an argument.
type Runner struct {
	Command string
	Timeout time.Duration
}

// New creates a runner with the given command template and timeout.
func New(command string, timeout time.Duration) *Runner {
	return &Runner{Command: command, Timeout: timeout}
}

// Run executes the configured command with the todo content as an argument.
// The content is shell-escaped and appended to the command string.
// The command runs in the specified directory.
// If sessionID is non-empty, --session-id is injected into the command.
func (r *Runner) Run(ctx context.Context, dir string, sessionID string, content string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	command := r.Command
	if sessionID != "" {
		command += " --session-id " + sessionID
	}

	escaped := shellEscape(content)
	cmd := exec.CommandContext(ctx, "sh", "-c", command+" "+escaped)
	cmd.Dir = dir
	cmd.Env = cleanEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timed out after %s", r.Timeout)
		}
		return stderr.String(), fmt.Errorf("exit error: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
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
// This is the standard POSIX approach: replace ' with '\” and wrap in '...'
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
