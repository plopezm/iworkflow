package jira_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/persistence"
)

func TestWorklogRepositoryStoresAndListsPerTaskOldestFirst(t *testing.T) {
	repo := persistence.NewWorklogRepository(dbtest.New(t))
	ctx := context.Background()
	started := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	first := models.WorklogRecord{TaskID: "t1", IssueKey: "A-1", JiraID: "10", Seconds: 3600, Comment: "uno", StartedAt: started, LoggedAt: started.Add(time.Hour)}
	second := models.WorklogRecord{TaskID: "t1", IssueKey: "A-1", JiraID: "11", Seconds: 900, Comment: "dos", StartedAt: started.Add(time.Hour), LoggedAt: started.Add(2 * time.Hour)}
	other := models.WorklogRecord{TaskID: "t2", IssueKey: "A-2", JiraID: "12", Seconds: 60, Comment: "otro", StartedAt: started, LoggedAt: started}
	for _, record := range []models.WorklogRecord{first, second, other} {
		require.NoError(t, repo.Add(ctx, record))
	}

	got, err := repo.ListByTask(ctx, "t1")

	require.NoError(t, err)
	assert.Equal(t, []models.WorklogRecord{first, second}, got)
	none, err := repo.ListByTask(ctx, "missing")
	require.NoError(t, err)
	assert.Empty(t, none)
}
