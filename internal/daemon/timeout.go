package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const defaultWarningWindow = 5 * time.Minute

// extendTimeout extends a running task's timeout by the given duration.
// If absolute is true, the new deadline is set to `duration` from now.
// If absolute is false, the new deadline adds `duration` to the current remaining time.
// Returns the new deadline and remaining time. Must be called with d.tasksMu held.
func extendTimeout(task *RunningTask, duration time.Duration, absolute bool) (newDeadline time.Time, remaining time.Duration, err error) {
	if task.TimeoutTimer == nil {
		return time.Time{}, 0, fmt.Errorf("task has no timeout configured")
	}

	if duration <= 0 {
		return time.Time{}, 0, fmt.Errorf("duration must be positive")
	}

	// Stop the old timer
	task.TimeoutTimer.Stop()

	now := time.Now()
	if absolute {
		newDeadline = now.Add(duration)
	} else {
		currentRemaining := time.Until(task.CurrentDeadline)
		if currentRemaining < 0 {
			currentRemaining = 0
		}
		newDeadline = now.Add(currentRemaining + duration)
	}

	remaining = time.Until(newDeadline)

	// Create new timer
	task.TimeoutTimer = time.AfterFunc(remaining, func() {
		task.Cancel()
	})

	task.CurrentDeadline = newDeadline
	task.ExtensionCount++
	task.TotalExtended += duration
	task.WarningFired = false // reset warning for new deadline

	return newDeadline, remaining, nil
}

// setupWarningTimer creates a timer that fires the on_timeout_warning hook
// when the task enters the warning window (default 5 minutes before deadline).
// Must be called with d.tasksMu held. The timer executes the hook asynchronously.
func (d *Daemon) setupWarningTimer(task *RunningTask, hookCmd, projectPath string) {
	if hookCmd == "" {
		return
	}

	// Cancel existing warning timer if any
	if task.WarningTimer != nil {
		task.WarningTimer.Stop()
	}

	remaining := time.Until(task.CurrentDeadline)
	warningBefore := defaultWarningWindow
	if warningBefore >= remaining {
		// Deadline is already within warning window — fire immediately
		warningBefore = 0
	}

	fireAt := remaining - warningBefore
	if fireAt < 0 {
		fireAt = 0
	}

	task.WarningFired = false
	task.WarningTimer = time.AfterFunc(fireAt, func() {
		d.tasksMu.Lock()
		task.WarningFired = true
		d.tasksMu.Unlock()
		go d.runTimeoutWarningHook(hookCmd, task.Name, projectPath, task)
	})
}

// runTimeoutWarningHook executes the on_timeout_warning hook as a shell command
// with environment variables for timeout state.
func (d *Daemon) runTimeoutWarningHook(command, taskName, projectPath string, task *RunningTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	hookCmd := exec.CommandContext(ctx, "sh", "-c", command)
	hookCmd.Dir = projectPath

	d.tasksMu.RLock()
	remaining := time.Until(task.CurrentDeadline).Round(time.Second)
	originalTimeout := task.OriginalTimeout
	extensionCount := task.ExtensionCount
	autoExtRemaining := 0
	if task.AutoExtendConfig.Enabled {
		max := task.AutoExtendConfig.MaxExtensions
		if max <= 0 {
			max = 3
		}
		autoExtRemaining = max - task.AutoExtensions
		if autoExtRemaining < 0 {
			autoExtRemaining = 0
		}
	}
	d.tasksMu.RUnlock()

	hookCmd.Env = append(os.Environ(),
		"ANVIL_TASK_NAME="+taskName,
		"ANVIL_PROJECT="+projectPath,
		"ANVIL_TIMEOUT_REMAINING="+remaining.String(),
		"ANVIL_TIMEOUT_ORIGINAL="+originalTimeout.String(),
		fmt.Sprintf("ANVIL_EXTENSIONS_USED=%d", extensionCount),
		fmt.Sprintf("ANVIL_AUTO_EXTEND_REMAINING=%d", autoExtRemaining),
	)

	if err := hookCmd.Run(); err != nil {
		dlog.Warn("on_timeout_warning hook failed for %s: %v", taskName, err)
	} else {
		dlog.Info("on_timeout_warning hook completed for %s", taskName)
	}
}
