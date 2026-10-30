package dto_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

var dtoNow = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func newTask(t *testing.T, status models.TaskStatus) models.Task {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return models.Task{
		ID:           id,
		WorkspaceID:  "ws-1",
		Title:        "t",
		Prompt:       "p",
		Branch:       "pi/t",
		BaseBranch:   "main",
		WorktreePath: "/wt/t",
		Status:       status,
		Priority:     models.PriorityHigh,
		CreatedAt:    dtoNow,
		UpdatedAt:    dtoNow,
	}
}

func TestTaskDTOUsesSerdeFieldNames(t *testing.T) {
	t.Parallel()
	task := newTask(t, models.StatusBacklog)

	data, err := json.Marshal(dto.TaskFromDomain(task, dtoNow))
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(data, &fields))
	for _, name := range []string{
		"id", "workspace_id", "base_branch", "worktree_path", "memory_limit_mb", "cpu_limit_percent",
		"resume_at", "actions", "stage", "needs_attention", "pending_dialogs", "created_at", "updated_at",
	} {
		assert.Contains(t, fields, name)
	}
	assert.Equal(t, "high", fields["priority"])
	assert.Equal(t, "backlog", fields["status"])
	assert.Equal(t, "backlog", fields["stage"])
}

func TestTaskDTOActionsFollowTheStatus(t *testing.T) {
	t.Parallel()
	backlog := dto.TaskFromDomain(newTask(t, models.StatusBacklog), dtoNow).Actions
	assert.True(t, backlog.CanStart)
	assert.True(t, backlog.CanEdit)
	assert.True(t, backlog.CanDelete)
	assert.False(t, backlog.CanViewDiff)
	assert.False(t, backlog.CanDiscard)
	assert.False(t, backlog.CanReset)

	running := dto.TaskFromDomain(newTask(t, models.StatusRunning), dtoNow).Actions
	assert.True(t, running.CanMessage)
	assert.True(t, running.CanAbort)
	assert.True(t, running.CanFinish)
	assert.True(t, running.CanOpenTerminal)

	merged := newTask(t, models.StatusMerged)
	assert.False(t, dto.TaskFromDomain(merged, dtoNow).Actions.CanViewDiff)
	merged.DiffRange = &models.DiffRange{BaseSHA: "a", HeadSHA: "b"}
	assert.True(t, dto.TaskFromDomain(merged, dtoNow).Actions.CanViewDiff)
	assert.False(t, dto.TaskFromDomain(merged, dtoNow).Actions.CanOpenTerminal)
}

func TestTaskDTOResumeAtIsRFC3339InUTC(t *testing.T) {
	t.Parallel()
	task := newTask(t, models.StatusRateLimited)
	resumeAt := time.Date(2026, 10, 24, 15, 45, 30, 0, time.FixedZone("CEST", 2*3600))
	task.UsageLimit = models.UsageLimitState{ResumeAt: &resumeAt}

	view := dto.TaskFromDomain(task, dtoNow)

	require.NotNil(t, view.ResumeAt)
	assert.Equal(t, "2026-10-24T13:45:30+00:00", *view.ResumeAt)
}

func TestTaskDTOCountsOnlyDialogsThatHaveNotExpired(t *testing.T) {
	t.Parallel()
	task := newTask(t, models.StatusRunning)
	expired := dtoNow
	task.PendingDialogs = []models.PendingDialog{
		models.NewPendingDialog("live", nil),
		models.NewPendingDialog("expired", &expired),
	}

	view := dto.TaskFromDomain(task, dtoNow)

	assert.Equal(t, 1, view.PendingDialogs)
	assert.Equal(t, "attention", view.Stage)
	assert.True(t, view.NeedsAttention)
}

func TestCreateTaskDTOConvertsToDomainInput(t *testing.T) {
	t.Parallel()
	input, err := dto.CreateTaskDTO{
		WorkspaceID: "ws-1",
		Title:       "t",
		Prompt:      "p",
		Priority:    "urgent",
		StartNow:    true,
		Images:      []dto.ImageDTO{{Name: "a.png", MimeType: "image/png", Data: "aGk="}},
	}.ToNewTask()

	require.NoError(t, err)
	assert.Equal(t, models.PriorityUrgent, input.Priority)
	assert.True(t, input.StartNow)
	require.Len(t, input.Images, 1)
	assert.Equal(t, "a.png", input.Images[0].Name)
}

func TestCreateTaskDTOWithoutPriorityUsesTheDefault(t *testing.T) {
	t.Parallel()
	input, err := dto.CreateTaskDTO{WorkspaceID: "ws-1", Title: "t", Prompt: "p"}.ToNewTask()

	require.NoError(t, err)
	assert.Equal(t, models.DefaultTaskPriority, input.Priority)
}

func TestCreateTaskDTORejectsUnknownPriority(t *testing.T) {
	t.Parallel()
	_, err := dto.CreateTaskDTO{Priority: "critical"}.ToNewTask()

	assert.ErrorIs(t, err, werrors.InvalidPriority(""))
}

func TestUpdateTaskDTOConvertsToUseCaseInput(t *testing.T) {
	t.Parallel()
	input, err := dto.UpdateTaskDTO{
		Title:               "nuevo",
		Prompt:              "p",
		Priority:            "low",
		RemoveAttachmentIDs: []string{"x"},
	}.ToUpdateInput()

	require.NoError(t, err)
	assert.Equal(t, application.UpdateInput{
		Title:     "nuevo",
		Prompt:    "p",
		Priority:  models.PriorityLow,
		RemoveIDs: []string{"x"},
		Images:    []models.ImageAttachment{},
	}, input)
}
