//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"fmt"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// linkAttempts is how many times the link between the issue and the new task is recorded.
const linkAttempts = 3

// ConvertedIssue is the task created from an issue. StartError explains a task that stayed in the
// backlog or a link that could not be recorded.
type ConvertedIssue struct {
	TaskID     string
	StartError *string
}

// ConvertIssue creates the iworkflow task from the user-confirmed mapping and links it to the issue.
type ConvertIssue struct {
	issues outbound.IssueRepository
	tasks  outbound.TaskCreation
}

// NewConvertIssue builds the use case.
func NewConvertIssue(issues outbound.IssueRepository, tasks outbound.TaskCreation) *ConvertIssue {
	return &ConvertIssue{issues: issues, tasks: tasks}
}

// Execute validates the mapping, creates the task and links it to the issue.
func (u *ConvertIssue) Execute(ctx context.Context, request models.ConversionRequest) (ConvertedIssue, error) {
	validated, err := request.Validated()
	if err != nil {
		return ConvertedIssue{}, err
	}
	issue, err := u.issues.Find(ctx, validated.Key)
	if err != nil {
		return ConvertedIssue{}, err
	}
	if issue == nil {
		return ConvertedIssue{}, jiraerrors.IssueNotFound(validated.Key)
	}
	if issue.LocalState == models.IssueLocalStateConverted {
		return ConvertedIssue{}, jiraerrors.AlreadyConverted(validated.Key)
	}
	created, err := u.tasks.Create(ctx, outbound.NewLinkedTask{
		WorkspaceID: validated.WorkspaceID,
		BaseBranch:  validated.BaseBranch,
		Title:       validated.Title,
		Prompt:      validated.Prompt,
		Priority:    validated.Priority,
		StartNow:    validated.StartNow,
	})
	if err != nil {
		return ConvertedIssue{}, err
	}
	// The task already exists: failing here would invite a retry that creates a duplicate. Retry the
	// link a few times and, if it still fails, report it next to the task instead.
	linkError := u.link(ctx, validated.Key, created.TaskID)
	return ConvertedIssue{TaskID: created.TaskID, StartError: combineStartAndLink(created.StartError, linkError)}, nil
}

// link records the conversion and returns the text of the last failure, nil when it worked.
func (u *ConvertIssue) link(ctx context.Context, key string, taskID string) *string {
	var linkError *string
	for range linkAttempts {
		err := u.issues.SetLocalState(ctx, key, models.IssueLocalStateConverted, &taskID)
		linkError = nil
		if err == nil {
			break
		}
		message := err.Error()
		linkError = &message
	}
	return linkError
}

func combineStartAndLink(startError *string, linkError *string) *string {
	var message string
	switch {
	case linkError == nil:
		return startError
	case startError == nil:
		message = fmt.Sprintf("La tarea se creó, pero no se pudo enlazar con la tarea de Jira: %s", *linkError)
	default:
		message = fmt.Sprintf("%s. Además no se pudo enlazar con la tarea de Jira: %s", *startError, *linkError)
	}
	return &message
}
