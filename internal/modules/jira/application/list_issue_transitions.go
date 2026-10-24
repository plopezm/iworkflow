//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// ListIssueTransitions lists the status changes Jira allows right now (they depend on the workflow).
type ListIssueTransitions struct {
	connections outbound.ConnectionRepository
	client      outbound.JiraClient
}

// NewListIssueTransitions builds the use case.
func NewListIssueTransitions(connections outbound.ConnectionRepository, client outbound.JiraClient) *ListIssueTransitions {
	return &ListIssueTransitions{connections: connections, client: client}
}

// Execute returns the allowed transitions of the issue.
func (u *ListIssueTransitions) Execute(ctx context.Context, key string) ([]models.Transition, error) {
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return nil, err
	}
	return u.client.Transitions(ctx, connection, key)
}
