package daemon

import (
	"testing"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

func TestExtendTimeout_Additive(t *testing.T) {
	cancel := func() {}
	deadline := time.Now().Add(10 * time.Minute)
	task := &RunningTask{
		Cancel:          cancel,
		CurrentDeadline: deadline,
		TimeoutTimer:    time.AfterFunc(10*time.Minute, func() {}),
	}

	newDeadline, remaining, err := extendTimeout(task, 5*time.Minute, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// New deadline should be approximately 15 minutes from now (10 remaining + 5 extension)
	expectedMin := time.Now().Add(14 * time.Minute)
	expectedMax := time.Now().Add(16 * time.Minute)
	if newDeadline.Before(expectedMin) || newDeadline.After(expectedMax) {
		t.Errorf("newDeadline = %v, expected between %v and %v", newDeadline, expectedMin, expectedMax)
	}
	if remaining < 14*time.Minute || remaining > 16*time.Minute {
		t.Errorf("remaining = %v, expected ~15m", remaining)
	}
	if task.ExtensionCount != 1 {
		t.Errorf("ExtensionCount = %d, want 1", task.ExtensionCount)
	}
	if task.TotalExtended != 5*time.Minute {
		t.Errorf("TotalExtended = %v, want 5m", task.TotalExtended)
	}
	task.TimeoutTimer.Stop()
}

func TestExtendTimeout_Absolute(t *testing.T) {
	cancel := func() {}
	deadline := time.Now().Add(10 * time.Minute)
	task := &RunningTask{
		Cancel:          cancel,
		CurrentDeadline: deadline,
		TimeoutTimer:    time.AfterFunc(10*time.Minute, func() {}),
	}

	newDeadline, remaining, err := extendTimeout(task, 30*time.Minute, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// New deadline should be approximately 30 minutes from now (absolute)
	expectedMin := time.Now().Add(29 * time.Minute)
	expectedMax := time.Now().Add(31 * time.Minute)
	if newDeadline.Before(expectedMin) || newDeadline.After(expectedMax) {
		t.Errorf("newDeadline = %v, expected between %v and %v", newDeadline, expectedMin, expectedMax)
	}
	if remaining < 29*time.Minute || remaining > 31*time.Minute {
		t.Errorf("remaining = %v, expected ~30m", remaining)
	}
	task.TimeoutTimer.Stop()
}

func TestExtendTimeout_NoTimer(t *testing.T) {
	task := &RunningTask{
		Cancel: func() {},
	}

	_, _, err := extendTimeout(task, 5*time.Minute, false)
	if err == nil {
		t.Fatal("expected error for task with no timeout timer")
	}
}

func TestExtendTimeout_NegativeDuration(t *testing.T) {
	task := &RunningTask{
		Cancel:          func() {},
		CurrentDeadline: time.Now().Add(10 * time.Minute),
		TimeoutTimer:    time.AfterFunc(10*time.Minute, func() {}),
	}

	_, _, err := extendTimeout(task, -5*time.Minute, false)
	if err == nil {
		t.Fatal("expected error for negative duration")
	}
	task.TimeoutTimer.Stop()
}

func TestExtendTimeout_MultipleExtensions(t *testing.T) {
	cancel := func() {}
	deadline := time.Now().Add(10 * time.Minute)
	task := &RunningTask{
		Cancel:          cancel,
		CurrentDeadline: deadline,
		TimeoutTimer:    time.AfterFunc(10*time.Minute, func() {}),
	}

	// First extension
	_, _, err := extendTimeout(task, 5*time.Minute, false)
	if err != nil {
		t.Fatalf("extension 1 failed: %v", err)
	}

	// Second extension
	_, _, err = extendTimeout(task, 10*time.Minute, false)
	if err != nil {
		t.Fatalf("extension 2 failed: %v", err)
	}

	if task.ExtensionCount != 2 {
		t.Errorf("ExtensionCount = %d, want 2", task.ExtensionCount)
	}
	if task.TotalExtended != 15*time.Minute {
		t.Errorf("TotalExtended = %v, want 15m", task.TotalExtended)
	}
	if task.WarningFired {
		t.Error("WarningFired should be false after extension")
	}
	task.TimeoutTimer.Stop()
}

func TestAutoExtend_WithinWarningWindow(t *testing.T) {
	// Task with 3 minutes remaining (within 5-minute warning window)
	cancel := func() {}
	deadline := time.Now().Add(3 * time.Minute)
	task := &RunningTask{
		Cancel:          cancel,
		CurrentDeadline: deadline,
		TimeoutTimer:    time.AfterFunc(3*time.Minute, func() {}),
		AutoExtendConfig: project.AutoExtendConfig{
			Enabled:           true,
			MaxExtensions:     3,
			ExtensionDuration: 15 * time.Minute,
		},
	}

	// Simulate checkpoint arriving within warning window
	warningWindow := 5 * time.Minute
	timeUntilDeadline := task.CurrentDeadline.Sub(time.Now())
	if timeUntilDeadline <= warningWindow && timeUntilDeadline > 0 {
		if task.AutoExtensions < task.AutoExtendConfig.MaxExtensions {
			_, _, err := extendTimeout(task, task.AutoExtendConfig.ExtensionDuration, false)
			if err != nil {
				t.Fatalf("auto-extend failed: %v", err)
			}
			task.AutoExtensions++
		}
	}

	if task.AutoExtensions != 1 {
		t.Errorf("AutoExtensions = %d, want 1", task.AutoExtensions)
	}
	if task.ExtensionCount != 1 {
		t.Errorf("ExtensionCount = %d, want 1", task.ExtensionCount)
	}
	task.TimeoutTimer.Stop()
}

func TestAutoExtend_OutsideWarningWindow(t *testing.T) {
	// Task with 10 minutes remaining (outside 5-minute warning window)
	cancel := func() {}
	deadline := time.Now().Add(10 * time.Minute)
	task := &RunningTask{
		Cancel:          cancel,
		CurrentDeadline: deadline,
		TimeoutTimer:    time.AfterFunc(10*time.Minute, func() {}),
		AutoExtendConfig: project.AutoExtendConfig{
			Enabled:           true,
			MaxExtensions:     3,
			ExtensionDuration: 15 * time.Minute,
		},
	}

	// Checkpoint arrives but outside warning window - should NOT extend
	warningWindow := 5 * time.Minute
	timeUntilDeadline := task.CurrentDeadline.Sub(time.Now())
	if timeUntilDeadline <= warningWindow && timeUntilDeadline > 0 {
		if task.AutoExtensions < task.AutoExtendConfig.MaxExtensions {
			extendTimeout(task, task.AutoExtendConfig.ExtensionDuration, false)
			task.AutoExtensions++
		}
	}

	if task.AutoExtensions != 0 {
		t.Errorf("AutoExtensions = %d, want 0 (outside warning window)", task.AutoExtensions)
	}
	task.TimeoutTimer.Stop()
}

func TestAutoExtend_MaxExtensionsReached(t *testing.T) {
	cancel := func() {}
	deadline := time.Now().Add(3 * time.Minute)
	task := &RunningTask{
		Cancel:          cancel,
		CurrentDeadline: deadline,
		TimeoutTimer:    time.AfterFunc(3*time.Minute, func() {}),
		AutoExtendConfig: project.AutoExtendConfig{
			Enabled:           true,
			MaxExtensions:     2,
			ExtensionDuration: 15 * time.Minute,
		},
		AutoExtensions: 2, // already at max
	}

	warningWindow := 5 * time.Minute
	timeUntilDeadline := task.CurrentDeadline.Sub(time.Now())
	extended := false
	if timeUntilDeadline <= warningWindow && timeUntilDeadline > 0 {
		if task.AutoExtensions < task.AutoExtendConfig.MaxExtensions {
			extendTimeout(task, task.AutoExtendConfig.ExtensionDuration, false)
			task.AutoExtensions++
			extended = true
		}
	}

	if extended {
		t.Error("should NOT have extended — max extensions reached")
	}
	if task.AutoExtensions != 2 {
		t.Errorf("AutoExtensions = %d, want 2", task.AutoExtensions)
	}
	task.TimeoutTimer.Stop()
}
