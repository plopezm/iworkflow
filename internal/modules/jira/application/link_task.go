//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"strings"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// LinkTaskToIssue links an existing iworkflow task to a Jira issue chosen by its key. An issue that
// was never imported is fetched from Jira and stored first. Local only: nothing is written to Jira.
type LinkTaskToIssue struct {
	issues      outbound.IssueRepository
	connections outbound.ConnectionRepository
	client      outbound.JiraClient
	clock       Clock
}

// NewLinkTaskToIssue builds the use case.
func NewLinkTaskToIssue(
	issues outbound.IssueRepository,
	connections outbound.ConnectionRepository,
	client outbound.JiraClient,
	clock Clock,
) *LinkTaskToIssue {
	return &LinkTaskToIssue{issues: issues, connections: connections, client: client, clock: clock}
}

// Execute links the task and returns the linked issue.
func (u *LinkTaskToIssue) Execute(ctx context.Context, taskID string, key string) (models.Issue, error) {
	normalized := strings.ToUpper(strings.TrimSpace(key))
	if !models.IsValidIssueKey(normalized) {
		return models.Issue{}, jiraerrors.InvalidIssueKey(normalized)
	}
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return models.Issue{}, err
	}
	issue, err := u.issues.Find(ctx, normalized)
	if err != nil {
		return models.Issue{}, err
	}
	if issue == nil {
		if err := u.importIssue(ctx, connection, normalized); err != nil {
			return models.Issue{}, err
		}
	}
	// Linking moves the link: the task's previous issue is released.
	if err := u.releaseOtherLink(ctx, taskID, normalized); err != nil {
		return models.Issue{}, err
	}
	if err := u.issues.SetLocalState(ctx, normalized, models.IssueLocalStateConverted, &taskID); err != nil {
		return models.Issue{}, err
	}
	linked, err := u.issues.Find(ctx, normalized)
	if err != nil {
		return models.Issue{}, err
	}
	if linked == nil {
		return models.Issue{}, jiraerrors.IssueNotFound(normalized)
	}
	return *linked, nil
}

// releaseOtherLink keeps one issue per task: a task already linked to another issue is unlinked
// from it (the issue goes back to new) so the task-to-issue lookup stays unambiguous.
func (u *LinkTaskToIssue) releaseOtherLink(ctx context.Context, taskID string, key string) error {
	current, err := u.issues.FindByTaskID(ctx, taskID)
	if err != nil {
		return err
	}
	if current != nil && current.Key() != key {
		return u.issues.SetLocalState(ctx, current.Key(), models.IssueLocalStateNew, nil)
	}
	return nil
}

func (u *LinkTaskToIssue) importIssue(ctx context.Context, connection models.Connection, key string) error {
	remote, err := u.client.GetIssue(ctx, connection, key)
	if err != nil {
		return err
	}
	return u.issues.InsertRemote(ctx, remote, u.clock())
}

// GetTaskIssue finds the issue linked to a task; nil means the task has none.
type GetTaskIssue struct {
	issues outbound.IssueRepository
}

// NewGetTaskIssue builds the use case.
func NewGetTaskIssue(issues outbound.IssueRepository) *GetTaskIssue {
	return &GetTaskIssue{issues: issues}
}

// Execute returns the linked issue, or nil when the task is not linked.
func (u *GetTaskIssue) Execute(ctx context.Context, taskID string) (*models.Issue, error) {
	return u.issues.FindByTaskID(ctx, taskID)
}
