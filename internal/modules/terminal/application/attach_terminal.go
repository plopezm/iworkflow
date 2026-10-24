package application

import (
	"context"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/domain/ports/outbound"
)

// AttachTerminal connects to an existing session: recent scrollback plus live output.
type AttachTerminal struct {
	pty outbound.PtyPort
}

// NewAttachTerminal builds the use case.
func NewAttachTerminal(pty outbound.PtyPort) *AttachTerminal {
	return &AttachTerminal{pty: pty}
}

// Execute returns the attachment. The caller must Detach it when it stops reading.
func (u *AttachTerminal) Execute(ctx context.Context, id models.TerminalSessionID) (models.TerminalAttachment, error) {
	attachment, err := u.pty.Attach(ctx, id)
	return attachment, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
