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

func TestGetTaskRunTimeSumsTheWorkingIntervalsOfTheTask(t *testing.T) {
	base := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	repo.storedEvents = []models.TaskEvent{
		{Kind: models.AgentEventKind{Event: models.RunStarted{}}, CreatedAt: base},
		{Kind: models.AgentEventKind{Event: models.RunSettled{}}, CreatedAt: base.Add(12 * time.Minute)},
	}
	use := application.NewGetTaskRunTime(repo, func() time.Time { return base.Add(time.Hour) })

	got, err := use.Execute(context.Background(), newTaskIDValue(t))

	require.NoError(t, err)
	assert.Equal(t, 12*time.Minute, got)
	assert.Nil(t, repo.eventsAfter, "the whole history is read")
}
