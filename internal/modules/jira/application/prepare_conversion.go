//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// ConversionPreparation is everything the conversion screen needs: the issue, the suggested task
// content and the workspaces to map it onto.
type ConversionPreparation struct {
	Issue      models.Issue
	Draft      models.ConversionDraft
	Workspaces []models.WorkspaceOption
}

// PrepareConversion gathers what the conversion screen shows.
type PrepareConversion struct {
	issues     outbound.IssueRepository
	workspaces outbound.WorkspaceCatalog
}

// NewPrepareConversion builds the use case.
func NewPrepareConversion(issues outbound.IssueRepository, workspaces outbound.WorkspaceCatalog) *PrepareConversion {
	return &PrepareConversion{issues: issues, workspaces: workspaces}
}

// Execute loads the issue, its suggested draft and the workspaces.
func (u *PrepareConversion) Execute(ctx context.Context, key string) (ConversionPreparation, error) {
	issue, err := u.issues.Find(ctx, key)
	if err != nil {
		return ConversionPreparation{}, err
	}
	if issue == nil {
		return ConversionPreparation{}, jiraerrors.IssueNotFound(key)
	}
	workspaces, err := u.workspaces.Workspaces(ctx)
	if err != nil {
		return ConversionPreparation{}, err
	}
	return ConversionPreparation{Issue: *issue, Draft: models.DraftFromIssue(*issue), Workspaces: workspaces}, nil
}
