// Package outbound declares the ports the jira use cases depend on. Adapters implement them.
package outbound

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
)

// JiraClient talks to the Jira Server/Data Center REST API v2. Every method takes the connection, so
// the client holds no credentials of its own.
type JiraClient interface {
	// Verify checks URL and token and returns the display name of the authenticated user.
	Verify(ctx context.Context, connection models.Connection) (string, error)
	// Search returns the issues of one JQL query, bounded by the adapter's import limit.
	Search(ctx context.Context, connection models.Connection, jql string) ([]models.RemoteIssue, error)
	GetIssue(ctx context.Context, connection models.Connection, key string) (models.RemoteIssue, error)
	Comments(ctx context.Context, connection models.Connection, key string) ([]models.Comment, error)
	AddComment(ctx context.Context, connection models.Connection, key string, body string) error
	Transitions(ctx context.Context, connection models.Connection, key string) ([]models.Transition, error)
	Transition(ctx context.Context, connection models.Connection, key string, transitionID string) error
}
