package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// GetListenPort reads the port the user chose for the server, if any.
type GetListenPort struct {
	repository outbound.PreferencesRepository
}

// NewGetListenPort builds the use case on top of the preferences repository.
func NewGetListenPort(repository outbound.PreferencesRepository) *GetListenPort {
	return &GetListenPort{repository: repository}
}

// Execute returns the chosen port, or nil before any choice.
func (u *GetListenPort) Execute(ctx context.Context) (*models.ListenPort, error) {
	return u.repository.ListenPort(ctx) //nolint:wrapcheck // storage errors reach the caller unchanged
}
