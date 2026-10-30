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

// laneSync is a real SyncIssues over the fakes: lane use cases sync through it.
func laneSync(t *testing.T, lanes *fakeLanes, client *fakeClient) *application.SyncIssues {
	t.Helper()
	return application.NewSyncIssues(connectedRepo(t, "https://jira.local", "t"), &fakeIssues{}, lanes, client, fixedClock(syncAt))
}

func TestCreateLaneStoresTheTrimmedDefinitionAndSyncsTheNewLane(t *testing.T) {
	lanes := &fakeLanes{lanes: []models.Lane{{ID: "mine", IsDefault: true}}}
	client := newFakeClient()
	client.searchFn = func(string) ([]models.RemoteIssue, error) { return []models.RemoteIssue{remote("B-1")}, nil }

	saved, err := application.NewCreateLane(lanes, laneSync(t, lanes, client)).Execute(context.Background(), " Backend ", " project = B ")

	require.NoError(t, err)
	require.Len(t, lanes.created, 1)
	assert.Equal(t, "Backend", lanes.created[0].Name())
	assert.Equal(t, "project = B", lanes.created[0].JQL())
	assert.Equal(t, "new", saved.Lane.ID)
	assert.Nil(t, saved.SyncError)
	assert.Equal(t, []string{"project = B"}, client.searchedJQL)
}

func TestAFailingFirstSyncKeepsTheLaneAndReportsTheError(t *testing.T) {
	lanes := &fakeLanes{}
	client := newFakeClient()
	client.searchFn = func(string) ([]models.RemoteIssue, error) { return nil, jiraerrors.Remote("JQL inválido") }

	saved, err := application.NewCreateLane(lanes, laneSync(t, lanes, client)).Execute(context.Background(), "Backend", "bad jql")

	require.NoError(t, err)
	assert.Len(t, lanes.created, 1)
	require.NotNil(t, saved.SyncError)
	assert.Contains(t, *saved.SyncError, "JQL inválido")
}

func TestInvalidInputStoresNothing(t *testing.T) {
	lanes := &fakeLanes{}

	_, err := application.NewCreateLane(lanes, laneSync(t, lanes, newFakeClient())).Execute(context.Background(), " ", "x = y")

	require.ErrorIs(t, err, jiraerrors.EmptyLaneName())
	assert.Empty(t, lanes.created)
}

func TestAChangedQueryIsSynced(t *testing.T) {
	lanes := &fakeLanes{lanes: []models.Lane{{ID: "l1", Name: "Viejo", JQL: "project = OLD"}}}
	client := newFakeClient()

	saved, err := application.NewUpdateLane(lanes, laneSync(t, lanes, client)).Execute(context.Background(), "l1", "Nuevo", "project = NEW")

	require.NoError(t, err)
	assert.Equal(t, "Nuevo", saved.Lane.Name)
	assert.Equal(t, []string{"project = NEW"}, client.searchedJQL)
	assert.Nil(t, saved.SyncError)
}

func TestARenameAloneDoesNotHitJira(t *testing.T) {
	lanes := &fakeLanes{lanes: []models.Lane{{ID: "l1", Name: "Viejo", JQL: "project = X"}}}
	client := newFakeClient()

	saved, err := application.NewUpdateLane(lanes, laneSync(t, lanes, client)).Execute(context.Background(), "l1", "Otro nombre", "project = X")

	require.NoError(t, err)
	assert.Equal(t, "Otro nombre", saved.Lane.Name)
	assert.Empty(t, client.searchedJQL)
}

func TestTheDefaultLaneCannotBeEdited(t *testing.T) {
	lanes := &fakeLanes{lanes: []models.Lane{{ID: "mine", IsDefault: true}}}

	_, err := application.NewUpdateLane(lanes, laneSync(t, lanes, newFakeClient())).Execute(context.Background(), "mine", "x", "a = b")

	require.ErrorIs(t, err, jiraerrors.DefaultLaneImmutable())
	assert.Empty(t, lanes.updated)
}

func TestAnUnknownLaneIsReportedOnUpdate(t *testing.T) {
	lanes := &fakeLanes{}

	_, err := application.NewUpdateLane(lanes, laneSync(t, lanes, newFakeClient())).Execute(context.Background(), "zz", "x", "a = b")

	require.EqualError(t, err, "carril no encontrado: zz")
}

func TestDeleteLaneDeletesACustomLane(t *testing.T) {
	lanes := &fakeLanes{lanes: []models.Lane{{ID: "l1"}}}

	require.NoError(t, application.NewDeleteLane(lanes).Execute(context.Background(), "l1"))

	assert.Equal(t, []string{"l1"}, lanes.deleted)
}

func TestDeleteLaneRefusesTheDefaultLaneAndUnknownIDs(t *testing.T) {
	lanes := &fakeLanes{lanes: []models.Lane{{ID: "mine", IsDefault: true}}}
	useCase := application.NewDeleteLane(lanes)

	require.ErrorIs(t, useCase.Execute(context.Background(), "mine"), jiraerrors.DefaultLaneImmutable())
	require.EqualError(t, useCase.Execute(context.Background(), "zz"), "carril no encontrado: zz")
	assert.Empty(t, lanes.deleted)
}

func TestSetLaneCollapsedForwardsTheFlag(t *testing.T) {
	lanes := &fakeLanes{}

	require.NoError(t, application.NewSetLaneCollapsed(lanes).Execute(context.Background(), "mine", true))

	assert.True(t, lanes.collapsed["mine"])
}
