package application

import (
	"context"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/domain/ports/outbound"
)

// ResizeTerminal changes the size of a terminal session after validating the dimensions.
type ResizeTerminal struct {
	pty outbound.PtyPort
}

// NewResizeTerminal builds the use case.
func NewResizeTerminal(pty outbound.PtyPort) *ResizeTerminal {
	return &ResizeTerminal{pty: pty}
}

// Execute rejects zero dimensions before reaching the PTY.
func (u *ResizeTerminal) Execute(ctx context.Context, id models.TerminalSessionID, cols uint16, rows uint16) error {
	size, err := models.NewTerminalSize(cols, rows)
	if err != nil {
		return err //nolint:wrapcheck // the domain validation error is returned unchanged
	}
	return u.pty.Resize(ctx, id, size) //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
