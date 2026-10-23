package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// SaveLanguage stores the interface language so every browser opens in it.
type SaveLanguage struct {
	repository outbound.PreferencesRepository
}

// NewSaveLanguage builds the use case on top of the preferences repository.
func NewSaveLanguage(repository outbound.PreferencesRepository) *SaveLanguage {
	return &SaveLanguage{repository: repository}
}

// Execute validates the code and stores it. Unsupported codes never reach storage.
func (u *SaveLanguage) Execute(ctx context.Context, code string) (models.UILanguage, error) {
	language, err := models.ParseUILanguage(code)
	if err != nil {
		return models.UILanguage{}, err //nolint:wrapcheck // validation errors reach the caller unchanged
	}
	if err := u.repository.SaveLanguage(ctx, language); err != nil {
		return models.UILanguage{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	return language, nil
}
