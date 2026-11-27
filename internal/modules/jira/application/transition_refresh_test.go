package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func newTransition(t *testing.T, issues *fakeIssues, client *fakeClient) *application.TransitionIssue {
	t.Helper()
	return application.NewTransitionIssue(connectedRepo(t, "https://j", "t"), issues, client, fixedClock(syncAt))
}

func TestATransientRefreshFailureAfterTheTransitionIsRetried(t *testing.T) {
	client := newFakeClient()
	attempts := 0
	client.getIssueFn = func(key string) (models.RemoteIssue, error) {
		attempts++
		if attempts == 1 {
			return models.RemoteIssue{}, jiraerrors.Remote("timeout")
		}
		remote := sampleRemote(key)
		remote.Status = "Done"
		return remote, nil
	}
	issues := transitionIssues(models.IssueLocalStateNew)

	issue, err := newTransition(t, issues, client).Execute(context.Background(), "A-1", "31")

	require.NoError(t, err)
	assert.Equal(t, 2, client.calls["get_issue"])
	assert.Equal(t, "Done", issue.Remote.Status)
}

func TestAPersistentRefreshFailureStoresTheTargetStatusOfTheTransition(t *testing.T) {
	client := newFakeClient()
	client.transitionsFn = func(string) ([]models.Transition, error) {
		return []models.Transition{{ID: "21", Name: "Start", ToStatus: "In Progress"}, {ID: "31", Name: "Close", ToStatus: "Done"}}, nil
	}
	client.getIssueFn = func(string) (models.RemoteIssue, error) { return models.RemoteIssue{}, jiraerrors.Remote("timeout") }
	issues := transitionIssues(models.IssueLocalStateNew)

	issue, err := newTransition(t, issues, client).Execute(context.Background(), "A-1", "31")

	require.NoError(t, err)
	assert.Equal(t, "Done", issue.Remote.Status)
	require.Len(t, issues.updated, 1)
	assert.Equal(t, "Done", issues.updated[0].Status)
}

func TestTheTargetStatusLookupNeverBlocksTheTransition(t *testing.T) {
	client := newFakeClient()
	client.transitionsFn = func(string) ([]models.Transition, error) { return nil, jiraerrors.Remote("HTTP 500") }
	issues := transitionIssues(models.IssueLocalStateNew)

	_, err := newTransition(t, issues, client).Execute(context.Background(), "A-1", "31")

	require.NoError(t, err)
	assert.Equal(t, 1, client.calls["transition"])
}
