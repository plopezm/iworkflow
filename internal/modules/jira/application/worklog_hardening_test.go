package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func TestRegisterWorklogRetriesTheLocalSaveBeforeGivingUp(t *testing.T) {
	worklogs := &fakeWorklogs{failAddTimes: 2}

	_, err := newRegisterWorklog(t, linkedIssues("task-1"), newFakeClient(), worklogs).Execute(context.Background(),
		application.RegisterWorklogInput{TaskID: "task-1", Time: "1h", Description: "x"})

	require.NoError(t, err)
	assert.Len(t, worklogs.records, 1)
}

func TestRegisterWorklogRefusesAnIdenticalEntryLoggedAMomentAgo(t *testing.T) {
	client := newFakeClient()
	worklogs := &fakeWorklogs{records: []models.WorklogRecord{
		{TaskID: "task-1", Seconds: 3600, Comment: "x", LoggedAt: worklogNow.Add(-30 * time.Second)},
	}}

	_, err := newRegisterWorklog(t, linkedIssues("task-1"), client, worklogs).Execute(context.Background(),
		application.RegisterWorklogInput{TaskID: "task-1", Time: "1h", Description: "x"})

	require.ErrorIs(t, err, jiraerrors.DuplicateWorklog())
	assert.Empty(t, client.calls)
	assert.Len(t, worklogs.records, 1)
}

func TestRegisterWorklogAllowsTheSameEntryAfterTheDuplicateWindow(t *testing.T) {
	worklogs := &fakeWorklogs{records: []models.WorklogRecord{
		{TaskID: "task-1", Seconds: 3600, Comment: "x", LoggedAt: worklogNow.Add(-10 * time.Minute)},
	}}

	_, err := newRegisterWorklog(t, linkedIssues("task-1"), newFakeClient(), worklogs).Execute(context.Background(),
		application.RegisterWorklogInput{TaskID: "task-1", Time: "1h", Description: "x"})

	require.NoError(t, err)
	assert.Len(t, worklogs.records, 2)
}

func TestLinkTaskReleasesTheTaskPreviousIssue(t *testing.T) {
	taskID := "task-1"
	current := issueIn("A-1", models.IssueLocalStateConverted)
	current.TaskID = &taskID
	issues := &fakeIssues{issues: []models.Issue{current, issueIn("A-2", models.IssueLocalStateNew)}}
	client := newFakeClient()

	linked, err := application.NewLinkTaskToIssue(issues, connectedRepo(t, "https://j", "t"), client, linkNow).
		Execute(context.Background(), "task-1", "A-2")

	require.NoError(t, err)
	assert.Equal(t, "A-2", linked.Key())
	require.Len(t, issues.setStates, 2)
	assert.Equal(t, "A-1", issues.setStates[0].key)
	assert.Equal(t, models.IssueLocalStateNew, issues.setStates[0].state)
	assert.Nil(t, issues.setStates[0].taskID)
	assert.Equal(t, "A-2", issues.setStates[1].key)
	assert.Equal(t, "task-1", *issues.setStates[1].taskID)
}

func TestLinkTaskToTheSameIssueAgainIsHarmless(t *testing.T) {
	taskID := "task-1"
	current := issueIn("A-1", models.IssueLocalStateConverted)
	current.TaskID = &taskID
	issues := &fakeIssues{issues: []models.Issue{current}}

	result, err := application.NewLinkTaskToIssue(issues, connectedRepo(t, "https://j", "t"), newFakeClient(), linkNow).
		Execute(context.Background(), "task-1", "A-1")

	require.NoError(t, err)
	assert.Equal(t, "A-1", result.Key())
}
