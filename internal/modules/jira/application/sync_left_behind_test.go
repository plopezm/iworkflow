package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

// byKeySearch answers the lane query with the given lane issues and a key query with the given Jira copies.
func byKeySearch(laneIssues []models.RemoteIssue, jira map[string]models.RemoteIssue) func(string) ([]models.RemoteIssue, error) {
	return func(jql string) ([]models.RemoteIssue, error) {
		if !strings.HasPrefix(jql, "key in (") {
			return laneIssues, nil
		}
		var found []models.RemoteIssue
		for key, issue := range jira {
			if strings.Contains(jql, `"`+key+`"`) {
				found = append(found, issue)
			}
		}
		return found, nil
	}
}

func convertedIssue(key string, status string) models.Issue {
	taskID := "task-" + key
	return models.Issue{Remote: models.RemoteIssue{Key: key, Status: status}, LocalState: models.IssueLocalStateConverted, TaskID: &taskID}
}

func TestSyncRefreshesLinkedIssuesThatLeftEveryLaneWithOneKeyQuery(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{issues: []models.Issue{
		convertedIssue("A-9", "In Progress"),
		convertedIssue("A-7", "To Do"),
		{Remote: models.RemoteIssue{Key: "A-8", Status: "To Do"}, LocalState: models.IssueLocalStateNew},
		convertedIssue("A-1", "To Do"),
	}}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = byKeySearch([]models.RemoteIssue{remote("A-1")}, map[string]models.RemoteIssue{
		"A-9": {Key: "A-9", Status: "Done"},
		"A-7": {Key: "A-7", Status: "In Review"},
	})

	_, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, client.searchedJQL, 2, "one lane query plus one key query")
	assert.Equal(t, `key in ("A-9","A-7")`, client.searchedJQL[1])
	assert.Zero(t, client.calls["get_issue"])
	require.Len(t, issues.updated, 2)
	statuses := map[string]string{}
	for _, updated := range issues.updated {
		statuses[updated.Key] = updated.Status
	}
	assert.Equal(t, map[string]string{"A-9": "Done", "A-7": "In Review"}, statuses)
}

func TestSyncDoesNotAskAgainForLinkedIssuesAlreadyDone(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{issues: []models.Issue{convertedIssue("A-9", "Done"), convertedIssue("A-6", "Hecho")}}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = byKeySearch(nil, nil)

	_, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"mine-jql"}, client.searchedJQL)
	assert.Empty(t, issues.updated)
}

func TestSyncSplitsTheKeyQueryInBatchesOf100(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{}
	for index := range 101 {
		issues.issues = append(issues.issues, convertedIssue(fmt.Sprintf("A-%d", index+1), "In Progress"))
	}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = byKeySearch(nil, nil)

	_, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, client.searchedJQL, 3, "one lane query plus two key queries")
	assert.Equal(t, 100, strings.Count(client.searchedJQL[1], `"A-`))
	assert.Equal(t, 1, strings.Count(client.searchedJQL[2], `"A-`))
}

func TestSyncLeavesALinkedIssueUntouchedWhenJiraDoesNotReturnIt(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{issues: []models.Issue{convertedIssue("A-9", "In Progress")}}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = byKeySearch(nil, nil)

	_, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Empty(t, issues.updated)
	found, err := issues.Find(context.Background(), "A-9")
	require.NoError(t, err)
	assert.Equal(t, "In Progress", found.Remote.Status)
}

func TestSyncKeepsGoingWhenTheKeyQueryFails(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{issues: []models.Issue{convertedIssue("A-9", "In Progress")}}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = func(jql string) ([]models.RemoteIssue, error) {
		if strings.HasPrefix(jql, "key in (") {
			return nil, jiraerrors.Remote("boom")
		}
		return nil, nil
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
