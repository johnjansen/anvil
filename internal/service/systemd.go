//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

var unitTemplate = template.Must(template.New("unit").Parse(`[Unit]
Description=Anvil Task Daemon
After=network.target

[Service]
Type=simple
ExecStart={{.BinaryPath}} watch
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`))

type systemdManager struct {
	unitPath string
}

// New returns a systemd-based service manager for Linux.
func New() (Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	return &systemdManager{
		unitPath: filepath.Join(unitDir, "anvil.service"),
	}, nil
}

func (m *systemdManager) Install(binaryPath string) error {
	// Warn if binary looks temporary
	if strings.Contains(binaryPath, "/tmp/") || strings.Contains(binaryPath, "go-build") {
		fmt.Fprintf(os.Stderr, "warning: binary path looks temporary: %s\n", binaryPath)
		fmt.Fprintf(os.Stderr, "  install anvil to a permanent location first (e.g. go install or anvil update)\n")
	}

	// Ensure unit directory exists
	if err := os.MkdirAll(filepath.Dir(m.unitPath), 0755); err != nil {
		return fmt.Errorf("creating systemd user directory: %w", err)
	}

	// Generate unit file
	f, err := os.Create(m.unitPath)
	if err != nil {
		return fmt.Errorf("creating unit file: %w", err)
	}
	defer f.Close()

	data := struct {
		BinaryPath string
	}{
		BinaryPath: binaryPath,
	}
	if err := unitTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}

	// Reload systemd to pick up the new unit
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %s", string(out))
	}

	// Enable and start the service
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", "anvil.service").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable failed: %s", string(out))
	} else {
		_ = out
	}

	return nil
}

func (m *systemdManager) Uninstall() error {
	if _, err := os.Stat(m.unitPath); os.IsNotExist(err) {
		return nil // not installed
	}

	// Disable and stop the service
	_ = exec.Command("systemctl", "--user", "disable", "--now", "anvil.service").Run()

	// Remove the unit file
	if err := os.Remove(m.unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file: %w", err)
	}

	// Reload systemd
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

func (m *systemdManager) Status() (ServiceStatus, error) {
	if _, err := os.Stat(m.unitPath); os.IsNotExist(err) {
		return ServiceStatus{
			Installed: false,
			Running:   false,
			Message:   "not installed",
		}, nil
	}

	// Check if the service is active
	cmd := exec.Command("systemctl", "--user", "is-active", "anvil.service")
	out, err := cmd.Output()
	active := err == nil && strings.TrimSpace(string(out)) == "active"

	msg := "installed, stopped"
	if active {
		msg = "installed, running"
	}

	return ServiceStatus{
		Installed: true,
		Running:   active,
		Message:   msg,
	}, nil
}
