package models_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

func newTaskWith(status models.TaskStatus, dialogs ...models.PendingDialog) models.Task {
	now := time.Now().UTC()
	return models.Task{
		ID:             mustTaskID(),
		WorkspaceID:    "ws",
		Title:          "t",
		Prompt:         "p",
		Branch:         "pi/t",
		BaseBranch:     "main",
		WorktreePath:   "/wt/t",
		Status:         status,
		Priority:       models.DefaultTaskPriority,
		PendingDialogs: dialogs,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func mustTaskID() models.TaskID {
	id, err := models.NewTaskID()
	if err != nil {
		panic(err)
	}
	return id
}

func TestSlugifyNormalizesAccentsAndSymbols(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "anade-login-oauth-github", models.Slugify("  Añade Login: OAuth/GitHub!! "))
	assert.Equal(t, "task", models.Slugify("¿¿??"))
}

func TestSlugifyIsBounded(t *testing.T) {
	t.Parallel()
	slug := models.Slugify(repeat("a", 200))
	assert.LessOrEqual(t, len(slug), 40)
}

func TestNamesEmbedTheUniqueID(t *testing.T) {
	t.Parallel()
	unique := uuid.New()
	assert.Equal(t, "pi/fix-bug-"+unique.String(), models.BranchName("Fix bug", unique))
	assert.Equal(t, "fix-bug-"+unique.String(), models.WorktreeDirName("Fix bug", unique))
	assert.Equal(t, "my-app-ws-1", models.WorkspaceDirName("My App", "ws-1"))
}

func TestIsTaskBranchRecognizesOnlyPiBranches(t *testing.T) {
	t.Parallel()
	assert.True(t, models.IsTaskBranch("pi/x"))
	assert.False(t, models.IsTaskBranch("pix"))
	assert.False(t, models.IsTaskBranch("main"))
}

func TestNewTaskDerivesTitleFromFirstPromptLine(t *testing.T) {
	t.Parallel()
	task, err := models.NewTask{
		WorkspaceID: " ws ",
		Title:       "  ",
		Prompt:      "  Arregla el test\nmás detalles",
	}.Validated()
	require.NoError(t, err)
	assert.Equal(t, "Arregla el test", task.Title)
	assert.Equal(t, "ws", task.WorkspaceID)
}

func TestNewTaskRejectsZeroLimits(t *testing.T) {
	t.Parallel()
	zero := uint32(0)
	_, err := models.NewTask{
		WorkspaceID: "ws", Title: "t", Prompt: "p",
		Limits: models.ResourceLimits{MemoryMB: &zero},
	}.Validated()
	assert.ErrorIs(t, err, werrors.InvalidLimits())
}

func TestNewTaskKeepsPriorityWhenValidated(t *testing.T) {
	t.Parallel()
	task, err := models.NewTask{WorkspaceID: "ws", Title: "t", Prompt: "p", Priority: models.PriorityUrgent}.Validated()
	require.NoError(t, err)
	assert.Equal(t, models.PriorityUrgent, task.Priority)
}

func TestNewTaskRejectsInvalidImages(t *testing.T) {
	t.Parallel()
	_, err := models.NewTask{
		WorkspaceID: "ws", Title: "t", Prompt: "p",
		Images: []models.ImageAttachment{{Name: "a.bmp", MimeType: "image/bmp", Data: "aGk="}},
	}.Validated()
	assert.ErrorIs(t, err, werrors.InvalidAttachment(""))
}

func TestNewTaskKeepsValidImages(t *testing.T) {
	t.Parallel()
	image := models.ImageAttachment{Name: "a.png", MimeType: "image/png", Data: "aGk="}
	task, err := models.NewTask{WorkspaceID: "ws", Title: "t", Prompt: "p", Images: []models.ImageAttachment{image}}.Validated()
	require.NoError(t, err)
	assert.Equal(t, []models.ImageAttachment{image}, task.Images)
}

func TestNewTaskRequiresPrompt(t *testing.T) {
	t.Parallel()
	_, err := models.NewTask{WorkspaceID: "ws", Title: "t", Prompt: " \n "}.Validated()
	assert.ErrorIs(t, err, werrors.EmptyPrompt())
}

func TestNewTaskRejectsTooLongTitle(t *testing.T) {
	t.Parallel()
	_, err := models.NewTask{WorkspaceID: "ws", Title: repeat("x", 121), Prompt: "p"}.Validated()
	assert.ErrorIs(t, err, werrors.InvalidTitle())
}

func TestValidatedTrimsTheRequestedBaseBranch(t *testing.T) {
	t.Parallel()
	branch := "  origin/develop "
	task, err := models.NewTask{WorkspaceID: "ws", Title: "t", Prompt: "p", BaseBranch: &branch}.Validated()
	require.NoError(t, err)
	require.NotNil(t, task.BaseBranch)
	assert.Equal(t, "origin/develop", *task.BaseBranch)
}

func TestValidatedDropsABlankBaseBranch(t *testing.T) {
	t.Parallel()
	branch := "   "
	task, err := models.NewTask{WorkspaceID: "ws", Title: "t", Prompt: "p", BaseBranch: &branch}.Validated()
	require.NoError(t, err)
	assert.Nil(t, task.BaseBranch)
}

func TestStatusRoundTripsThroughString(t *testing.T) {
	t.Parallel()
	for _, status := range models.AllTaskStatuses {
		parsed, err := models.ParseTaskStatus(string(status))
		require.NoError(t, err)
		assert.Equal(t, status, parsed)
	}
	_, err := models.ParseTaskStatus("nope")
	assert.Error(t, err)
}

func TestAgentEventsDriveLiveStatuses(t *testing.T) {
	t.Parallel()
	next, detail, ok := models.StatusStarting.AfterAgentEvent(models.RunStarted{})
	assert.True(t, ok)
	assert.Equal(t, models.StatusRunning, next)
	assert.Nil(t, detail)

	next, _, ok = models.StatusRunning.AfterAgentEvent(models.RunSettled{})
	assert.True(t, ok)
	assert.Equal(t, models.StatusIdle, next)

	_, _, ok = models.StatusRunning.AfterAgentEvent(models.RunStarted{})
	assert.False(t, ok)

	code := int32(1)
	next, detail, ok = models.StatusIdle.AfterAgentEvent(models.ProcessExited{Code: &code})
	assert.True(t, ok)
	assert.Equal(t, models.StatusInterrupted, next)
	require.NotNil(t, detail)
	assert.Contains(t, *detail, "código 1")
}

func TestAgentEventsDoNotTouchNonLiveStatuses(t *testing.T) {
	t.Parallel()
	exited := models.ProcessExited{Code: int32Ptr(0)}
	_, _, ok := models.StatusFinishing.AfterAgentEvent(exited)
	assert.False(t, ok)
	_, _, ok = models.StatusDiscarded.AfterAgentEvent(exited)
	assert.False(t, ok)
	_, _, ok = models.StatusAwaitingReview.AfterAgentEvent(models.RunStarted{})
	assert.False(t, ok)
}

func TestUsageLimitEventParksTheRunningTask(t *testing.T) {
	t.Parallel()
	next, detail, ok := models.StatusRunning.AfterAgentEvent(models.UsageLimitReached{Message: "límite"})
	assert.True(t, ok)
	assert.Equal(t, models.StatusRateLimited, next)
	require.NotNil(t, detail)
	assert.Equal(t, "límite", *detail)
}

func TestStatusRules(t *testing.T) {
	t.Parallel()
	assert.True(t, models.StatusIdle.AcceptsMessages())
	assert.False(t, models.StatusAwaitingReview.AcceptsMessages())
	assert.True(t, models.StatusConflict.CanMerge())
	assert.True(t, models.StatusConflict.CanResume())
	assert.False(t, models.StatusMerged.CanDiscard())
	assert.False(t, models.StatusFinishing.CanDiscard())
}

func TestResetIsNotAllowedInBacklogMergedOrFinishing(t *testing.T) {
	t.Parallel()
	assert.False(t, models.StatusBacklog.CanReset())
	assert.False(t, models.StatusMerged.CanReset())
	assert.False(t, models.StatusFinishing.CanReset())
	assert.True(t, models.StatusDiscarded.CanReset())
	assert.True(t, models.StatusRunning.CanReset())
	assert.True(t, models.StatusConflict.CanReset())
}

func TestOnlyBacklogCanStartEditOrDelete(t *testing.T) {
	t.Parallel()
	assert.True(t, models.StatusBacklog.CanStart())
	assert.True(t, models.StatusBacklog.CanEdit())
	assert.True(t, models.StatusBacklog.CanDelete())
	assert.False(t, models.StatusBacklog.CanDiscard())
	assert.False(t, models.StatusIdle.CanStart())
	assert.False(t, models.StatusIdle.CanEdit())
	assert.False(t, models.StatusIdle.CanDelete())
}

func TestBacklogHasNoLiveAgentAndIsNotClosed(t *testing.T) {
	t.Parallel()
	assert.False(t, models.StatusBacklog.HasLiveAgent())
	assert.False(t, models.StatusBacklog.IsClosed())
	assert.False(t, models.StatusBacklog.CanFinish())
	assert.False(t, models.StatusBacklog.CanResume())
	assert.False(t, models.StatusBacklog.CanMerge())
}

func TestWorktreeExistsUnlessBacklogMergedOrDiscarded(t *testing.T) {
	t.Parallel()
	for _, status := range models.AllTaskStatuses {
		expected := status != models.StatusBacklog && status != models.StatusMerged && status != models.StatusDiscarded
		assert.Equal(t, expected, status.HasWorktree(), string(status))
	}
}

func TestOnlyBacklogMergedAndDiscardedAreInactive(t *testing.T) {
	t.Parallel()
	for _, status := range []models.TaskStatus{models.StatusBacklog, models.StatusMerged, models.StatusDiscarded} {
		assert.True(t, status.IsInactive())
	}
	for _, status := range []models.TaskStatus{
		models.StatusStarting, models.StatusRunning, models.StatusIdle, models.StatusFinishing,
		models.StatusAwaitingReview, models.StatusConflict, models.StatusFailed, models.StatusInterrupted,
	} {
		assert.False(t, status.IsInactive())
	}
}

func TestCanViewDiffForOpenTasksExceptBacklog(t *testing.T) {
	t.Parallel()
	for _, status := range models.AllTaskStatuses {
		expected := !status.IsClosed() && status != models.StatusBacklog
		assert.Equal(t, expected, newTaskWith(status).CanViewDiff(), string(status))
	}
}

func TestCanViewDiffForMergedOnlyWithRange(t *testing.T) {
	t.Parallel()
	merged := newTaskWith(models.StatusMerged)
	assert.False(t, merged.CanViewDiff())
	merged.DiffRange = &models.DiffRange{BaseSHA: "a", HeadSHA: "b"}
	assert.True(t, merged.CanViewDiff())
	discarded := newTaskWith(models.StatusDiscarded)
	discarded.DiffRange = merged.DiffRange
	assert.False(t, discarded.CanViewDiff())
}

func TestStageMapsBacklogMergedDiscardedAndReview(t *testing.T) {
	t.Parallel()
	now := time.Now()
	assert.Equal(t, models.StageBacklog, newTaskWith(models.StatusBacklog).StageAt(now))
	assert.Equal(t, models.StageDone, newTaskWith(models.StatusMerged).StageAt(now))
	assert.Equal(t, models.StageDone, newTaskWith(models.StatusDiscarded).StageAt(now))
	assert.Equal(t, models.StageReview, newTaskWith(models.StatusAwaitingReview).StageAt(now))
}

func TestStageMapsIdleConflictFailedAndInterruptedToAttention(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for _, status := range []models.TaskStatus{models.StatusIdle, models.StatusConflict, models.StatusFailed, models.StatusInterrupted} {
		assert.Equal(t, models.StageAttention, newTaskWith(status).StageAt(now), string(status))
	}
}

func TestStageMapsLiveStatusesToWorkingUnlessPendingDialogs(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for _, status := range []models.TaskStatus{models.StatusStarting, models.StatusRunning, models.StatusFinishing} {
		assert.Equal(t, models.StageWorking, newTaskWith(status).StageAt(now), string(status))
		blocked := newTaskWith(status, models.NewPendingDialog("d1", nil))
		assert.Equal(t, models.StageAttention, blocked.StageAt(now), string(status))
	}
}

func TestIdleWithActiveSubagentsIsWorkingUnlessDialogPending(t *testing.T) {
	t.Parallel()
	now := time.Now()
	idle := newTaskWith(models.StatusIdle)
	idle.Subagents = models.NewSubagentActivity(2, 0)
	assert.Equal(t, models.StageWorking, idle.StageAt(now))
	assert.False(t, idle.NeedsAttentionAt(now))

	blocked := newTaskWith(models.StatusIdle, models.NewPendingDialog("d1", nil))
	blocked.Subagents = models.NewSubagentActivity(2, 0)
	assert.Equal(t, models.StageAttention, blocked.StageAt(now))

	onlyWaiting := newTaskWith(models.StatusIdle)
	onlyWaiting.Subagents = models.NewSubagentActivity(0, 1)
	assert.Equal(t, models.StageAttention, onlyWaiting.StageAt(now))
}

func TestNeedsAttentionIsTrueOnlyForAttentionAndReviewStages(t *testing.T) {
	t.Parallel()
	now := time.Now()
	assert.True(t, newTaskWith(models.StatusIdle).NeedsAttentionAt(now))
	assert.True(t, newTaskWith(models.StatusAwaitingReview).NeedsAttentionAt(now))
	assert.True(t, newTaskWith(models.StatusRunning, models.NewPendingDialog("d1", nil)).NeedsAttentionAt(now))
	assert.False(t, newTaskWith(models.StatusRunning).NeedsAttentionAt(now))
	assert.False(t, newTaskWith(models.StatusBacklog).NeedsAttentionAt(now))
	assert.False(t, newTaskWith(models.StatusMerged).NeedsAttentionAt(now))
}

func TestExpiredPendingDialogsDoNotCountTowardsAttention(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	expired := models.NewPendingDialog("d1", &now)
	task := newTaskWith(models.StatusRunning, expired)

	assert.Equal(t, 0, task.PendingDialogCountAt(now))
	assert.Equal(t, models.StageWorking, task.StageAt(now))
	assert.False(t, task.NeedsAttentionAt(now))

	before := now.Add(-time.Millisecond)
	assert.Equal(t, 1, task.PendingDialogCountAt(before))
	assert.Equal(t, models.StageAttention, task.StageAt(before))
	assert.True(t, task.NeedsAttentionAt(before))
}

func repeat(text string, times int) string {
	result := ""
	for range times {
		result += text
	}
	return result
}

func int32Ptr(value int32) *int32 {
	return &value
}
