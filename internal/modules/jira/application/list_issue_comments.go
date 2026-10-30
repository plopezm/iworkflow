//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// ListIssueComments reads the comments of an issue. They are read live from Jira; they are not imported.
type ListIssueComments struct {
	connections outbound.ConnectionRepository
	client      outbound.JiraClient
}

// NewListIssueComments builds the use case.
func NewListIssueComments(connections outbound.ConnectionRepository, client outbound.JiraClient) *ListIssueComments {
	return &ListIssueComments{connections: connections, client: client}
}

// Execute returns the comments of the issue.
func (u *ListIssueComments) Execute(ctx context.Context, key string) ([]models.Comment, error) {
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return nil, err
	}
	return u.client.Comments(ctx, connection, key)
}
