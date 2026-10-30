package jira_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/persistence"
)

// storedNow is the clock of the repositories under test; it truncates to the storage precision.
var storedNow = time.Date(2024, 5, 1, 10, 0, 0, 123456000, time.UTC)

func remote(key string, status string, updatedSeconds int64) models.RemoteIssue {
	priority := "High"
	reporter := "Luis"
	return models.RemoteIssue{
		Key:           key,
		Summary:       "resumen " + key,
		Description:   "desc",
		Status:        status,
		IssueType:     "Bug",
		Priority:      &priority,
		Reporter:      &reporter,
		Labels:        []string{"a", "b"},
		URL:           "https://jira/browse/" + key,
		JiraUpdatedAt: time.Unix(1_700_000_000+updatedSeconds, 0).UTC(),
	}
}

func mine(issues ...models.RemoteIssue) []models.LaneBatch {
	return []models.LaneBatch{{LaneID: models.DefaultLaneID, Issues: issues}}
}

func newConnectionRepo(t *testing.T) (*persistence.ConnectionRepository, *persistence.IssueRepository, *persistence.LaneRepository) {
	t.Helper()
	db := dbtest.New(t)
	clock := func() time.Time { return storedNow }
	ids := 0
	newID := func() string {
		ids++
		return fmt.Sprintf("lane-%d", ids)
	}
	return persistence.NewConnectionRepository(db, clock),
		persistence.NewIssueRepository(db),
		persistence.NewLaneRepository(db, clock, newID)
}

func connectionFor(t *testing.T, url string) models.Connection {
	t.Helper()
	connection, err := models.NewConnection(url, "secret", true)
	require.NoError(t, err)
	return connection
}

func mustDefinition(t *testing.T, name string, jql string) models.LaneDefinition {
	t.Helper()
	definition, err := models.NewLaneDefinition(name, jql)
	require.NoError(t, err)
	return definition
}

func keysOfIssues(t *testing.T, repo outbound.IssueRepository) []string {
	t.Helper()
	issues, err := repo.List(context.Background())
	require.NoError(t, err)
	keys := make([]string, 0, len(issues))
	for _, issue := range issues {
		keys = append(keys, issue.Key())
	}
	return keys
}

func TestConnectionIsSavedReadAndDeleted(t *testing.T) {
	connections, _, _ := newConnectionRepo(t)
	ctx := context.Background()

	stored, err := connections.Get(ctx)
	require.NoError(t, err)
	assert.Nil(t, stored)

	original := connectionFor(t, "https://jira.local")
	require.NoError(t, connections.Save(ctx, original))
	stored, err = connections.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "https://jira.local", stored.BaseURL())
	assert.Equal(t, "secret", stored.Token())
	assert.True(t, stored.AcceptInvalidCerts())

	replaced, err := models.NewConnection("https://jira.local", "other", false)
	require.NoError(t, err)
	require.NoError(t, connections.Save(ctx, replaced))
	stored, err = connections.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "other", stored.Token())
	assert.False(t, stored.AcceptInvalidCerts())

	require.NoError(t, connections.Delete(ctx))
	stored, err = connections.Get(ctx)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

func TestSyncBookkeepingSurvivesEditsButNotAURLChange(t *testing.T) {
	connections, _, _ := newConnectionRepo(t)
	ctx := context.Background()
	require.NoError(t, connections.Save(ctx, connectionFor(t, "https://jira.local")))
	at := storedNow
	failure := "boom"
	require.NoError(t, connections.RecordSync(ctx, at, &failure))

	stored, err := connections.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stored.LastSyncAt())
	assert.True(t, at.Equal(*stored.LastSyncAt()))
	assert.Equal(t, "boom", *stored.LastSyncError())

	edited, err := models.NewConnection("https://jira.local", "secret", false)
	require.NoError(t, err)
	require.NoError(t, connections.Save(ctx, edited))
	kept, err := connections.Get(ctx)
	require.NoError(t, err)
	assert.False(t, kept.AcceptInvalidCerts())
	require.NotNil(t, kept.LastSyncAt())
	assert.True(t, at.Equal(*kept.LastSyncAt()))

	require.NoError(t, connections.Save(ctx, connectionFor(t, "https://other.local")))
	reset, err := connections.Get(ctx)
	require.NoError(t, err)
	assert.Nil(t, reset.LastSyncAt())
	assert.Nil(t, reset.LastSyncError())
}

func TestReplaceSyncedInsertsListsNewestFirstAndFinds(t *testing.T) {
	_, issues, _ := newConnectionRepo(t)
	ctx := context.Background()

	added, err := issues.ReplaceSynced(ctx, mine(remote("A-1", "To Do", 10), remote("A-2", "Done", 20)), storedNow)
	require.NoError(t, err)
	assert.Equal(t, 2, added)
	assert.Equal(t, []string{"A-2", "A-1"}, keysOfIssues(t, issues))

	found, err := issues.Find(ctx, "A-1")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, remote("A-1", "To Do", 10), found.Remote)
	assert.Equal(t, models.IssueLocalStateNew, found.LocalState)
	assert.Nil(t, found.TaskID)
	assert.True(t, storedNow.Equal(found.SyncedAt))

	missing, err := issues.Find(ctx, "NOPE-1")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestARepeatedKeyInOneBatchDoesNotAbortTheSync(t *testing.T) {
	_, issues, _ := newConnectionRepo(t)
	ctx := context.Background()

	added, err := issues.ReplaceSynced(ctx, mine(remote("A-1", "To Do", 10), remote("A-1", "Done", 20), remote("A-2", "To Do", 30)), storedNow)

	require.NoError(t, err)
	assert.Equal(t, 2, added)
	assert.Len(t, keysOfIssues(t, issues), 2)
}

