package tasks_test

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

func TestInsertsFindsAndUpdatesStatus(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	original := newTask(t, workspaceID, "uno", models.StatusStarting)
	insertTask(t, repo, original)

	require.NoError(t, repo.Tasks.UpdateStatus(bg(), original.ID, models.StatusConflict, ptr("conflictos")))

	got := found(t, repo, original.ID)
	assert.Equal(t, models.StatusConflict, got.Status)
	require.NotNil(t, got.Error)
	assert.Equal(t, "conflictos", *got.Error)
	assert.Equal(t, original.Title, got.Title)
	assert.Equal(t, original.WorktreePath, got.WorktreePath)
}

func TestUpdateStatusOfMissingTaskIsNotFound(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	err := repo.Tasks.UpdateStatus(bg(), newTaskID(t), models.StatusRunning, nil)
	assert.ErrorContains(t, err, "tarea no encontrada")
}

func TestListsNewestFirstFilteredByWorkspaceAndStatus(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	first := insertWorkspace(t, repo.DB, "/repos/a")
	second := insertWorkspace(t, repo.DB, "/repos/b")
	older := newTask(t, first, "older", models.StatusRunning)
	older.CreatedAt = older.CreatedAt.Add(-time.Hour)
	newer := newTask(t, first, "newer", models.StatusRunning)
	other := newTask(t, second, "other", models.StatusRunning)
	other.CreatedAt = other.CreatedAt.Add(time.Minute)
	insertTask(t, repo, older)
	insertTask(t, repo, newer)
	insertTask(t, repo, other)

	all, err := repo.Tasks.List(bg(), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"other", "newer", "older"}, titles(all))

	scoped, err := repo.Tasks.List(bg(), &first)
	require.NoError(t, err)
	assert.Equal(t, []string{"newer", "older"}, titles(scoped))

	byStatus, err := repo.Tasks.ListByStatus(bg(), []models.TaskStatus{models.StatusRunning})
	require.NoError(t, err)
	assert.Len(t, byStatus, 3)
}

func TestAppendsEventsWithIncreasingSeqAndReadsAfterCursor(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	task := newTask(t, workspaceID, "uno", models.StatusRunning)
	insertTask(t, repo, task)

	first, err := repo.Tasks.AppendEvent(bg(), task.ID, models.AgentEventKind{Event: models.RunStarted{}})
	require.NoError(t, err)
	second, err := repo.Tasks.AppendEvent(bg(), task.ID, models.AgentEventKind{Event: models.UserMessage{Text: "hola"}})
	require.NoError(t, err)
	require.NotNil(t, first.Seq)
	require.NotNil(t, second.Seq)
	assert.Less(t, *first.Seq, *second.Seq)

	all, err := repo.Tasks.Events(bg(), task.ID, nil)
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, models.UserMessage{Text: "hola"}, all[1].Kind.(models.AgentEventKind).Event)

	after, err := repo.Tasks.Events(bg(), task.ID, first.Seq)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, *second.Seq, *after[0].Seq)
}

