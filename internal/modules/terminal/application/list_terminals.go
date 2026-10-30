package application

import (
	"context"
	"slices"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/domain/ports/outbound"
)

// ListTerminals lists the open terminal sessions, newest first.
type ListTerminals struct {
	pty outbound.PtyPort
}

// NewListTerminals builds the use case.
func NewListTerminals(pty outbound.PtyPort) *ListTerminals {
	return &ListTerminals{pty: pty}
}

// Execute returns the sessions sorted by creation time, most recent first.
func (u *ListTerminals) Execute(ctx context.Context) ([]models.TerminalSession, error) {
	sessions, err := u.pty.List(ctx)
	if err != nil {
		return nil, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}
	slices.SortStableFunc(sessions, func(a models.TerminalSession, b models.TerminalSession) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	return sessions, nil
}
