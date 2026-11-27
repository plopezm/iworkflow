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

func updatedStatuses(issues *fakeIssues) map[string]string {
	statuses := map[string]string{}
	for _, updated := range issues.updated {
		statuses[updated.Key] = updated.Status
	}
	return statuses
}

func keyQueries(client *fakeClient) []string {
	var queries []string
	for _, jql := range client.searchedJQL {
		if strings.HasPrefix(jql, "key in (") {
			queries = append(queries, jql)
		}
	}
	return queries
}

func TestSyncSkipsMalformedStoredKeysButStillQueriesTheValidOnes(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{issues: []models.Issue{
		convertedIssue(`A-1") OR project = X OR key in ("B-2`, "In Progress"),
		convertedIssue("not a key", "In Progress"),
		convertedIssue("A-2", "In Progress"),
	}}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = byKeySearch(nil, map[string]models.RemoteIssue{"A-2": {Key: "A-2", Status: "Done"}})

	_, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{`key in ("A-2")`}, keyQueries(client))
	assert.Equal(t, map[string]string{"A-2": "Done"}, updatedStatuses(issues))
}

func TestARejectedBatchIsSplitSoOneBadKeyDoesNotBlockTheOthers(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{}
	jira := map[string]models.RemoteIssue{}
	for _, key := range []string{"A-1", "A-2", "A-3", "A-4", "A-5", "A-6"} {
		issues.issues = append(issues.issues, convertedIssue(key, "In Progress"))
		jira[key] = models.RemoteIssue{Key: key, Status: "Done"}
	}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	answer := byKeySearch(nil, jira)
	client.searchFn = func(jql string) ([]models.RemoteIssue, error) {
		if strings.Contains(jql, `"A-4"`) {
			return nil, jiraerrors.Remote("la issue A-4 no existe")
		}
		return answer(jql)
	}

	outcome, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Empty(t, outcome.FailedLanes)
	assert.Equal(t, map[string]string{"A-1": "Done", "A-2": "Done", "A-3": "Done", "A-5": "Done", "A-6": "Done"}, updatedStatuses(issues))
}

func TestSplittingARejectedBatchIsBounded(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{}
	for index := range 100 {
		issues.issues = append(issues.issues, convertedIssue(fmt.Sprintf("A-%d", index+1), "In Progress"))
	}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = func(jql string) ([]models.RemoteIssue, error) {
		if strings.HasPrefix(jql, "key in (") {
			return nil, jiraerrors.Remote("Jira caído")
		}
		return nil, nil
	}

	_, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.LessOrEqual(t, len(keyQueries(client)), 33, "a failing Jira must not be hit once per key")
	assert.Empty(t, issues.updated)
}

// TestAReopenedLinkedIssueOutsideEveryLaneKeepsShowingDone pins a known tradeoff: issues stored as Done are not
// asked for again, so one reopened in Jira that no lane matches shows Done until a lane matches it or it is
// refreshed by hand from the task panel.
func TestAReopenedLinkedIssueOutsideEveryLaneKeepsShowingDone(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{issues: []models.Issue{convertedIssue("A-9", "Done")}}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql")}}
	client := newFakeClient()
	client.searchFn = byKeySearch(nil, map[string]models.RemoteIssue{"A-9": {Key: "A-9", Status: "In Progress"}})

	_, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Empty(t, keyQueries(client))
	found, err := issues.Find(context.Background(), "A-9")
	require.NoError(t, err)
	assert.Equal(t, "Done", found.Remote.Status)
}
