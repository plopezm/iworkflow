//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// RegisterWorklogInput is what the user confirmed in the worklog form.
type RegisterWorklogInput struct {
	TaskID      string
	Time        string
	Description string
	AlsoComment bool
}

// RegisteredWorklog is the outcome: the stored record and, when the optional comment failed, why.
type RegisteredWorklog struct {
	Record       models.WorklogRecord
	CommentError string
}

// RegisterWorklog logs time on the Jira issue linked to a task. It always runs on the user's explicit
// confirmation, never automatically.
type RegisterWorklog struct {
	issues      outbound.IssueRepository
	connections outbound.ConnectionRepository
	client      outbound.JiraClient
	worklogs    outbound.WorklogRepository
	clock       Clock
}

// NewRegisterWorklog builds the use case.
func NewRegisterWorklog(
	issues outbound.IssueRepository,
	connections outbound.ConnectionRepository,
	client outbound.JiraClient,
	worklogs outbound.WorklogRepository,
	clock Clock,
) *RegisterWorklog {
	return &RegisterWorklog{issues: issues, connections: connections, client: client, worklogs: worklogs, clock: clock}
}

// Execute validates the form, sends the worklog and stores the record. The work is reported as
// finished now, so it started the logged duration ago.
func (u *RegisterWorklog) Execute(ctx context.Context, input RegisterWorklogInput) (RegisteredWorklog, error) {
	spent, err := models.ParseJiraDuration(input.Time)
	if err != nil {
		return RegisteredWorklog{}, err
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return RegisteredWorklog{}, jiraerrors.EmptyWorklogDescription()
	}
	issue, err := u.issues.FindByTaskID(ctx, input.TaskID)
	if err != nil {
		return RegisteredWorklog{}, err
	}
	if issue == nil {
		return RegisteredWorklog{}, jiraerrors.TaskNotLinked()
	}
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return RegisteredWorklog{}, err
	}
	now := u.clock()
	if err := u.refuseDuplicate(ctx, input.TaskID, spent, description, now); err != nil {
		return RegisteredWorklog{}, err
	}
	started := now.Add(-spent)
	seconds := int(spent / time.Second)
	jiraID, err := u.client.AddWorklog(ctx, connection, issue.Key(), models.NewWorklog(started, seconds, description))
	if err != nil {
		return RegisteredWorklog{}, err
	}
	record := models.WorklogRecord{
		TaskID: input.TaskID, IssueKey: issue.Key(), JiraID: jiraID,
		Seconds: seconds, Comment: description, StartedAt: started, LoggedAt: now,
	}
	if err := u.saveLocally(ctx, record); err != nil {
		return RegisteredWorklog{}, fmt.Errorf("el tiempo ya se registró en Jira (worklog %s) pero no se pudo guardar localmente: %w", jiraID, err)
	}
	result := RegisteredWorklog{Record: record}
	if input.AlsoComment {
		if err := u.client.AddComment(ctx, connection, issue.Key(), description); err != nil {
			result.CommentError = err.Error()
		}
	}
	return result, nil
}

// duplicateWindow is how long an identical worklog is refused, which stops a double click or a resend.
const duplicateWindow = 2 * time.Minute

// localSaveAttempts bounds the tries to store the record once Jira accepted the worklog.
const localSaveAttempts = 3

func (u *RegisterWorklog) refuseDuplicate(ctx context.Context, taskID string, spent time.Duration, description string, now time.Time) error {
	previous, err := u.worklogs.ListByTask(ctx, taskID)
	if err != nil {
		return err
	}
	for _, record := range previous {
		sameEntry := time.Duration(record.Seconds)*time.Second == spent && record.Comment == description
		if sameEntry && now.Sub(record.LoggedAt) < duplicateWindow {
			return jiraerrors.DuplicateWorklog()
		}
	}
	return nil
}

// saveLocally retries the write: after Jira accepted the worklog a failed save is the one case that
// could lead the user to log the same time twice.
func (u *RegisterWorklog) saveLocally(ctx context.Context, record models.WorklogRecord) error {
	var err error
	for range localSaveAttempts {
		if err = u.worklogs.Add(ctx, record); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
	}
	return err
}

// TaskWorklogs is what was already logged for a task.
type TaskWorklogs struct {
	Records []models.WorklogRecord
	Total   time.Duration
}

// ListTaskWorklogs reads the worklogs sent for a task and their total.
type ListTaskWorklogs struct {
	worklogs outbound.WorklogRepository
}

// NewListTaskWorklogs builds the use case.
func NewListTaskWorklogs(worklogs outbound.WorklogRepository) *ListTaskWorklogs {
	return &ListTaskWorklogs{worklogs: worklogs}
}

// Execute returns the records and the total logged time.
func (u *ListTaskWorklogs) Execute(ctx context.Context, taskID string) (TaskWorklogs, error) {
	records, err := u.worklogs.ListByTask(ctx, taskID)
	if err != nil {
		return TaskWorklogs{}, err
	}
	var total time.Duration
	for _, record := range records {
		total += time.Duration(record.Seconds) * time.Second
	}
	return TaskWorklogs{Records: records, Total: total}, nil
}
