package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// SaveRequireAccessToken stores whether network access demands the access token (applied on the
// next start).
type SaveRequireAccessToken struct {
	repository outbound.PreferencesRepository
}

// NewSaveRequireAccessToken builds the use case on top of the preferences repository.
func NewSaveRequireAccessToken(repository outbound.PreferencesRepository) *SaveRequireAccessToken {
	return &SaveRequireAccessToken{repository: repository}
}

// Execute stores the choice and returns it.
func (u *SaveRequireAccessToken) Execute(ctx context.Context, required bool) (bool, error) {
	if err := u.repository.SaveRequireAccessToken(ctx, required); err != nil {
		return false, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	return required, nil
}
