package system

import "iworkflow/internal/core/restart"

// ServerRestarter asks the process to restart through the shared restart handle, which main owns.
type ServerRestarter struct {
	handle *restart.Handle
}

// NewServerRestarter wraps the restart handle created at start-up.
func NewServerRestarter(handle *restart.Handle) *ServerRestarter {
	return &ServerRestarter{handle: handle}
}

// RequestRestart asks for a restart. Safe to call more than once.
func (s *ServerRestarter) RequestRestart() {
	s.handle.Request()
}
