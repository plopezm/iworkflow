package application

import (
	"context"
	"fmt"

	domainerrors "iworkflow/internal/modules/data_home/domain/errors"
	"iworkflow/internal/modules/data_home/domain/ports/outbound"
)

// MoveInput is the destination the user typed.
type MoveInput struct {
	Destination string
}

// RequestDataHomeMove records a move of the data folder for the next start and restarts the server so
// the move runs. The checks run in this order: a fixed folder, then active tasks, then the move engine
// (which validates the destination). The restart is requested only after the move is recorded.
type RequestDataHomeMove struct {
	mover     outbound.HomeMoverPort
	tasks     outbound.ActiveTasksPort
	restarter outbound.ServerRestarterPort
}

// NewRequestDataHomeMove builds the use case.
func NewRequestDataHomeMove(mover outbound.HomeMoverPort, tasks outbound.ActiveTasksPort, restarter outbound.ServerRestarterPort) *RequestDataHomeMove {
	return &RequestDataHomeMove{mover: mover, tasks: tasks, restarter: restarter}
}

// Execute checks the request, records the move and asks for a restart.
func (u *RequestDataHomeMove) Execute(ctx context.Context, input MoveInput) error {
	status, err := u.mover.Status(ctx)
	if err != nil {
		return fmt.Errorf("read the data folder: %w", err)
	}
	if status.EnvLocked {
		return domainerrors.ErrEnvLocked
	}
	active, err := u.tasks.HasActive(ctx)
	if err != nil {
		return fmt.Errorf("check the active tasks: %w", err)
	}
	if active {
		return domainerrors.ErrActiveTasks
	}
	if err := u.mover.Request(ctx, input.Destination); err != nil {
		return fmt.Errorf("request the data folder move: %w", err)
	}
	u.restarter.RequestRestart()
	return nil
}
