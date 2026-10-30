package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// SaveListenPort validates and stores the server port (applied on the next start).
type SaveListenPort struct {
	repository outbound.PreferencesRepository
}

// NewSaveListenPort builds the use case on top of the preferences repository.
func NewSaveListenPort(repository outbound.PreferencesRepository) *SaveListenPort {
	return &SaveListenPort{repository: repository}
}

// Execute validates the port and stores it. Privileged ports never reach storage.
func (u *SaveListenPort) Execute(ctx context.Context, port uint16) (models.ListenPort, error) {
	listen, err := models.NewListenPort(port)
	if err != nil {
		return models.ListenPort{}, err //nolint:wrapcheck // validation errors reach the caller unchanged
	}
	if err := u.repository.SaveListenPort(ctx, listen); err != nil {
		return models.ListenPort{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	return listen, nil
}
