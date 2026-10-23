package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// SaveNetworkAccess stores whether the server listens on every network interface (applied on the
// next start).
type SaveNetworkAccess struct {
	repository outbound.PreferencesRepository
}

// NewSaveNetworkAccess builds the use case on top of the preferences repository.
func NewSaveNetworkAccess(repository outbound.PreferencesRepository) *SaveNetworkAccess {
	return &SaveNetworkAccess{repository: repository}
}

// Execute stores the choice and returns it.
func (u *SaveNetworkAccess) Execute(ctx context.Context, enabled bool) (bool, error) {
	if err := u.repository.SaveNetworkAccess(ctx, enabled); err != nil {
		return false, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	return enabled, nil
}
