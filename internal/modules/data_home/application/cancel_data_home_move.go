package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/data_home/domain/ports/outbound"
)

// CancelDataHomeMove withdraws the move waiting for the next start.
type CancelDataHomeMove struct {
	mover outbound.HomeMoverPort
}

// NewCancelDataHomeMove builds the use case.
func NewCancelDataHomeMove(mover outbound.HomeMoverPort) *CancelDataHomeMove {
	return &CancelDataHomeMove{mover: mover}
}

// Execute removes the pending move. Nothing pending is not an error.
func (u *CancelDataHomeMove) Execute(ctx context.Context) error {
	if err := u.mover.Cancel(ctx); err != nil {
		return fmt.Errorf("cancel the data folder move: %w", err)
	}
	return nil
}
