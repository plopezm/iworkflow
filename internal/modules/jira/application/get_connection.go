//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// GetConnection reads the stored connection; nil means there is none.
type GetConnection struct {
	connections outbound.ConnectionRepository
}

// NewGetConnection builds the use case.
func NewGetConnection(connections outbound.ConnectionRepository) *GetConnection {
	return &GetConnection{connections: connections}
}

// Execute returns the connection, or nil when Jira is not configured.
func (u *GetConnection) Execute(ctx context.Context) (*models.Connection, error) {
	return u.connections.Get(ctx)
}

// requireConnection loads the connection or fails with NotConfigured. Every use case that talks to
// Jira starts with it.
func requireConnection(ctx context.Context, connections outbound.ConnectionRepository) (models.Connection, error) {
	connection, err := connections.Get(ctx)
	if err != nil {
		return models.Connection{}, err
	}
	if connection == nil {
		return models.Connection{}, jiraerrors.NotConfigured()
	}
	return *connection, nil
}
