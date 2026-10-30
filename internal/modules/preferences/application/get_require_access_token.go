package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// GetRequireAccessToken reads whether network access demands the access token (it does unless
// switched off).
type GetRequireAccessToken struct {
	repository outbound.PreferencesRepository
}

// NewGetRequireAccessToken builds the use case on top of the preferences repository.
func NewGetRequireAccessToken(repository outbound.PreferencesRepository) *GetRequireAccessToken {
	return &GetRequireAccessToken{repository: repository}
}

// Execute reports the saved choice.
func (u *GetRequireAccessToken) Execute(ctx context.Context) (bool, error) {
	return u.repository.RequireAccessToken(ctx) //nolint:wrapcheck // storage errors reach the caller unchanged
}
