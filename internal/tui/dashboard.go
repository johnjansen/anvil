package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
)

// StartDashboard starts the TUI dashboard
func StartDashboard() error {
	if !daemon.IsDaemonRunning() {
		return fmt.Errorf("daemon not running")
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

	// Create and start the Bubble Tea program
	p := tea.NewProgram(NewModel(daemonPID, maxWorkers))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running dashboard: %v", err)
	}

	return nil
}