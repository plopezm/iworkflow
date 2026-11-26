//nolint:wrapcheck // Port errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// ReleaseTaskIssue frees the Jira issue linked to a task, so a new task can be created from it. The issue goes back to
// new. An issue that is no longer converted (for example ignored since) is left as it is. Local only: nothing is written to Jira.
type ReleaseTaskIssue struct {
	issues outbound.IssueRepository
}

// NewReleaseTaskIssue builds the use case.
func NewReleaseTaskIssue(issues outbound.IssueRepository) *ReleaseTaskIssue {
	return &ReleaseTaskIssue{issues: issues}
}

// Execute unlinks the issue of the task. A task without a converted issue is left alone.
func (u *ReleaseTaskIssue) Execute(ctx context.Context, taskID string) error {
	issue, err := u.issues.FindByTaskID(ctx, taskID)
	if err != nil {
		return err
	}
	if issue == nil || issue.LocalState != models.IssueLocalStateConverted {
		return nil
	}
	return u.issues.SetLocalState(ctx, issue.Key(), models.IssueLocalStateNew, nil)
}
