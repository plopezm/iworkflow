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

func dialogRequested(id string, timeoutMS *uint64) models.AgentEvent {
	return models.DialogRequested{Request: models.DialogRequest{
		ID:        id,
		Title:     "título",
		Kind:      models.ConfirmDialog{},
		TimeoutMS: timeoutMS,
	}}
}

func TestDialogRequestedAddsPendingDialogBeforeEmitting(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	repo := newFakeRepository(task)
	lifecycle, publisher := newLifecycle(repo)

	require.NoError(t, lifecycle.Record(context.Background(), task.ID, dialogRequested("d1", nil)))

	require.Len(t, repo.dialogAdds, 1)
	assert.Equal(t, "d1", repo.dialogAdds[0].ID)
	assert.Nil(t, repo.dialogAdds[0].ExpiresAt)
	assert.Len(t, publisher.published, 1)
}

func TestDialogRequestedComputesExpiryFromTimeout(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)
	timeout := uint64(1000)

	require.NoError(t, lifecycle.Record(context.Background(), task.ID, dialogRequested("d1", &timeout)))

	require.Len(t, repo.dialogAdds, 1)
	require.NotNil(t, repo.dialogAdds[0].ExpiresAt)
	assert.True(t, repo.dialogAdds[0].ExpiresAt.Equal(fixedNow.Add(time.Second)))
}

func TestDialogResolvedRemovesPendingDialog(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning, "d1")
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)

	require.NoError(t, lifecycle.Record(context.Background(), task.ID, models.DialogResolved{DialogID: "d1"}))

	assert.Equal(t, []string{"d1"}, repo.dialogRemoves)
}

func TestSubagentChangesArePersistedAndPublished(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	repo := newFakeRepository(task)
	lifecycle, publisher := newLifecycle(repo)

	require.NoError(t, lifecycle.Record(context.Background(), task.ID, models.SubagentsChanged{Active: 2, Waiting: 1}))

	assert.Equal(t, []models.SubagentActivity{models.NewSubagentActivity(2, 1)}, repo.subagents)
	assert.Len(t, publisher.published, 1)
}

func TestUnrelatedAgentEventsDoNotTouchPendingDialogs(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)

	require.NoError(t, lifecycle.Record(context.Background(), task.ID, models.RunStarted{}))

	assert.Empty(t, repo.dialogAdds)
	assert.Empty(t, repo.dialogRemoves)
	assert.Zero(t, repo.dialogClears)
}

func TestTransitionToStatusWithoutLiveAgentClearsPendingDialogs(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning, "d1")
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)

	require.NoError(t, lifecycle.Transition(context.Background(), task.ID, models.StatusFailed, nil))

	assert.Equal(t, 1, repo.dialogClears)
}

func TestTransitionToStatusWithLiveAgentKeepsPendingDialogs(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusStarting, "d1")
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)

	require.NoError(t, lifecycle.Transition(context.Background(), task.ID, models.StatusRunning, nil))

	assert.Zero(t, repo.dialogClears)
}

func TestLoadOfMissingTaskIsNotFound(t *testing.T) {
	t.Parallel()
	lifecycle, _ := newLifecycle(newFakeRepository())
	id, err := models.NewTaskID()
	require.NoError(t, err)

	_, err = lifecycle.Load(context.Background(), id)
	assert.ErrorContains(t, err, "tarea no encontrada: "+id.String())
}

func TestEphemeralNotificationsAreNotPersisted(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	lifecycle, publisher := newLifecycle(repo)

	require.NoError(t, lifecycle.NotifyChanged(context.Background(), task.ID))

	assert.Empty(t, repo.events)
	require.Len(t, publisher.published, 1)
	assert.Nil(t, publisher.published[0].Seq)
}

func TestHandleAgentEventParksTaskAndSchedulesResume(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)
	resetsAt := fixedNow.Add(time.Hour).Unix()

	require.NoError(t, application.NewHandleAgentEvent(lifecycle, clock).Execute(context.Background(), task.ID,
		models.UsageLimitReached{ResetsAt: &resetsAt, Message: "límite"}))

	assert.Equal(t, []models.TaskStatus{models.StatusRateLimited}, repo.statuses)
	require.Len(t, repo.usageLimits, 1)
	assert.Equal(t, uint32(1), repo.usageLimits[0].Attempts)
	require.NotNil(t, repo.usageLimits[0].ResumeAt)
	assert.True(t, repo.usageLimits[0].ResumeAt.After(time.Unix(resetsAt, 0)))
}

func TestHandleAgentEventSettledRunResetsPreviousAttempts(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	task.UsageLimit = models.UsageLimitState{Attempts: 2}
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)

	require.NoError(t, application.NewHandleAgentEvent(lifecycle, clock).Execute(context.Background(), task.ID, models.RunSettled{}))

	assert.Equal(t, []models.UsageLimitState{{}}, repo.usageLimits)
}

func TestHandleAgentEventLateSettleWhileRateLimitedKeepsAttempts(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRateLimited)
	task.UsageLimit = models.UsageLimitState{Attempts: 2}
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)

	require.NoError(t, application.NewHandleAgentEvent(lifecycle, clock).Execute(context.Background(), task.ID, models.RunSettled{}))

	assert.Empty(t, repo.usageLimits)
}

func TestHandleAgentEventRunStartMovesTaskToRunning(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusStarting)
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)

	require.NoError(t, application.NewHandleAgentEvent(lifecycle, clock).Execute(context.Background(), task.ID, models.RunStarted{}))

	assert.Equal(t, []models.TaskStatus{models.StatusRunning}, repo.statuses)
	assert.Len(t, repo.events, 2, "the run start and the status change are both recorded")
}

func TestHandleAgentEventDeltasArePublishedButNotPersisted(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	repo := newFakeRepository(task)
	lifecycle, publisher := newLifecycle(repo)

	require.NoError(t, application.NewHandleAgentEvent(lifecycle, clock).Execute(context.Background(), task.ID, models.TextDelta{Delta: "ho"}))

	assert.Empty(t, repo.events)
	assert.Len(t, publisher.published, 1)
	assert.Empty(t, repo.statuses)
}

func TestHandleAgentEventExitAfterFinishingIsRecordedWithoutStatusChange(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusFinishing)
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)
	code := int32(0)

	require.NoError(t, application.NewHandleAgentEvent(lifecycle, clock).Execute(context.Background(), task.ID, models.ProcessExited{Code: &code}))

	assert.Empty(t, repo.statuses)
	assert.Equal(t, []models.TaskEventKind{models.AgentEventKind{Event: models.ProcessExited{Code: &code}}}, repo.events)
}
