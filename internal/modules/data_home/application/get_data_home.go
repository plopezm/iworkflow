// Package application holds the use cases of the data_home module. Each one depends on the outbound
// ports only; the composition root binds the adapters.
package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/data_home/domain/models"
	"iworkflow/internal/modules/data_home/domain/ports/outbound"
)

// GetDataHome reads the data folder the settings screen shows.
type GetDataHome struct {
	mover outbound.HomeMoverPort
}

// NewGetDataHome builds the use case.
func NewGetDataHome(mover outbound.HomeMoverPort) *GetDataHome {
	return &GetDataHome{mover: mover}
}

// Execute returns the folder in use, its source and the pending and last moves.
func (u *GetDataHome) Execute(ctx context.Context) (models.HomeStatus, error) {
	status, err := u.mover.Status(ctx)
	if err != nil {
		return models.HomeStatus{}, fmt.Errorf("read the data folder: %w", err)
	}
	return status, nil
}
