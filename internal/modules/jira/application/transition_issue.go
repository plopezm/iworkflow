//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// TransitionIssue moves the issue to another status in Jira and refreshes the local copy.
type TransitionIssue struct {
	connections outbound.ConnectionRepository
	issues      outbound.IssueRepository
	client      outbound.JiraClient
	clock       Clock
}

// NewTransitionIssue builds the use case.
func NewTransitionIssue(connections outbound.ConnectionRepository, issues outbound.IssueRepository, client outbound.JiraClient, clock Clock) *TransitionIssue {
	return &TransitionIssue{connections: connections, issues: issues, client: client, clock: clock}
}

// Execute transitions the issue and returns its local copy after the refresh.
func (u *TransitionIssue) Execute(ctx context.Context, key string, transitionID string) (models.Issue, error) {
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return models.Issue{}, err
	}
	known, err := u.issues.Find(ctx, key)
	if err != nil {
		return models.Issue{}, err
	}
	if known == nil {
		return models.Issue{}, jiraerrors.IssueNotFound(key)
	}
	if err := u.client.Transition(ctx, connection, key, transitionID); err != nil {
		return models.Issue{}, err
	}
	// The transition is already done in Jira. If refreshing the local copy fails, keep the stale copy
	// instead of reporting a failed transition: the next sync catches up.
	if refreshed, err := u.client.GetIssue(ctx, connection, key); err == nil {
		_ = u.issues.UpdateRemote(ctx, refreshed, u.clock())
	}
	current, err := u.issues.Find(ctx, key)
	if err != nil {
		return models.Issue{}, err
	}
	if current == nil {
		return models.Issue{}, jiraerrors.IssueNotFound(key)
	}
	return *current, nil
}
