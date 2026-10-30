//nolint:wrapcheck // Port errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// DeleteConnection disconnects from Jira: it forgets the token and the imported issues. Converted
// iworkflow tasks stay.
type DeleteConnection struct {
	connections outbound.ConnectionRepository
	issues      outbound.IssueRepository
}

// NewDeleteConnection builds the use case.
func NewDeleteConnection(connections outbound.ConnectionRepository, issues outbound.IssueRepository) *DeleteConnection {
	return &DeleteConnection{connections: connections, issues: issues}
}

// Execute clears the issues first: if that fails the connection is kept.
func (u *DeleteConnection) Execute(ctx context.Context) error {
	if err := u.issues.DeleteAll(ctx); err != nil {
		return err
	}
	return u.connections.Delete(ctx)
}
