package application

import "iworkflow/internal/modules/preferences/domain/ports/outbound"

// RequestServerRestart asks the process to restart so the saved network settings apply. The
// response is sent before the server stops.
type RequestServerRestart struct {
	restarter outbound.ServerRestarter
}

// NewRequestServerRestart builds the use case on the process restart port.
func NewRequestServerRestart(restarter outbound.ServerRestarter) *RequestServerRestart {
	return &RequestServerRestart{restarter: restarter}
}

// Execute requests the restart. It cannot fail: the restart itself is asynchronous.
func (u *RequestServerRestart) Execute() {
	u.restarter.RequestRestart()
}
