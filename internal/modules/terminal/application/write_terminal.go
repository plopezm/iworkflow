package application

import (
	"context"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/domain/ports/outbound"
)

// WriteTerminal sends input (keyboard bytes) to an open terminal session.
type WriteTerminal struct {
	pty outbound.PtyPort
}

// NewWriteTerminal builds the use case.
func NewWriteTerminal(pty outbound.PtyPort) *WriteTerminal {
	return &WriteTerminal{pty: pty}
}

// Execute forwards the bytes to the session's shell.
func (u *WriteTerminal) Execute(ctx context.Context, id models.TerminalSessionID, data []byte) error {
	return u.pty.Write(ctx, id, data) //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
