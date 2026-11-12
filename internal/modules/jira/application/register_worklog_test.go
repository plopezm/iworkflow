package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

var worklogNow = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

func linkedIssues(taskID string) *fakeIssues {
	linked := issueIn("A-1", models.IssueLocalStateConverted)
	linked.TaskID = &taskID
	return &fakeIssues{issues: []models.Issue{linked}}
}

func newRegisterWorklog(t *testing.T, issues *fakeIssues, client *fakeClient, worklogs *fakeWorklogs) *application.RegisterWorklog {
	t.Helper()
	return application.NewRegisterWorklog(issues, connectedRepo(t, "https://j", "t"), client, worklogs, fixedClock(worklogNow))
}

func TestRegisterWorklogSendsParsedTimeAndStoresTheRecord(t *testing.T) {
	client := newFakeClient()
	client.addWorklogFn = func(_ string, _ models.Worklog) (string, error) { return "777", nil }
	worklogs := &fakeWorklogs{}

	result, err := newRegisterWorklog(t, linkedIssues("task-1"), client, worklogs).Execute(context.Background(),
		application.RegisterWorklogInput{TaskID: "task-1", Time: "1h 30m", Description: "  hizo cosas "})

	require.NoError(t, err)
	assert.Equal(t, []string{"A-1:hizo cosas"}, client.postedWorklogs)
	require.Len(t, worklogs.records, 1)
	record := worklogs.records[0]
	assert.Equal(t, "777", record.JiraID)
	assert.Equal(t, "A-1", record.IssueKey)
	assert.Equal(t, 5400, record.Seconds)
	assert.Equal(t, worklogNow.Add(-90*time.Minute), record.StartedAt)
	assert.Equal(t, record, result.Record)
	assert.Empty(t, client.postedComments)
}

func TestRegisterWorklogRefusesInvalidInputBeforeCallingJira(t *testing.T) {
	tests := []struct {
		name  string
		input application.RegisterWorklogInput
		want  error
	}{
		{"bad time", application.RegisterWorklogInput{TaskID: "task-1", Time: "soon", Description: "x"}, jiraerrors.InvalidDuration("")},
		{"blank description", application.RegisterWorklogInput{TaskID: "task-1", Time: "1h", Description: "  "}, jiraerrors.EmptyWorklogDescription()},
		{"task not linked", application.RegisterWorklogInput{TaskID: "task-2", Time: "1h", Description: "x"}, jiraerrors.TaskNotLinked()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient()
			worklogs := &fakeWorklogs{}

			_, err := newRegisterWorklog(t, linkedIssues("task-1"), client, worklogs).Execute(context.Background(), tt.input)

			require.ErrorIs(t, err, tt.want)
			assert.Empty(t, client.calls)
			assert.Empty(t, worklogs.records)
		})
	}
}

func TestRegisterWorklogKeepsNothingWhenJiraRejectsIt(t *testing.T) {
	client := newFakeClient()
	client.addWorklogFn = func(string, models.Worklog) (string, error) { return "", jiraerrors.Unauthorized() }
	worklogs := &fakeWorklogs{}

	_, err := newRegisterWorklog(t, linkedIssues("task-1"), client, worklogs).Execute(context.Background(),
		application.RegisterWorklogInput{TaskID: "task-1", Time: "1h", Description: "x"})

	require.ErrorIs(t, err, jiraerrors.Unauthorized())
	assert.Empty(t, worklogs.records)
}

func TestRegisterWorklogCanAlsoCommentAndAFailedCommentDoesNotUndoTheWorklog(t *testing.T) {
	client := newFakeClient()
	client.addCommentFn = func(string, string) error { return errors.New("boom") }
	worklogs := &fakeWorklogs{}

	result, err := newRegisterWorklog(t, linkedIssues("task-1"), client, worklogs).Execute(context.Background(),
		application.RegisterWorklogInput{TaskID: "task-1", Time: "1h", Description: "desc", AlsoComment: true})

	require.NoError(t, err)
	assert.Len(t, worklogs.records, 1)
	assert.NotEmpty(t, result.CommentError)
	assert.Equal(t, []string{"A-1:desc"}, client.postedComments)
}

func TestRegisterWorklogReportsALocalSaveFailureAfterJiraAccepted(t *testing.T) {
	client := newFakeClient()
	worklogs := &fakeWorklogs{addErr: errors.New("disk full")}

	_, err := newRegisterWorklog(t, linkedIssues("task-1"), client, worklogs).Execute(context.Background(),
		application.RegisterWorklogInput{TaskID: "task-1", Time: "1h", Description: "x"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ya se registró en Jira")
}

func TestListTaskWorklogsReturnsRecordsAndTheLoggedTotal(t *testing.T) {
	worklogs := &fakeWorklogs{records: []models.WorklogRecord{
		{TaskID: "task-1", Seconds: 3600}, {TaskID: "task-1", Seconds: 900}, {TaskID: "task-2", Seconds: 60},
	}}

	summary, err := application.NewListTaskWorklogs(worklogs).Execute(context.Background(), "task-1")

	require.NoError(t, err)
	assert.Len(t, summary.Records, 2)
	assert.Equal(t, 75*time.Minute, summary.Total)
}
