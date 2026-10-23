package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// GetLanguage reads the interface language the user picked, if any.
type GetLanguage struct {
	repository outbound.PreferencesRepository
}

// NewGetLanguage builds the use case on top of the preferences repository.
func NewGetLanguage(repository outbound.PreferencesRepository) *GetLanguage {
	return &GetLanguage{repository: repository}
}

// Execute returns the chosen language, or nil before any choice.
func (u *GetLanguage) Execute(ctx context.Context) (*models.UILanguage, error) {
	return u.repository.Language(ctx) //nolint:wrapcheck // storage errors reach the caller unchanged
}