func TestUpdatesDetailsOnlyWhileInBacklog(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	backlog := newTask(t, workspaceID, "uno", models.StatusBacklog)
	insertTask(t, repo, backlog)
	limits := models.ResourceLimits{MemoryMB: ptr(uint32(512)), CPUPercent: ptr(uint32(150))}

	updated, err := repo.Tasks.UpdateDetailsIfBacklog(bg(), backlog.ID, "nuevo", "prompt", models.PriorityUrgent, limits, "develop")
	require.NoError(t, err)
	assert.True(t, updated)
	got := found(t, repo, backlog.ID)
	assert.Equal(t, "nuevo", got.Title)
	assert.Equal(t, models.PriorityUrgent, got.Priority)
	assert.Equal(t, "develop", got.BaseBranch)
	assert.Equal(t, limits, got.Limits)

	running := newTask(t, workspaceID, "dos", models.StatusRunning)
	insertTask(t, repo, running)
	updated, err = repo.Tasks.UpdateDetailsIfBacklog(bg(), running.ID, "x", "y", models.PriorityLow, models.ResourceLimits{}, "main")
	require.NoError(t, err)
	assert.False(t, updated)
	assert.Equal(t, "dos", found(t, repo, running.ID).Title)

	updated, err = repo.Tasks.UpdateDetailsIfBacklog(bg(), newTaskID(t), "x", "y", models.PriorityLow, models.ResourceLimits{}, "main")
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestClaimForStartSetsPlanAndIsExclusive(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	original := newTask(t, workspaceID, "uno", models.StatusBacklog)
	insertTask(t, repo, original)

	first, err := repo.Tasks.ClaimForStart(bg(), original.ID, "pi/nueva-rama", "/wt/nueva", "develop")
	require.NoError(t, err)
	second, err := repo.Tasks.ClaimForStart(bg(), original.ID, "pi/otra", "/wt/otra", "main")
	require.NoError(t, err)
	assert.True(t, first)
	assert.False(t, second)

	claimed := found(t, repo, original.ID)
	assert.Equal(t, models.StatusStarting, claimed.Status)
	assert.Equal(t, "pi/nueva-rama", claimed.Branch)
	assert.Equal(t, "/wt/nueva", claimed.WorktreePath)
	assert.Equal(t, "develop", claimed.BaseBranch)
}

// TestConcurrentClaimsOfOneTaskHaveExactlyOneWinner is the race the optimistic claim exists for: many
// starts of the same backlog task at once must reclaim it only once.
func TestConcurrentClaimsOfOneTaskHaveExactlyOneWinner(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/race")
	task := newTask(t, workspaceID, "race", models.StatusBacklog)
	insertTask(t, repo, task)

	const contenders = 24
	results := make([]bool, contenders)
	errs := make([]error, contenders)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for index := range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[index], errs[index] = repo.Tasks.ClaimForStart(bg(), task.ID, "pi/race", "/wt/race", "main")
		}()
	}
	close(start)
	wg.Wait()

	winners := 0
	for index := range contenders {
		require.NoError(t, errs[index], "claim %d", index)
		if results[index] {
			winners++
		}
	}
	assert.Equal(t, 1, winners, "exactly one claim must win")
	assert.Equal(t, models.StatusStarting, found(t, repo, task.ID).Status)
}

func TestClaimForStartRejectsMissingTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	claimed, err := repo.Tasks.ClaimForStart(bg(), newTaskID(t), "b", "/wt/b", "main")
	require.NoError(t, err)
	assert.False(t, claimed)
}

func TestRevertStartClaimReturnsStartingTaskToBacklog(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	original := newTask(t, workspaceID, "uno", models.StatusBacklog)
	insertTask(t, repo, original)
	_, err := repo.Tasks.ClaimForStart(bg(), original.ID, "pi/x", "/wt/x", "main")
	require.NoError(t, err)

	reverted, err := repo.Tasks.RevertStartClaim(bg(), original.ID)
	require.NoError(t, err)
	again, err := repo.Tasks.RevertStartClaim(bg(), original.ID)
	require.NoError(t, err)
	assert.True(t, reverted)
	assert.False(t, again)
	assert.Equal(t, models.StatusBacklog, found(t, repo, original.ID).Status)
}

func TestRevertStartClaimIgnoresTasksNotInStarting(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	running := newTask(t, workspaceID, "uno", models.StatusRunning)
	insertTask(t, repo, running)

	reverted, err := repo.Tasks.RevertStartClaim(bg(), running.ID)
	require.NoError(t, err)
	assert.False(t, reverted)
	assert.Equal(t, models.StatusRunning, found(t, repo, running.ID).Status)
}

