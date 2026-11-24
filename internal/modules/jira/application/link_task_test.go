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

var linkNow = fixedClock(time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC))

func TestLinkTaskToAnImportedIssueMarksItConverted(t *testing.T) {
	issues := &fakeIssues{issues: []models.Issue{issueIn("A-1", models.IssueLocalStateNew)}}
	client := newFakeClient()

	linked, err := application.NewLinkTaskToIssue(issues, connectedRepo(t, "https://j", "t"), client, linkNow).
		Execute(context.Background(), "task-1", " a-1 ")

	require.NoError(t, err)
	assert.Equal(t, "A-1", linked.Key())
	require.Len(t, issues.setStates, 1)
	assert.Equal(t, models.IssueLocalStateConverted, issues.setStates[0].state)
	assert.Equal(t, "task-1", *issues.setStates[0].taskID)
	assert.Zero(t, client.calls["get_issue"], "an imported issue is not fetched again")
}

func TestLinkTaskFetchesAndStoresAnIssueThatWasNotImported(t *testing.T) {
	issues := &fakeIssues{}
	client := newFakeClient()

	linked, err := application.NewLinkTaskToIssue(issues, connectedRepo(t, "https://j", "t"), client, linkNow).
		Execute(context.Background(), "task-1", "B-2")

	require.NoError(t, err)
	assert.Equal(t, "B-2", linked.Key())
	require.Len(t, issues.inserted, 1)
	assert.Equal(t, "task-1", *issues.issues[0].TaskID)
}

func TestLinkTaskRefusesAnInvalidKeyWithoutCallingAnything(t *testing.T) {
	issues := &fakeIssues{}
	client := newFakeClient()

	_, err := application.NewLinkTaskToIssue(issues, connectedRepo(t, "https://j", "t"), client, linkNow).
		Execute(context.Background(), "task-1", "../x")

	require.ErrorIs(t, err, jiraerrors.InvalidIssueKey(""))
	assert.Empty(t, client.calls)
	assert.Empty(t, issues.setStates)
}

func TestLinkTaskTakesOverAnIssueLinkedToAnotherTask(t *testing.T) {
	other := "task-9"
	taken := issueIn("A-1", models.IssueLocalStateConverted)
	taken.TaskID = &other
	issues := &fakeIssues{issues: []models.Issue{taken}}

	linked, err := application.NewLinkTaskToIssue(issues, connectedRepo(t, "https://j", "t"), newFakeClient(), linkNow).
		Execute(context.Background(), "task-1", "A-1")

	require.NoError(t, err)
	require.NotNil(t, linked.TaskID)
	assert.Equal(t, "task-1", *linked.TaskID)
}

func TestLinkTaskWithoutConnectionFailsNotConfigured(t *testing.T) {
	_, err := application.NewLinkTaskToIssue(&fakeIssues{}, &fakeConnections{}, newFakeClient(), linkNow).
		Execute(context.Background(), "task-1", "A-1")

	require.ErrorIs(t, err, jiraerrors.NotConfigured())
}

func TestGetTaskIssueReturnsTheLinkedIssueOrNil(t *testing.T) {
	taskID := "task-1"
	linked := issueIn("A-1", models.IssueLocalStateConverted)
	linked.TaskID = &taskID
	use := application.NewGetTaskIssue(&fakeIssues{issues: []models.Issue{linked}})

	found, err := use.Execute(context.Background(), "task-1")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "A-1", found.Key())

	none, err := use.Execute(context.Background(), "task-2")
	require.NoError(t, err)
	assert.Nil(t, none)
}

func TestLinkTaskMovesBothLinksInOneCall(t *testing.T) {
	other, mine := "task-9", "task-1"
	taken := issueIn("A-1", models.IssueLocalStateConverted)
	taken.TaskID = &other
	previous := issueIn("A-2", models.IssueLocalStateConverted)
	previous.TaskID = &mine
	issues := &fakeIssues{issues: []models.Issue{taken, previous}}

	linked, err := application.NewLinkTaskToIssue(issues, connectedRepo(t, "https://j", "t"), newFakeClient(), linkNow).
		Execute(context.Background(), "task-1", "A-1")

	require.NoError(t, err)
	require.NotNil(t, linked.TaskID)
	assert.Equal(t, "task-1", *linked.TaskID)
	require.Len(t, issues.setStates, 2)
	assert.Equal(t, "A-2", issues.setStates[0].key)
	assert.Equal(t, models.IssueLocalStateNew, issues.setStates[0].state)
	assert.Nil(t, issues.setStates[0].taskID)
}
