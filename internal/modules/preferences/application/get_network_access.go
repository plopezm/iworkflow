package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// GetNetworkAccess reads whether the user opted in to serving the app on every network interface.
type GetNetworkAccess struct {
	repository outbound.PreferencesRepository
}

// NewGetNetworkAccess builds the use case on top of the preferences repository.
func NewGetNetworkAccess(repository outbound.PreferencesRepository) *GetNetworkAccess {
	return &GetNetworkAccess{repository: repository}
}

// Execute reports the saved choice; false (private server) until the user enables it.
func (u *GetNetworkAccess) Execute(ctx context.Context) (bool, error) {
	return u.repository.NetworkAccess(ctx) //nolint:wrapcheck // storage errors reach the caller unchanged
}
