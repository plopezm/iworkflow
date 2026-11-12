//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"strings"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// RefreshIssue reads the issue from Jira now and stores its Jira data locally, keeping its local state. An issue
// that was never imported is inserted. Local state is never changed by a refresh.
type RefreshIssue struct {
	connections outbound.ConnectionRepository
	issues      outbound.IssueRepository
	client      outbound.JiraClient
	clock       Clock
}

// NewRefreshIssue builds the use case.
func NewRefreshIssue(
	connections outbound.ConnectionRepository,
	issues outbound.IssueRepository,
	client outbound.JiraClient,
	clock Clock,
) *RefreshIssue {
	return &RefreshIssue{connections: connections, issues: issues, client: client, clock: clock}
}

// Execute refreshes the issue and returns its local copy. A Jira failure leaves the stored copy untouched.
func (u *RefreshIssue) Execute(ctx context.Context, key string) (models.Issue, error) {
	normalized := normalizeIssueKey(key)
	if !models.IsValidIssueKey(normalized) {
		return models.Issue{}, jiraerrors.InvalidIssueKey(normalized)
	}
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return models.Issue{}, err
	}
	remote, err := u.client.GetIssue(ctx, connection, normalized)
	if err != nil {
		return models.Issue{}, err
	}
	known, err := u.issues.Find(ctx, normalized)
	if err != nil {
		return models.Issue{}, err
	}
	if known == nil {
		err = u.issues.InsertRemote(ctx, remote, u.clock())
	} else {
		err = u.issues.UpdateRemote(ctx, remote, u.clock())
	}
	if err != nil {
		return models.Issue{}, err
	}
	current, err := u.issues.Find(ctx, normalized)
	if err != nil {
		return models.Issue{}, err
	}
	if current == nil {
		return models.Issue{}, jiraerrors.IssueNotFound(normalized)
	}
	return *current, nil
}

// normalizeIssueKey trims and upper-cases a key typed by the user, so `proj-1 ` finds `PROJ-1`.
func normalizeIssueKey(key string) string {
	return strings.ToUpper(strings.TrimSpace(key))
}
