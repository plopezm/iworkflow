//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// GetIssue reads one imported issue.
type GetIssue struct {
	issues outbound.IssueRepository
}

// NewGetIssue builds the use case.
func NewGetIssue(issues outbound.IssueRepository) *GetIssue {
	return &GetIssue{issues: issues}
}

// Execute returns the issue, or IssueNotFound.
func (u *GetIssue) Execute(ctx context.Context, key string) (models.Issue, error) {
	issue, err := u.issues.Find(ctx, key)
	if err != nil {
		return models.Issue{}, err
	}
	if issue == nil {
		return models.Issue{}, jiraerrors.IssueNotFound(key)
	}
	return *issue, nil
}
