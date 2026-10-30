package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

var syncAt = time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)

func lane(id string, jql string) models.Lane {
	return models.Lane{ID: id, Name: "Carril " + id, JQL: jql}
}

func remote(key string) models.RemoteIssue {
	return models.RemoteIssue{Key: key, Summary: "Resumen de " + key}
}

func newSync(connections *fakeConnections, issues *fakeIssues, lanes *fakeLanes, client *fakeClient) *application.SyncIssues {
	return application.NewSyncIssues(connections, issues, lanes, client, fixedClock(syncAt))
}

func TestSyncImportsEachLaneWithItsOwnJQLAndRecordsSuccess(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{replaceAdded: 3}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "mine-jql"), lane("l2", "other-jql")}}
	client := newFakeClient()
	client.searchFn = func(jql string) ([]models.RemoteIssue, error) {
		if jql == "mine-jql" {
			return []models.RemoteIssue{remote("A-1"), remote("A-2")}, nil
		}
		return []models.RemoteIssue{remote("A-2"), remote("A-3")}, nil
	}

	outcome, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, outcome.Total)
	assert.Equal(t, 3, outcome.Added)
	assert.Empty(t, outcome.FailedLanes)
	require.Len(t, issues.replaced, 1)
	require.Len(t, issues.replaced[0], 2)
	assert.Equal(t, "mine", issues.replaced[0][0].LaneID)
	assert.Len(t, issues.replaced[0][0].Issues, 2)
	assert.Equal(t, "l2", issues.replaced[0][1].LaneID)
	require.Len(t, connections.syncs, 1)
	assert.Nil(t, connections.syncs[0].error)
	assert.Equal(t, syncAt, connections.syncs[0].at)
}

func TestAFailingLaneDoesNotBlockTheOthers(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{replaceAdded: 1}
	lanes := &fakeLanes{lanes: []models.Lane{lane("mine", "ok"), lane("l2", "broken")}}
	client := newFakeClient()
	client.searchFn = func(jql string) ([]models.RemoteIssue, error) {
		if jql == "broken" {
			return nil, jiraerrors.Remote("JQL inválido")
		}
		return []models.RemoteIssue{remote("A-1")}, nil
	}

	outcome, err := newSync(connections, issues, lanes, client).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, outcome.Total)
	require.Len(t, outcome.FailedLanes, 1)
	assert.True(t, strings.HasPrefix(outcome.FailedLanes[0], "«Carril l2»"))
	assert.Contains(t, outcome.FailedLanes[0], "JQL inválido")
	require.Len(t, issues.replaced[0], 1)
	assert.Equal(t, "mine", issues.replaced[0][0].LaneID)
	require.NotNil(t, connections.syncs[0].error)
	assert.Contains(t, *connections.syncs[0].error, "«Carril l2»")
}

func TestWhenEveryLaneFailsTheErrorIsRecordedAndReturnedWithoutTouchingIssues(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{}
	client := newFakeClient()
	client.searchFn = func(string) ([]models.RemoteIssue, error) { return nil, jiraerrors.Remote("caído") }

	_, err := newSync(connections, issues, &fakeLanes{lanes: []models.Lane{lane("mine", "x")}}, client).
		Execute(context.Background())

	require.ErrorIs(t, err, jiraerrors.Remote(""))
	require.EqualError(t, err, "no se pudo contactar con Jira: caído")
	assert.Empty(t, issues.replaced)
	require.Len(t, connections.syncs, 1)
	require.NotNil(t, connections.syncs[0].error)
	assert.Contains(t, *connections.syncs[0].error, "caído")
}

func TestWithoutAConnectionNothingIsCalled(t *testing.T) {
	client := newFakeClient()
	lanes := &fakeLanes{}
	issues := &fakeIssues{}

	_, err := newSync(&fakeConnections{}, issues, lanes, client).Execute(context.Background())

	require.ErrorIs(t, err, jiraerrors.NotConfigured())
	assert.Empty(t, client.calls)
	assert.Empty(t, issues.replaced)
}

func TestABookkeepingFailureAfterStoringTheIssuesDoesNotFailTheSync(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	connections.recordErr = jiraerrors.Repository("bloqueada")
	issues := &fakeIssues{replaceAdded: 1}
	client := newFakeClient()
	client.searchFn = func(string) ([]models.RemoteIssue, error) { return []models.RemoteIssue{remote("A-1")}, nil }

	outcome, err := newSync(connections, issues, &fakeLanes{lanes: []models.Lane{lane("mine", "x")}}, client).
		Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, outcome.Added)
}

func TestSyncingOneLaneOnlyQueriesThatLane(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")
	issues := &fakeIssues{replaceAdded: 1}
	lanes := &fakeLanes{lanes: []models.Lane{lane("l2", "lane-jql"), lane("other", "other-jql")}}
	client := newFakeClient()
	client.searchFn = func(string) ([]models.RemoteIssue, error) { return []models.RemoteIssue{remote("B-1")}, nil }

	outcome, err := newSync(connections, issues, lanes, client).ExecuteLane(context.Background(), "l2")

	require.NoError(t, err)
	assert.Equal(t, 1, outcome.Total)
	assert.Equal(t, []string{"lane-jql"}, client.searchedJQL)
	require.Len(t, issues.replaced[0], 1)
	assert.Equal(t, "l2", issues.replaced[0][0].LaneID)
}

func TestSyncingAnUnknownLaneFails(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "t")

	_, err := newSync(connections, &fakeIssues{}, &fakeLanes{}, newFakeClient()).ExecuteLane(context.Background(), "nope")

	require.ErrorIs(t, err, jiraerrors.LaneNotFound(""))
	require.EqualError(t, err, "carril no encontrado: nope")
}
