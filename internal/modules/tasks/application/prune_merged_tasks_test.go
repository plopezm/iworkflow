package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

func TestPruneMergedTasksDeletesOnlyMergedTasksOlderThanOneWeek(t *testing.T) {
	now := fixedNow.Add(30 * 24 * time.Hour)
	old := newTask(models.StatusMerged)
	old.UpdatedAt = now.Add(-application.MergedTaskRetention - time.Minute)
	recent := newTask(models.StatusMerged)
	recent.UpdatedAt = now.Add(-application.MergedTaskRetention + time.Minute)
	oldDiscarded := newTask(models.StatusDiscarded)
	oldDiscarded.UpdatedAt = old.UpdatedAt
	oldBacklog := newTask(models.StatusBacklog)
	oldBacklog.UpdatedAt = old.UpdatedAt
	repo := newFakeRepository(old, recent, oldDiscarded, oldBacklog)
	storage := &fakeStorage{}
	lifecycle, publisher := newLifecycle(repo)

	pruned, err := application.NewPruneMergedTasks(lifecycle, storage).Execute(context.Background(), now)

	require.NoError(t, err)
	assert.Equal(t, 1, pruned)
	assert.NotContains(t, repo.tasks, old.ID)
	assert.Contains(t, repo.tasks, recent.ID)
	assert.Contains(t, repo.tasks, oldDiscarded.ID)
	assert.Contains(t, repo.tasks, oldBacklog.ID)
	assert.Equal(t, []models.TaskID{old.ID}, storage.deletedDirs, "the attachment folder of the pruned task is removed")
	assert.Len(t, publisher.published, 1, "the pruned task is announced to the board")
}

func TestPruneMergedTasksIsIdleWhenNothingIsOldEnough(t *testing.T) {
	recent := newTask(models.StatusMerged)
	repo := newFakeRepository(recent)
	lifecycle, publisher := newLifecycle(repo)

	pruned, err := application.NewPruneMergedTasks(lifecycle, &fakeStorage{}).Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Zero(t, pruned)
	assert.Contains(t, repo.tasks, recent.ID)
	assert.Empty(t, publisher.published)
}

func TestPruneMergedTasksAttachmentCleanupFailureDoesNotFail(t *testing.T) {
	now := fixedNow.Add(30 * 24 * time.Hour)
	old := newTask(models.StatusMerged)
	repo := newFakeRepository(old)
	lifecycle, _ := newLifecycle(repo)

	pruned, err := application.NewPruneMergedTasks(lifecycle, &fakeStorage{failDeleteDir: true}).Execute(context.Background(), now)

	require.NoError(t, err)
	assert.Equal(t, 1, pruned)
}
