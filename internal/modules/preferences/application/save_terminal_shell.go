package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// SaveTerminalShell validates and stores the shell of the terminals. Blank input restores the default.
type SaveTerminalShell struct {
	repository outbound.PreferencesRepository
}

// NewSaveTerminalShell builds the use case on top of the preferences repository.
func NewSaveTerminalShell(repository outbound.PreferencesRepository) *SaveTerminalShell {
	return &SaveTerminalShell{repository: repository}
}

// Execute validates the path and stores it. Invalid input never reaches storage.
func (u *SaveTerminalShell) Execute(ctx context.Context, raw string) (models.TerminalShell, error) {
	shell, err := models.ParseTerminalShell(raw)
	if err != nil {
		return models.TerminalShell{}, err //nolint:wrapcheck // validation errors reach the caller unchanged
	}
	if err := u.repository.SaveTerminalShell(ctx, shell); err != nil {
		return models.TerminalShell{}, err //nolint:wrapcheck // storage errors reach the caller unchanged
	}
	return shell, nil
}