func TestResetToBacklogDeletesEventsAndClearsFields(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	task := newTask(t, workspaceID, "uno", models.StatusFailed)
	task.Error = ptr("boom")
	task.Subagents = models.NewSubagentActivity(2, 1)
	task.PendingDialogs = []models.PendingDialog{models.NewPendingDialog("d1", nil)}
	insertTask(t, repo, task)
	_, err := repo.Tasks.AppendEvent(bg(), task.ID, models.AgentEventKind{Event: models.RunStarted{}})
	require.NoError(t, err)

	reset, err := repo.Tasks.ResetToBacklog(bg(), task.ID)
	require.NoError(t, err)
	assert.True(t, reset)

	got := found(t, repo, task.ID)
	assert.Equal(t, models.StatusBacklog, got.Status)
	assert.Empty(t, got.WorktreePath)
	assert.Empty(t, got.Branch)
	assert.Nil(t, got.Error)
	assert.Empty(t, got.PendingDialogs)
	assert.Equal(t, models.SubagentActivity{}, got.Subagents)
	events, err := repo.Tasks.Events(bg(), task.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestResetToBacklogIgnoresBacklogMergedFinishingAndMissingTasks(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	for _, status := range []models.TaskStatus{models.StatusBacklog, models.StatusMerged, models.StatusFinishing} {
		task := newTask(t, workspaceID, string(status), status)
		insertTask(t, repo, task)
		reset, err := repo.Tasks.ResetToBacklog(bg(), task.ID)
		require.NoError(t, err)
		assert.False(t, reset, string(status))
		assert.Equal(t, status, found(t, repo, task.ID).Status)
	}
	reset, err := repo.Tasks.ResetToBacklog(bg(), newTaskID(t))
	require.NoError(t, err)
	assert.False(t, reset)
}

func TestPendingDialogsAddRemoveClearAndIgnoreDuplicates(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	task := newTask(t, workspaceID, "uno", models.StatusRunning)
	insertTask(t, repo, task)
	deadline := fixedClock.Add(time.Minute)

	require.NoError(t, repo.Tasks.AddPendingDialog(bg(), task.ID, models.NewPendingDialog("d1", nil)))
	require.NoError(t, repo.Tasks.AddPendingDialog(bg(), task.ID, models.NewPendingDialog("d2", &deadline)))
	require.NoError(t, repo.Tasks.AddPendingDialog(bg(), task.ID, models.NewPendingDialog("d1", nil)))
	dialogs := found(t, repo, task.ID).PendingDialogs
	assert.Equal(t, []string{"d1", "d2"}, dialogIDs(dialogs))
	require.NotNil(t, dialogs[1].ExpiresAt)
	assert.True(t, dialogs[1].ExpiresAt.Equal(deadline))

	require.NoError(t, repo.Tasks.RemovePendingDialog(bg(), task.ID, "d1"))
	require.NoError(t, repo.Tasks.RemovePendingDialog(bg(), task.ID, "missing"))
	assert.Equal(t, []string{"d2"}, dialogIDs(found(t, repo, task.ID).PendingDialogs))

	require.NoError(t, repo.Tasks.ClearPendingDialogs(bg(), task.ID))
	assert.Empty(t, found(t, repo, task.ID).PendingDialogs)
}

func TestAddPendingDialogIgnoredForMissingOrDeadTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	idle := newTask(t, workspaceID, "uno", models.StatusAwaitingReview)
	insertTask(t, repo, idle)

	require.NoError(t, repo.Tasks.AddPendingDialog(bg(), idle.ID, models.NewPendingDialog("d1", nil)))
	require.NoError(t, repo.Tasks.AddPendingDialog(bg(), newTaskID(t), models.NewPendingDialog("d1", nil)))
	assert.Empty(t, found(t, repo, idle.ID).PendingDialogs)
}

func TestDeleteIfBacklogCascadesEvents(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	backlog := newTask(t, workspaceID, "uno", models.StatusBacklog)
	insertTask(t, repo, backlog)
	_, err := repo.Tasks.AppendEvent(bg(), backlog.ID, models.StatusKind(models.StatusBacklog, nil))
	require.NoError(t, err)

	deleted, err := repo.Tasks.DeleteIfBacklog(bg(), backlog.ID)
	require.NoError(t, err)
	assert.True(t, deleted)
	gone, err := repo.Tasks.Find(bg(), backlog.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
	events, err := repo.Tasks.Events(bg(), backlog.ID, nil)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestDeleteIfBacklogKeepsTasksInOtherStates(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	running := newTask(t, workspaceID, "uno", models.StatusRunning)
	insertTask(t, repo, running)

	deleted, err := repo.Tasks.DeleteIfBacklog(bg(), running.ID)
	require.NoError(t, err)
	assert.False(t, deleted)
	assert.Equal(t, models.StatusRunning, found(t, repo, running.ID).Status)
}

func TestDeleteInactiveByWorkspaceDeletesEveryInactiveTaskAndItsEvents(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	backlog := newTask(t, workspaceID, "b", models.StatusBacklog)
	merged := newTask(t, workspaceID, "m", models.StatusMerged)
	insertTask(t, repo, backlog)
	insertTask(t, repo, merged)
	_, err := repo.Tasks.AppendEvent(bg(), merged.ID, models.StatusKind(models.StatusMerged, nil))
	require.NoError(t, err)

	ids, deleted, err := repo.Tasks.DeleteInactiveByWorkspace(bg(), workspaceID)
	require.NoError(t, err)
	assert.True(t, deleted)
	assert.ElementsMatch(t, []models.TaskID{backlog.ID, merged.ID}, ids)
	remaining, err := repo.Tasks.List(bg(), &workspaceID)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestDeleteInactiveByWorkspaceDeletesNothingWhenOneTaskIsActive(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	backlog := newTask(t, workspaceID, "b", models.StatusBacklog)
	running := newTask(t, workspaceID, "r", models.StatusRunning)
	insertTask(t, repo, backlog)
	insertTask(t, repo, running)

	ids, deleted, err := repo.Tasks.DeleteInactiveByWorkspace(bg(), workspaceID)
	require.NoError(t, err)
	assert.False(t, deleted)
	assert.Nil(t, ids)
	remaining, err := repo.Tasks.List(bg(), &workspaceID)
	require.NoError(t, err)
	assert.Len(t, remaining, 2)
}

func TestDeleteInactiveByWorkspaceDoesNotTouchOtherWorkspaces(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	target := insertWorkspace(t, repo.DB, "/repos/target")
	other := insertWorkspace(t, repo.DB, "/repos/other")
	insertTask(t, repo, newTask(t, target, "t", models.StatusBacklog))
	survivor := newTask(t, other, "o", models.StatusBacklog)
	insertTask(t, repo, survivor)

	_, deleted, err := repo.Tasks.DeleteInactiveByWorkspace(bg(), target)
	require.NoError(t, err)
	assert.True(t, deleted)
	assert.Equal(t, survivor.ID, found(t, repo, survivor.ID).ID)
}

func TestWorkspaceWithTasksCannotBeDeleted(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	insertTask(t, repo, newTask(t, workspaceID, "uno", models.StatusBacklog))

	_, err := repo.DB.ExecContext(bg(), "DELETE FROM workspaces WHERE id = ?", workspaceID)
	assert.ErrorContains(t, err, "FOREIGN KEY constraint failed")
}

func TestSubagentActivityPersistsWhileLiveAndResetsWhenAgentIsGone(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	task := newTask(t, workspaceID, "uno", models.StatusRunning)
	insertTask(t, repo, task)

	require.NoError(t, repo.Tasks.SetSubagentActivity(bg(), task.ID, models.NewSubagentActivity(2, 1)))
	assert.Equal(t, models.NewSubagentActivity(2, 1), found(t, repo, task.ID).Subagents)

	require.NoError(t, repo.Tasks.UpdateStatus(bg(), task.ID, models.StatusAwaitingReview, nil))
	assert.Equal(t, models.SubagentActivity{}, found(t, repo, task.ID).Subagents)

	require.NoError(t, repo.Tasks.SetSubagentActivity(bg(), task.ID, models.NewSubagentActivity(5, 5)))
	assert.Equal(t, models.SubagentActivity{}, found(t, repo, task.ID).Subagents, "a task without a live agent keeps no counters")
}

func TestPriorityPersistsOnInsertAndDefaultsToNormal(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	urgent := newTask(t, workspaceID, "u", models.StatusBacklog)
	urgent.Priority = models.PriorityUrgent
	insertTask(t, repo, urgent)
	plain := newTask(t, workspaceID, "p", models.StatusBacklog)
	insertTask(t, repo, plain)

	assert.Equal(t, models.PriorityUrgent, found(t, repo, urgent.ID).Priority)
	assert.Equal(t, models.PriorityNormal, found(t, repo, plain.ID).Priority)
}

// TestConcurrentWritersNeverFailNorLoseUpdates runs dialog, event and subagent writes from many
// goroutines against the same tasks, as the agent event stream does.
func TestConcurrentWritersNeverFailNorLoseUpdates(t *testing.T) {
	t.Parallel()
	const tasksCount = 4
	const opsPerTask = 12
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/concurrent")
	tasks := make([]models.Task, 0, tasksCount)
	for index := range tasksCount {
		task := newTask(t, workspaceID, "conc"+string(rune('a'+index)), models.StatusRunning)
		insertTask(t, repo, task)
		tasks = append(tasks, task)
	}

	var wg sync.WaitGroup
	failures := make(chan error, tasksCount*opsPerTask*3)
	for _, task := range tasks {
		for op := range opsPerTask {
			wg.Add(1)
			go func() {
				defer wg.Done()
				dialogID := dialogIDFor(op)
				if err := repo.Tasks.AddPendingDialog(bg(), task.ID, models.NewPendingDialog(dialogID, nil)); err != nil {
					failures <- err
					return
				}
				if _, err := repo.Tasks.AppendEvent(bg(), task.ID, models.AgentEventKind{Event: models.UserMessage{Text: dialogID}}); err != nil {
					failures <- err
					return
				}
				if err := repo.Tasks.SetSubagentActivity(bg(), task.ID, models.NewSubagentActivity(1, 0)); err != nil {
					failures <- err
					return
				}
				if op%2 == 0 {
					if err := repo.Tasks.RemovePendingDialog(bg(), task.ID, dialogID); err != nil {
						failures <- err
					}
				}
			}()
		}
	}
	wg.Wait()
	close(failures)
	var failed []error
	for err := range failures {
		failed = append(failed, err)
	}
	require.Empty(t, failed, "concurrent writes must not fail")

	for _, task := range tasks {
		got := found(t, repo, task.ID)
		var want []string
		for op := range opsPerTask {
			if op%2 == 1 {
				want = append(want, dialogIDFor(op))
			}
		}
		sort.Strings(want)
		ids := dialogIDs(got.PendingDialogs)
		sort.Strings(ids)
		assert.Equal(t, want, ids, "a dialog update was lost")
		events, err := repo.Tasks.Events(bg(), task.ID, nil)
		require.NoError(t, err)
		assert.Len(t, events, opsPerTask, "events are missing")
	}
}

// TestDeleteInactiveByWorkspaceSurvivesConcurrentWrites checks that the transactional workspace cleanup
// does not fail while other tasks are being written.
func TestDeleteInactiveByWorkspaceSurvivesConcurrentWrites(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	target := insertWorkspace(t, repo.DB, "/repos/target")
	busy := insertWorkspace(t, repo.DB, "/repos/busy")
	live := newTask(t, busy, "live", models.StatusRunning)
	insertTask(t, repo, live)

	var wg sync.WaitGroup
	failures := make(chan error, 60)
	for op := range 60 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if op%6 == 0 {
				if _, _, err := repo.Tasks.DeleteInactiveByWorkspace(bg(), target); err != nil {
					failures <- err
				}
				return
			}
			if err := repo.Tasks.AddPendingDialog(bg(), live.ID, models.NewPendingDialog("x"+dialogIDFor(op), nil)); err != nil {
				failures <- err
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
}

func TestRecordsDiffRangeAndReturnsItOnFind(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	merged := newTask(t, workspaceID, "uno", models.StatusMerged)
	insertTask(t, repo, merged)

	require.NoError(t, repo.Tasks.RecordDiffRange(bg(), merged.ID, models.DiffRange{BaseSHA: "aaa", HeadSHA: "bbb"}))
	assert.Equal(t, &models.DiffRange{BaseSHA: "aaa", HeadSHA: "bbb"}, found(t, repo, merged.ID).DiffRange)
}

func TestTaskWithoutRecordedRangeHasNone(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	task := newTask(t, workspaceID, "uno", models.StatusMerged)
	insertTask(t, repo, task)
	assert.Nil(t, found(t, repo, task.ID).DiffRange)
}

func TestRecordingDiffRangeForMissingTaskIsNotAnError(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	require.NoError(t, repo.Tasks.RecordDiffRange(bg(), newTaskID(t), models.DiffRange{BaseSHA: "a", HeadSHA: "b"}))
}

func TestListActiveExcludesDoneOrdersNewestFirstAndFiltersWorkspace(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	first := insertWorkspace(t, repo.DB, "/repos/a")
	second := insertWorkspace(t, repo.DB, "/repos/b")
	oldest := newTask(t, first, "oldest", models.StatusIdle)
	oldest.CreatedAt = oldest.CreatedAt.Add(-2 * time.Hour)
	middle := newTask(t, first, "middle", models.StatusMerged)
	middle.CreatedAt = middle.CreatedAt.Add(-time.Hour)
	newest := newTask(t, first, "newest", models.StatusRunning)
	foreign := newTask(t, second, "foreign", models.StatusRunning)
	insertTask(t, repo, oldest)
	insertTask(t, repo, middle)
	insertTask(t, repo, newest)
	insertTask(t, repo, foreign)

	active, err := repo.Tasks.ListActive(bg(), &first)
	require.NoError(t, err)
	assert.Equal(t, []string{"newest", "oldest"}, titles(active))

	all, err := repo.Tasks.ListActive(bg(), nil)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestListDonePagePagesNewestFirstWithIDTiebreak(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	for index := range 5 {
		task := newTask(t, workspaceID, "done"+string(rune('a'+index)), models.StatusMerged)
		task.UpdatedAt = fixedClock.Add(-time.Duration(index) * time.Minute)
		insertTask(t, repo, task)
	}
	open := newTask(t, workspaceID, "open", models.StatusRunning)
	insertTask(t, repo, open)

	firstPage, err := repo.Tasks.ListDonePage(bg(), &workspaceID, 2, 0)
	require.NoError(t, err)
	secondPage, err := repo.Tasks.ListDonePage(bg(), &workspaceID, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"donea", "doneb"}, titles(firstPage))
	assert.Equal(t, []string{"donec", "doned"}, titles(secondPage))
}

func TestLimitsPersistOnInsertAndUpdate(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	task := newTask(t, workspaceID, "uno", models.StatusBacklog)
	task.Limits = models.ResourceLimits{MemoryMB: ptr(uint32(1024))}
	insertTask(t, repo, task)
	assert.Equal(t, task.Limits, found(t, repo, task.ID).Limits)

	updated, err := repo.Tasks.UpdateDetailsIfBacklog(bg(), task.ID, "uno", "p", models.DefaultTaskPriority, models.ResourceLimits{CPUPercent: ptr(uint32(200))}, "main")
	require.NoError(t, err)
	require.True(t, updated)
	assert.Equal(t, models.ResourceLimits{CPUPercent: ptr(uint32(200))}, found(t, repo, task.ID).Limits)
}

func TestUsageLimitScheduleRoundTripsAndIsClearedWhenLeavingRateLimited(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	task := newTask(t, workspaceID, "uno", models.StatusRateLimited)
	insertTask(t, repo, task)
	resumeAt := fixedClock.Add(time.Hour).Truncate(time.Microsecond)

	require.NoError(t, repo.Tasks.SetUsageLimit(bg(), task.ID, models.UsageLimitState{ResumeAt: &resumeAt, Attempts: 3}))
	got := found(t, repo, task.ID)
	require.NotNil(t, got.UsageLimit.ResumeAt)
	assert.True(t, got.UsageLimit.ResumeAt.Equal(resumeAt))
	assert.Equal(t, uint32(3), got.UsageLimit.Attempts)

	require.NoError(t, repo.Tasks.UpdateStatus(bg(), task.ID, models.StatusIdle, nil))
	left := found(t, repo, task.ID)
	assert.Nil(t, left.UsageLimit.ResumeAt)
	assert.Equal(t, uint32(3), left.UsageLimit.Attempts, "leaving rate_limited keeps the attempts")
}

func newTaskID(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return id
}

func dialogIDFor(op int) string {
	return "d" + string(rune('a'+op))
}

func dialogIDs(dialogs []models.PendingDialog) []string {
	ids := make([]string, 0, len(dialogs))
	for _, dialog := range dialogs {
		ids = append(ids, dialog.ID)
	}
	return ids
}

func titles(tasks []models.Task) []string {
	result := make([]string, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, task.Title)
	}
	return result
}

// Rust: task_repository_adapter_test::list_done_page_orders_by_completion_instead_of_creation
func TestListDonePageOrdersByCompletionInsteadOfCreation(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	base := fixedClock.Truncate(time.Microsecond)
	at := func(seconds int) time.Time { return base.Add(time.Duration(seconds) * time.Second) }

	createdFirst := newTask(t, workspaceID, "created-first", models.StatusMerged)
	createdFirst.CreatedAt = at(1)
	createdFirst.UpdatedAt = at(30)
	createdLast := newTask(t, workspaceID, "created-last", models.StatusMerged)
	createdLast.CreatedAt = at(10)
	createdLast.UpdatedAt = at(20)
	insertTask(t, repo, createdFirst)
	insertTask(t, repo, createdLast)

	page, err := repo.Tasks.ListDonePage(bg(), nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"created-first", "created-last"}, titles(page))
}

// Rust: task_repository_adapter_test::updated_at_is_frozen_after_closing_so_done_order_is_stable
// The clock advances between the steps, so a write that wrongly touches updated_at is visible.
func TestUpdatedAtIsFrozenAfterClosingSoDoneOrderIsStable(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/a")
	open := newTask(t, workspaceID, "open", models.StatusRunning)
	insertTask(t, repo, open)

	require.NoError(t, repo.Tasks.UpdateStatus(bg(), open.ID, models.StatusMerged, nil))
	closedAt := found(t, repo, open.ID).UpdatedAt

	repo.advance(time.Minute)
	require.NoError(t, repo.Tasks.RecordDiffRange(bg(), open.ID, models.DiffRange{BaseSHA: "aaa111", HeadSHA: "bbb222"}))
	repo.advance(time.Minute)
	require.NoError(t, repo.Tasks.SetSubagentActivity(bg(), open.ID, models.NewSubagentActivity(1, 0)))
	repo.advance(time.Minute)
	_, err := repo.Tasks.AppendEvent(bg(), open.ID, models.StatusKind(models.StatusMerged, nil))
	require.NoError(t, err)

	assert.True(t, closedAt.Equal(found(t, repo, open.ID).UpdatedAt), "closing sets updated_at; later writes must not move it")
}