func TestResyncRefreshesJiraDataAndKeepsLocalState(t *testing.T) {
	_, issues, _ := newConnectionRepo(t)
	ctx := context.Background()
	_, err := issues.ReplaceSynced(ctx, mine(remote("A-1", "To Do", 10)), storedNow)
	require.NoError(t, err)
	task := "task-9"
	require.NoError(t, issues.SetLocalState(ctx, "A-1", models.IssueLocalStateConverted, &task))

	added, err := issues.ReplaceSynced(ctx, mine(remote("A-1", "In Progress", 30)), storedNow)

	require.NoError(t, err)
	assert.Equal(t, 0, added)
	found, err := issues.Find(ctx, "A-1")
	require.NoError(t, err)
	assert.Equal(t, "In Progress", found.Remote.Status)
	assert.Equal(t, models.IssueLocalStateConverted, found.LocalState)
	require.NotNil(t, found.TaskID)
	assert.Equal(t, "task-9", *found.TaskID)
}

func TestResyncPrunesOnlyNewIssuesMissingFromJira(t *testing.T) {
	_, issues, _ := newConnectionRepo(t)
	ctx := context.Background()
	_, err := issues.ReplaceSynced(ctx, mine(remote("A-1", "To Do", 1), remote("A-2", "To Do", 2), remote("A-3", "To Do", 3)), storedNow)
	require.NoError(t, err)
	require.NoError(t, issues.SetLocalState(ctx, "A-2", models.IssueLocalStateIgnored, nil))
	task := "t"
	require.NoError(t, issues.SetLocalState(ctx, "A-3", models.IssueLocalStateConverted, &task))

	_, err = issues.ReplaceSynced(ctx, mine(), storedNow)

	require.NoError(t, err)
	assert.Equal(t, []string{"A-3", "A-2"}, keysOfIssues(t, issues))
}

func TestSetLocalStateClearsTheTaskLinkAndReportsUnknownKeys(t *testing.T) {
	_, issues, _ := newConnectionRepo(t)
	ctx := context.Background()
	_, err := issues.ReplaceSynced(ctx, mine(remote("A-1", "To Do", 1)), storedNow)
	require.NoError(t, err)
	task := "t"
	require.NoError(t, issues.SetLocalState(ctx, "A-1", models.IssueLocalStateConverted, &task))
	require.NoError(t, issues.SetLocalState(ctx, "A-1", models.IssueLocalStateNew, nil))

	found, err := issues.Find(ctx, "A-1")
	require.NoError(t, err)
	assert.Equal(t, models.IssueLocalStateNew, found.LocalState)
	assert.Nil(t, found.TaskID)

	require.ErrorIs(t, issues.SetLocalState(ctx, "X-9", models.IssueLocalStateIgnored, nil), jiraerrors.IssueNotFound(""))
	require.EqualError(t, issues.UpdateRemote(ctx, remote("X-9", "Done", 1), storedNow), "tarea de Jira no encontrada: X-9")
}

func TestUpdateRemoteRefreshesOneIssueAndDeleteAllClears(t *testing.T) {
	_, issues, _ := newConnectionRepo(t)
	ctx := context.Background()
	_, err := issues.ReplaceSynced(ctx, mine(remote("A-1", "To Do", 1)), storedNow)
	require.NoError(t, err)

	require.NoError(t, issues.UpdateRemote(ctx, remote("A-1", "Done", 5), storedNow))
	found, err := issues.Find(ctx, "A-1")
	require.NoError(t, err)
	assert.Equal(t, "Done", found.Remote.Status)

	require.NoError(t, issues.DeleteAll(ctx))
	assert.Empty(t, keysOfIssues(t, issues))
}

