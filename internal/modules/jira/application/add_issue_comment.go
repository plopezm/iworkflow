//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"strings"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// AddIssueComment posts a comment on the issue in Jira.
type AddIssueComment struct {
	connections outbound.ConnectionRepository
	client      outbound.JiraClient
}

// NewAddIssueComment builds the use case.
func NewAddIssueComment(connections outbound.ConnectionRepository, client outbound.JiraClient) *AddIssueComment {
	return &AddIssueComment{connections: connections, client: client}
}

// Execute posts the trimmed comment. A blank comment is refused before anything is called.
func (u *AddIssueComment) Execute(ctx context.Context, key string, body string) error {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return jiraerrors.EmptyComment()
	}
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return err
	}
	return u.client.AddComment(ctx, connection, key, trimmed)
}
