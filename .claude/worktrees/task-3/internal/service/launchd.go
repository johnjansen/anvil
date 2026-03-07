//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const launchdLabel = "com.anvil.daemon"

var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.BinaryPath}}</string>
		<string>watch</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.LogPath}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogPath}}</string>
	<key>ProcessType</key>
	<string>Background</string>
</dict>
</plist>
`))

type launchdManager struct {
	plistPath string
	logPath   string
}

// New returns a launchd-based service manager for macOS.
func New() (Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	return &launchdManager{
		plistPath: filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"),
		logPath:   filepath.Join(home, ".anvil", "daemon.log"),
	}, nil
}

func (m *launchdManager) Install(binaryPath string) error {
	// Warn if binary looks temporary
	if strings.Contains(binaryPath, "/tmp/") || strings.Contains(binaryPath, "go-build") {
		fmt.Fprintf(os.Stderr, "warning: binary path looks temporary: %s\n", binaryPath)
		fmt.Fprintf(os.Stderr, "  install anvil to a permanent location first (e.g. go install or anvil update)\n")
	}

	// Ensure LaunchAgents directory exists
	if err := os.MkdirAll(filepath.Dir(m.plistPath), 0755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}

	// If service is currently loaded, unload it first for idempotent re-install
	if m.isLoaded() {
		_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), launchdLabel)).Run()
	}

	// Generate plist
	f, err := os.Create(m.plistPath)
	if err != nil {
		return fmt.Errorf("creating plist: %w", err)
	}
	defer f.Close()

	data := struct {
		Label      string
		BinaryPath string
		LogPath    string
	}{
		Label:      launchdLabel,
		BinaryPath: binaryPath,
		LogPath:    m.logPath,
	}
	if err := plistTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	// Load the service using modern launchctl bootstrap
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	cmd := exec.Command("launchctl", "bootstrap", domain, m.plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fall back to legacy load for older macOS
		cmd2 := exec.Command("launchctl", "load", "-w", m.plistPath)
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return fmt.Errorf("launchctl load failed: %s\n%s", string(out), string(out2))
		}
	} else {
		_ = out
	}

	return nil
}

func (m *launchdManager) Uninstall() error {
	if _, err := os.Stat(m.plistPath); os.IsNotExist(err) {
		return nil // not installed, nothing to do
	}

	// Unload the service
	domain := fmt.Sprintf("gui/%d/%s", os.Getuid(), launchdLabel)
	cmd := exec.Command("launchctl", "bootout", domain)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fall back to legacy unload
		cmd2 := exec.Command("launchctl", "unload", m.plistPath)
		_ = cmd2.Run()
	}

	// Remove the plist file
	if err := os.Remove(m.plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist: %w", err)
	}

	return nil
}

func (m *launchdManager) Status() (ServiceStatus, error) {
	if _, err := os.Stat(m.plistPath); os.IsNotExist(err) {
		return ServiceStatus{
			Installed: false,
			Running:   false,
			Message:   "not installed",
		}, nil
	}

	running := m.isLoaded()
	msg := "installed, stopped"
	if running {
		msg = "installed, running"
	}

	return ServiceStatus{
		Installed: true,
		Running:   running,
		Message:   msg,
	}, nil
}

func (m *launchdManager) isLoaded() bool {
	cmd := exec.Command("launchctl", "list", launchdLabel)
	return cmd.Run() == nil
}
