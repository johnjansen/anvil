//go:build !darwin && !linux

package service

import "fmt"

type stubManager struct{}

// New returns an error on unsupported platforms.
func New() (Manager, error) {
	return &stubManager{}, nil
}

func (m *stubManager) Install(binaryPath string) error {
	return fmt.Errorf("system service installation is not supported on this platform; supported: macOS (launchd), Linux (systemd)")
}

func (m *stubManager) Uninstall() error {
	return fmt.Errorf("system service management is not supported on this platform; supported: macOS (launchd), Linux (systemd)")
}

func (m *stubManager) Status() (ServiceStatus, error) {
	return ServiceStatus{Message: "not supported on this platform"}, nil
}
