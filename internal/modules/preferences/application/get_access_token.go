package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// TokenGenerator draws a fresh access token. Injected so tests control the randomness and the
// failure path; the composition root passes models.GenerateAccessToken.
type TokenGenerator func() (models.AccessToken, error)

// GetAccessToken returns the access token, generating and storing one the first time it is needed.
type GetAccessToken struct {
	repository outbound.PreferencesRepository
	generate   TokenGenerator
}

// NewGetAccessToken builds the use case on top of the preferences repository.
func NewGetAccessToken(repository outbound.PreferencesRepository, generate TokenGenerator) *GetAccessToken {
	return &GetAccessToken{repository: repository, generate: generate}
}

// Execute returns the stored token or creates one. A token that failed to persist is never
// returned.
func (u *GetAccessToken) Execute(ctx context.Context) (models.AccessToken, error) {
	stored, err := u.repository.AccessToken(ctx)
	if err != nil {
		return models.AccessToken{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	if stored != nil {
		return *stored, nil
	}
	token, err := u.generate()
	if err != nil {
		return models.AccessToken{}, fmt.Errorf("generate access token: %w", err)
	}
	if err := u.repository.SaveAccessToken(ctx, token); err != nil {
		return models.AccessToken{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	return token, nil
}
