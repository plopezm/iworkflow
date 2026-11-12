package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func newRefreshIssue(t *testing.T, issues *fakeIssues, client *fakeClient) *application.RefreshIssue {
	t.Helper()
	return application.NewRefreshIssue(connectedRepo(t, "https://j", "t"), issues, client, fixedClock(syncAt))
}

func TestRefreshIssueUpdatesAKnownIssueAndKeepsItsLocalState(t *testing.T) {
	taskID := "task-1"
	linked := issueIn("A-1", models.IssueLocalStateConverted)
	linked.TaskID = &taskID
	issues := &fakeIssues{issues: []models.Issue{linked}}
	client := newFakeClient()
	client.getIssueFn = func(key string) (models.RemoteIssue, error) {
		fresh := remote(key)
		fresh.Summary = "resumen nuevo"
		fresh.Status = "Done"
		return fresh, nil
	}

	got, err := newRefreshIssue(t, issues, client).Execute(context.Background(), " a-1 ")

	require.NoError(t, err)
	assert.Equal(t, "resumen nuevo", got.Remote.Summary)
	assert.Equal(t, "Done", got.Remote.Status)
	assert.Equal(t, models.IssueLocalStateConverted, got.LocalState, "a refresh never changes the local state")
	assert.Equal(t, &taskID, got.TaskID)
	assert.Empty(t, issues.inserted)
	require.Len(t, issues.updated, 1)
	assert.Equal(t, "A-1", issues.updated[0].Key)
}

func TestRefreshIssueInsertsAnIssueThatWasNeverImported(t *testing.T) {
	issues := &fakeIssues{}
	client := newFakeClient()

	got, err := newRefreshIssue(t, issues, client).Execute(context.Background(), "B-7")

	require.NoError(t, err)
	assert.Equal(t, "B-7", got.Key())
	assert.Equal(t, models.IssueLocalStateNew, got.LocalState)
	require.Len(t, issues.inserted, 1)
	assert.Empty(t, issues.updated)
}

func TestRefreshIssueRefusesInvalidKeysBeforeAnyCall(t *testing.T) {
	issues := &fakeIssues{}
	client := newFakeClient()

	_, err := newRefreshIssue(t, issues, client).Execute(context.Background(), "../x")

	require.ErrorIs(t, err, jiraerrors.InvalidIssueKey("../X"))
	assert.Zero(t, client.calls["get_issue"])
}

func TestRefreshIssueKeepsTheStoredCopyWhenJiraFails(t *testing.T) {
	stored := issueIn("A-1", models.IssueLocalStateNew)
	issues := &fakeIssues{issues: []models.Issue{stored}}
	client := newFakeClient()
	client.getIssueFn = func(string) (models.RemoteIssue, error) { return models.RemoteIssue{}, errors.New("jira caído") }

	_, err := newRefreshIssue(t, issues, client).Execute(context.Background(), "A-1")

	require.EqualError(t, err, "jira caído")
	assert.Empty(t, issues.updated)
	assert.Empty(t, issues.inserted)
}

func TestRefreshIssueNeedsAConnection(t *testing.T) {
	client := newFakeClient()
	useCase := application.NewRefreshIssue(&fakeConnections{}, &fakeIssues{}, client, fixedClock(syncAt))

	_, err := useCase.Execute(context.Background(), "A-1")

	require.ErrorIs(t, err, jiraerrors.NotConfigured())
	assert.Zero(t, client.calls["get_issue"])
}
