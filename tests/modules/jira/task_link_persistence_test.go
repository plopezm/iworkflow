package jira_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/domain/models"
)

func TestInsertRemoteStoresAnIssueNoLaneListsAndKeepsAKnownOne(t *testing.T) {
	_, issues, _ := newConnectionRepo(t)
	ctx := context.Background()

	require.NoError(t, issues.InsertRemote(ctx, remote("ABC-1", "Open", 0), storedNow))
	require.NoError(t, issues.InsertRemote(ctx, remote("ABC-1", "Done", 10), storedNow))

	found, err := issues.Find(ctx, "ABC-1")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "Open", found.Remote.Status, "a known issue is left untouched")
	assert.Equal(t, models.IssueLocalStateNew, found.LocalState)
}

func TestFindByTaskIDReturnsTheLinkedIssueOnly(t *testing.T) {
	_, issues, _ := newConnectionRepo(t)
	ctx := context.Background()
	require.NoError(t, issues.InsertRemote(ctx, remote("ABC-1", "Open", 0), storedNow))
	require.NoError(t, issues.InsertRemote(ctx, remote("ABC-2", "Open", 0), storedNow))
	taskID := "task-1"
	require.NoError(t, issues.SetLocalState(ctx, "ABC-2", models.IssueLocalStateConverted, &taskID))

	linked, err := issues.FindByTaskID(ctx, "task-1")
	require.NoError(t, err)
	require.NotNil(t, linked)
	assert.Equal(t, "ABC-2", linked.Key())

	none, err := issues.FindByTaskID(ctx, "task-2")
	require.NoError(t, err)
	assert.Nil(t, none)
}
