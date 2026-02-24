package service

// ServiceStatus describes the current state of the system service.
type ServiceStatus struct {
	Installed bool
	Running   bool
	Message   string // human-readable status line
}

// Manager is the platform-agnostic interface for system service management.
type Manager interface {
	// Install generates and loads a system service definition that runs "anvil watch".
	// binaryPath is the absolute path to the anvil binary.
	// Idempotent: overwrites existing service definition if present.
	Install(binaryPath string) error

	// Uninstall stops and removes the system service definition.
	// No-op if the service is not installed.
	Uninstall() error

	// Status returns whether the service is installed and running.
	Status() (ServiceStatus, error)
}