func TestTheDefaultLaneIsSeededAndCannotBeChanged(t *testing.T) {
	_, _, lanes := newConnectionRepo(t)
	ctx := context.Background()

	listed, err := lanes.List(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	defaultLane := listed[0]
	assert.Equal(t, models.DefaultLaneID, defaultLane.ID)
	assert.True(t, defaultLane.IsDefault)
	assert.False(t, defaultLane.Collapsed)
	assert.Contains(t, defaultLane.JQL, "currentUser()")
	assert.Contains(t, defaultLane.JQL, `statusCategory in ("To Do", "In Progress")`)
	assert.NotContains(t, defaultLane.JQL, "Done")

	_, err = lanes.Update(ctx, models.DefaultLaneID, mustDefinition(t, "x", "a = b"))
	require.ErrorIs(t, err, jiraerrors.DefaultLaneImmutable())
	require.ErrorIs(t, lanes.Delete(ctx, models.DefaultLaneID), jiraerrors.DefaultLaneImmutable())

	// Folding is allowed for every lane, the built-in one included.
	require.NoError(t, lanes.SetCollapsed(ctx, models.DefaultLaneID, true))
	found, err := lanes.Find(ctx, models.DefaultLaneID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Collapsed)
}

func TestCustomLanesAreCreatedInOrderEditedCollapsedAndDeleted(t *testing.T) {
	_, _, lanes := newConnectionRepo(t)
	ctx := context.Background()

	first, err := lanes.Create(ctx, mustDefinition(t, "Backend", "project = B"))
	require.NoError(t, err)
	second, err := lanes.Create(ctx, mustDefinition(t, "Bugs", "type = Bug"))
	require.NoError(t, err)
	assert.False(t, first.IsDefault)
	assert.False(t, first.Collapsed)

	listed, err := lanes.List(ctx)
	require.NoError(t, err)
	names := make([]string, 0, len(listed))
	for _, lane := range listed {
		names = append(names, lane.Name)
	}
	assert.Equal(t, []string{"Mis tareas", "Backend", "Bugs"}, names)

	edited, err := lanes.Update(ctx, first.ID, mustDefinition(t, "Back", "project = BE"))
	require.NoError(t, err)
	assert.Equal(t, "Back", edited.Name)
	assert.Equal(t, "project = BE", edited.JQL)

	require.NoError(t, lanes.SetCollapsed(ctx, second.ID, true))
	found, err := lanes.Find(ctx, second.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Collapsed)
	require.EqualError(t, lanes.SetCollapsed(ctx, "nope", true), "carril no encontrado: nope")

	require.NoError(t, lanes.Delete(ctx, first.ID))
	gone, err := lanes.Find(ctx, first.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
	_, err = lanes.Update(ctx, first.ID, mustDefinition(t, "x", "a = b"))
	require.EqualError(t, err, "carril no encontrado: "+first.ID)
}

func TestAnIssueCanBelongToSeveralLanesAndASyncOnlyReplacesTheLanesItCarries(t *testing.T) {
	_, issues, lanes := newConnectionRepo(t)
	ctx := context.Background()
	custom, err := lanes.Create(ctx, mustDefinition(t, "Bugs", "type = Bug"))
	require.NoError(t, err)

	both := []models.LaneBatch{
		{LaneID: models.DefaultLaneID, Issues: []models.RemoteIssue{remote("A-1", "To Do", 1), remote("A-2", "To Do", 2)}},
		{LaneID: custom.ID, Issues: []models.RemoteIssue{remote("A-2", "To Do", 2), remote("B-1", "To Do", 3)}},
	}
	added, err := issues.ReplaceSynced(ctx, both, storedNow)
	require.NoError(t, err)
	assert.Equal(t, 3, added)

	memberships, err := issues.LaneMemberships(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"A-1", "A-2"}, memberships[models.DefaultLaneID])
	assert.ElementsMatch(t, []string{"A-2", "B-1"}, memberships[custom.ID])

	// Re-syncing only the custom lane keeps the default one as it was.
	_, err = issues.ReplaceSynced(ctx, []models.LaneBatch{{LaneID: custom.ID, Issues: []models.RemoteIssue{remote("B-2", "To Do", 4)}}}, storedNow)
	require.NoError(t, err)
	memberships, err = issues.LaneMemberships(ctx)
	require.NoError(t, err)
	assert.Len(t, memberships[models.DefaultLaneID], 2)
	assert.Equal(t, []string{"B-2"}, memberships[custom.ID])
	keys := keysOfIssues(t, issues)
	// B-1 was only in the custom lane and is New: retired. A-2 survives through the default lane.
	assert.NotContains(t, keys, "B-1")
	assert.Contains(t, keys, "A-2")
	assert.Contains(t, keys, "B-2")
}

func TestDeletingALaneRetiresItsNewIssuesButKeepsSharedAndConvertedOnes(t *testing.T) {
	_, issues, lanes := newConnectionRepo(t)
	ctx := context.Background()
	custom, err := lanes.Create(ctx, mustDefinition(t, "Bugs", "type = Bug"))
	require.NoError(t, err)
	_, err = issues.ReplaceSynced(ctx, []models.LaneBatch{
		{LaneID: models.DefaultLaneID, Issues: []models.RemoteIssue{remote("A-1", "To Do", 1)}},
		{LaneID: custom.ID, Issues: []models.RemoteIssue{remote("A-1", "To Do", 1), remote("B-1", "To Do", 2), remote("B-2", "To Do", 3)}},
	}, storedNow)
	require.NoError(t, err)
	task := "t"
	require.NoError(t, issues.SetLocalState(ctx, "B-2", models.IssueLocalStateConverted, &task))

	require.NoError(t, lanes.Delete(ctx, custom.ID))

	assert.ElementsMatch(t, []string{"A-1", "B-2"}, keysOfIssues(t, issues))
	memberships, err := issues.LaneMemberships(ctx)
	require.NoError(t, err)
	assert.NotContains(t, memberships, custom.ID)
}
