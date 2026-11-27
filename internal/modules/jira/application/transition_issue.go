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
	target := u.targetStatus(ctx, connection, key, transitionID)
	if err := u.client.Transition(ctx, connection, key, transitionID); err != nil {
		return models.Issue{}, err
	}
	// The transition is already done in Jira, so a failure while refreshing the local copy is never reported as a
	// failed transition. The read is retried once; if it still fails, the status the transition leads to is stored,
	// so the local copy does not keep showing the old status until the next sync.
	refreshed, err := u.client.GetIssue(ctx, connection, key)
	if err != nil {
		refreshed, err = u.client.GetIssue(ctx, connection, key)
	}
	switch {
	case err == nil:
		_ = u.issues.UpdateRemote(ctx, refreshed, u.clock())
	case target != "":
		optimistic := known.Remote
		optimistic.Status = target
		_ = u.issues.UpdateRemote(ctx, optimistic, u.clock())
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

// targetStatus is the status the transition leads to, or "" when Jira does not say. It is best effort: a failure
// here never blocks the transition.
func (u *TransitionIssue) targetStatus(ctx context.Context, connection models.Connection, key string, transitionID string) string {
	transitions, err := u.client.Transitions(ctx, connection, key)
	if err != nil {
		return ""
	}
	for _, transition := range transitions {
		if transition.ID == transitionID {
			return transition.ToStatus
		}
	}
	return ""
}
