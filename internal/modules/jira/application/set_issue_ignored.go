//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// SetIssueIgnored marks an issue as ignored, or makes it New again (also unlinking a converted task
// so it can be converted anew). Local only: nothing is written to Jira.
type SetIssueIgnored struct {
	issues outbound.IssueRepository
}

// NewSetIssueIgnored builds the use case.
func NewSetIssueIgnored(issues outbound.IssueRepository) *SetIssueIgnored {
	return &SetIssueIgnored{issues: issues}
}

// Execute changes the local state of the issue.
func (u *SetIssueIgnored) Execute(ctx context.Context, key string, ignored bool) error {
	issue, err := u.issues.Find(ctx, key)
	if err != nil {
		return err
	}
	if issue == nil {
		return jiraerrors.IssueNotFound(key)
	}
	if !ignored {
		return u.issues.SetLocalState(ctx, key, models.IssueLocalStateNew, nil)
	}
	if issue.LocalState == models.IssueLocalStateConverted {
		return jiraerrors.AlreadyConverted(key)
	}
	return u.issues.SetLocalState(ctx, key, models.IssueLocalStateIgnored, nil)
}
