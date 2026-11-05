// Package outbound holds the ports the data_home use cases need from infrastructure.
package outbound

import (
	"context"

	"iworkflow/internal/modules/data_home/domain/models"
)

// HomeMoverPort reads and changes the data folder through the move engine.
type HomeMoverPort interface {
	// Status returns the folder in use, its source and the pending and last moves.
	Status(ctx context.Context) (models.HomeStatus, error)
	// Request records a move of the current folder to destination for the next start. It validates the
	// destination and returns a *DestinationError (or ErrMovePending) when the move is refused.
	Request(ctx context.Context, destination string) error
	// Cancel withdraws the pending move. Cancelling when nothing is pending is not an error.
	Cancel(ctx context.Context) error
}

// ActiveTasksPort tells whether a task is starting, running or finishing: a move waits for them.
type ActiveTasksPort interface {
	HasActive(ctx context.Context) (bool, error)
}

// ServerRestarterPort asks the process to restart so the pending move runs at the next start.
type ServerRestarterPort interface {
	// RequestRestart starts the restart. It returns before the server stops.
	RequestRestart()
}
