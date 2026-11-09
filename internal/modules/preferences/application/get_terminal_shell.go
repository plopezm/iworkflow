package application

import (
	"context"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
)

// GetTerminalShell reads the shell the user chose for the terminals.
type GetTerminalShell struct {
	repository outbound.PreferencesRepository
}

// NewGetTerminalShell builds the use case on top of the preferences repository.
func NewGetTerminalShell(repository outbound.PreferencesRepository) *GetTerminalShell {
	return &GetTerminalShell{repository: repository}
}

// Execute returns the chosen shell; the zero value means the platform default.
func (u *GetTerminalShell) Execute(ctx context.Context) (models.TerminalShell, error) {
	return u.repository.TerminalShell(ctx) //nolint:wrapcheck // storage errors reach the caller unchanged
}
