//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"strings"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// SaveConnectionInput is what the user typed on the connection form.
type SaveConnectionInput struct {
	BaseURL string
	// Token blank keeps the stored token, so the token never has to travel back to the browser.
	Token              string
	AcceptInvalidCerts bool
}

// SaveConnection validates the connection against the server and only then stores it. Pointing the
// connection at another server discards the issues imported from the previous one.
type SaveConnection struct {
	connections outbound.ConnectionRepository
	issues      outbound.IssueRepository
	client      outbound.JiraClient
}

// NewSaveConnection builds the use case.
func NewSaveConnection(connections outbound.ConnectionRepository, issues outbound.IssueRepository, client outbound.JiraClient) *SaveConnection {
	return &SaveConnection{connections: connections, issues: issues, client: client}
}

// Execute verifies, stores and returns the connection as it is now stored.
func (u *SaveConnection) Execute(ctx context.Context, input SaveConnectionInput) (models.Connection, error) {
	existing, err := u.connections.Get(ctx)
	if err != nil {
		return models.Connection{}, err
	}
	typedToken := strings.TrimSpace(input.Token)
	token := typedToken
	if token == "" && existing != nil {
		token = existing.Token()
	}
	connection, err := models.NewConnection(input.BaseURL, token, input.AcceptInvalidCerts)
	if err != nil {
		return models.Connection{}, err
	}
	serverChanged := existing != nil && existing.BaseURL() != connection.BaseURL()
	// A stored token is never sent to a different server: it has to be typed again.
	if typedToken == "" && serverChanged {
		return models.Connection{}, jiraerrors.TokenRequiredForNewURL()
	}
	if _, err := u.client.Verify(ctx, connection); err != nil {
		return models.Connection{}, err
	}
	// Save first: if it fails nothing has been wiped. Issues left over from the previous server (the
	// wipe is best effort) are retired by the next sync, which only keeps what the new server returns.
	if err := u.connections.Save(ctx, connection); err != nil {
		return models.Connection{}, err
	}
	if serverChanged {
		_ = u.issues.DeleteAll(ctx) // Best effort: the next sync retires whatever is left.
	}
	saved, err := u.connections.Get(ctx)
	if err != nil {
		return models.Connection{}, err
	}
	if saved == nil {
		return models.Connection{}, jiraerrors.NotConfigured()
	}
	return *saved, nil
}
