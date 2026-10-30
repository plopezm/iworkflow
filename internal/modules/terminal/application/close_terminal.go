package application

import (
	"context"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/domain/ports/outbound"
)

// CloseTerminal closes a terminal session and releases its resources.
type CloseTerminal struct {
	pty outbound.PtyPort
}

// NewCloseTerminal builds the use case.
func NewCloseTerminal(pty outbound.PtyPort) *CloseTerminal {
	return &CloseTerminal{pty: pty}
}

// Execute kills the session's processes and removes it from the registry.
func (u *CloseTerminal) Execute(ctx context.Context, id models.TerminalSessionID) error {
	return u.pty.Close(ctx, id) //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
