package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// RegenerateAccessToken replaces the access token; every device using the old one must
// authenticate again.
type RegenerateAccessToken struct {
	repository outbound.PreferencesRepository
	generate   TokenGenerator
}

// NewRegenerateAccessToken builds the use case on top of the preferences repository.
func NewRegenerateAccessToken(repository outbound.PreferencesRepository, generate TokenGenerator) *RegenerateAccessToken {
	return &RegenerateAccessToken{repository: repository, generate: generate}
}

// Execute stores a new token and returns it.
func (u *RegenerateAccessToken) Execute(ctx context.Context) (models.AccessToken, error) {
	token, err := u.generate()
	if err != nil {
		return models.AccessToken{}, fmt.Errorf("generate access token: %w", err)
	}
	if err := u.repository.SaveAccessToken(ctx, token); err != nil {
		return models.AccessToken{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	return token, nil
}
