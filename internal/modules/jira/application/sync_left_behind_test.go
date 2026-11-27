package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func TestSyncRefreshesLinkedIssuesThatLeftEveryLane(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	taskID := "t1"
	issues := &fakeIssues{issues: []models.Issue{
		{Remote: models.RemoteIssue{Key: "A-9", Status: "In Progress"}, LocalState: models.IssueLocalStateConverted, TaskID: &taskID},
		{Remote: models.RemoteIssue{Key: "A-8", Status: "To Do"}, LocalState: models.IssueLocalStateNew},
		{Remote: models.RemoteIssue{Key: "A-1", Status: "To Do"}, LocalState: models.IssueLocalStateConverted},
	}}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = func(string) ([]models.RemoteIssue, error) {
		return []models.RemoteIssue{remote("A-1")}, nil
	}
	client.getIssueFn = func(key string) (models.RemoteIssue, error) {
		return models.RemoteIssue{Key: key, Status: "Done"}, nil
	}

	_, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, issues.updated, 1)
	assert.Equal(t, "A-9", issues.updated[0].Key)
	assert.Equal(t, "Done", issues.updated[0].Status)
	assert.Equal(t, 1, client.calls["get_issue"])
}

func TestSyncKeepsGoingWhenRefreshingALeftBehindIssueFails(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{issues: []models.Issue{
		{Remote: models.RemoteIssue{Key: "A-9", Status: "In Progress"}, LocalState: models.IssueLocalStateConverted},
	}}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.getIssueFn = func(string) (models.RemoteIssue, error) {
		return models.RemoteIssue{}, jiraerrors.Remote("boom")
	}

	outcome, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Empty(t, outcome.FailedLanes)
	assert.Empty(t, issues.updated)
	assert.Nil(t, connections.syncs[0].error)
}

func TestSyncReportsTheLanesWhoseResultsWereTruncated(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "small"), lane("big", "huge")}}
	client := newFakeClient()
	client.truncatedJQL = map[string]bool{"huge": true}
	client.searchFn = func(string) ([]models.RemoteIssue, error) { return []models.RemoteIssue{remote("A-1")}, nil }

	outcome, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"Carril big"}, outcome.TruncatedLanes)
}
