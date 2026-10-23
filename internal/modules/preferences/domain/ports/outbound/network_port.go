package outbound

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/models"
)

// NetworkDetector reports the addresses of this machine. Implemented by the operating system
// adapter; the use cases never see sockets or interfaces.
type NetworkDetector interface {
	// Interfaces lists the addresses other devices could use. Empty when none is detected; a failed
	// lookup is logged by the adapter and reported as empty, as the settings card always did.
	Interfaces(ctx context.Context) []models.NetworkInterface
	// PreferredAddress is the address the operating system routes through, when it has one.
	PreferredAddress(ctx context.Context) (string, bool)
}

// ServerRuntime describes the server process that is running.
type ServerRuntime interface {
	Status() models.ServerStatus
}

// ServerRestarter asks the process to restart so saved settings apply.
type ServerRestarter interface {
	// RequestRestart starts the restart. It returns before the server stops.
	RequestRestart()
}
