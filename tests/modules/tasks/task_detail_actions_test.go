package tasks_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// The actions tab of the detail page: what a task offers by its status, and the answers of the actions.

func TestDetailOfARunningTaskOffersItsAgentAndDangerActionsWithTheirConfirmations(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "actions-running", models.StatusRunning)
	insertTask(t, repo, running)
	module, _ := boardRouter(t, repo)
	id := running.ID.String()

	body := renderDetailTab(t, module, i18n.En, id)

	assert.Contains(t, body, ">Actions<", "the actions tab has its heading")
	assert.Contains(t, body, `hx-post="/tasks/`+id+`/abort`, "a live agent can be aborted")
	assert.Contains(t, body, `hx-post="/tasks/`+id+`/finish`, "a running task can be finished")
	assert.Contains(t, body, `hx-confirm="Finish the task? The agent will be closed and the changes will be consolidated for review."`)
	assert.Contains(t, body, `hx-post="/tasks/`+id+`/reset`, "a running task can be reset")
	assert.Contains(t, body, `hx-confirm="Reset the task? The agent will be stopped, the worktree, the branch and the history will be deleted, and it will go back to Backlog."`)
	assert.Contains(t, body, `hx-post="/tasks/`+id+`/discard`, "a running task can be discarded")
	assert.Contains(t, body, `hx-confirm="Discard the task? The worktree and the unmerged branch will be deleted."`)
	assert.NotContains(t, body, `hx-post="/tasks/`+id+`/resume`, "resume is for stopped tasks")
}

func TestDetailOfAReviewedTaskOffersResumeAndNoAgentStop(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	reviewed := newTask(t, workspaceID, "actions-review", models.StatusAwaitingReview)
	insertTask(t, repo, reviewed)
	module, _ := boardRouter(t, repo)
	id := reviewed.ID.String()

	body := renderDetailTab(t, module, i18n.En, id)

	assert.Contains(t, body, `hx-post="/tasks/`+id+`/resume`)
	assert.Contains(t, body, `hx-post="/tasks/`+id+`/reset`)
	assert.Contains(t, body, `hx-post="/tasks/`+id+`/discard`)
	assert.NotContains(t, body, `hx-post="/tasks/`+id+`/abort`, "no live agent to abort")
	assert.NotContains(t, body, `hx-post="/tasks/`+id+`/finish`, "an awaiting review task is not finished again")
}

func TestDetailOfAMergedTaskOffersNoAgentOrDangerAction(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	merged := newTask(t, workspaceID, "actions-merged", models.StatusMerged)
	insertTask(t, repo, merged)
	module, _ := boardRouter(t, repo)
	id := merged.ID.String()

	body := renderDetailTab(t, module, i18n.En, id)

	for _, action := range []string{"abort", "finish", "resume", "reset", "discard"} {
		assert.NotContains(t, body, `hx-post="/tasks/`+id+`/`+action, action)
	}
}

func TestDetailShowsTheInitialPromptTheBaseBranchAndTheWorktree(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "actions-context", models.StatusRunning)
	insertTask(t, repo, running)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.En, running.ID.String())

	assert.Contains(t, body, "Initial prompt")
	assert.Contains(t, body, "haz algo", "the prompt the task started with")
	assert.Contains(t, body, "Starting branch")
	assert.Contains(t, body, "main", "the base branch")
	assert.Contains(t, body, "Worktree")
	assert.Contains(t, body, "/wt/actions-context", "the worktree path")
}

// The images are shown once the task left backlog, as the Rust actions tab does; a backlog task edits them instead.
func TestDetailShowsTheAttachedImagesAsThumbnails(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	module, router := boardRouter(t, repo)
	id := insertWithImage(t, repo, router, workspaceID, "con-imagen").String()
	_, err := repo.DB.ExecContext(bg(), "UPDATE tasks SET status = ? WHERE id = ?", string(models.StatusAwaitingReview), id)
	require.NoError(t, err)

	body := renderDetailTab(t, module, i18n.En, id)

	assert.Contains(t, body, "Attachments")
	assert.Contains(t, body, `src="/api/tasks/`+id+`/attachments/`, "the thumbnail is the stored image")
	assert.Contains(t, body, `alt="first.png"`)
}

func TestAbortOfATaskWithoutALiveAgentFromTheDetailSaysWhy(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	reviewed := newTask(t, workspaceID, "abort-refused", models.StatusAwaitingReview)
	insertTask(t, repo, reviewed)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/tasks/"+reviewed.ID.String()+"/abort?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `id="task-detail-abc123"`, "the answer is the page of this instance")
	assert.Contains(t, body, `text-error`, "the refusal is shown")
	assert.Equal(t, string(models.StatusAwaitingReview), statusOf(t, repo, reviewed.ID))
}

func TestFinishAndResumeOfABacklogTaskFromTheDetailAreRefused(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	backlog := newTask(t, workspaceID, "refused-backlog", models.StatusBacklog)
	insertTask(t, repo, backlog)
	_, router := boardRouter(t, repo)

	for _, action := range []string{"finish", "resume"} {
		recorder, body := doRequest(t, router, http.MethodPost, "/tasks/"+backlog.ID.String()+"/"+action+"?scope=abc123", nil)

		require.Equal(t, http.StatusOK, recorder.Code, action)
		assert.Contains(t, body, `text-error`, action)
		assert.Equal(t, string(models.StatusBacklog), statusOf(t, repo, backlog.ID), action)
	}
}

func TestResetFromTheDetailReturnsTheTaskToBacklogAndClosesTheTab(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	reviewed := newTask(t, workspaceID, "reset-me", models.StatusAwaitingReview)
	insertTask(t, repo, reviewed)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/tasks/"+reviewed.ID.String()+"/reset?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"))
	assert.Contains(t, body, `data-dock-leave="/board/`+workspaceID+`"`, "the tab closes and opens the board")
	assert.Equal(t, string(models.StatusBacklog), statusOf(t, repo, reviewed.ID))
}

func TestDiscardFromTheDetailDiscardsTheTaskAndAnswersThePage(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	reviewed := newTask(t, workspaceID, "discard-me", models.StatusAwaitingReview)
	insertTask(t, repo, reviewed)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, inLocale(router, i18n.Es), http.MethodPost, "/tasks/"+reviewed.ID.String()+"/discard?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `id="task-detail-abc123"`, "the tab stays and shows the discarded task")
	assert.NotContains(t, body, "data-dock-leave")
	assert.Equal(t, string(models.StatusDiscarded), statusOf(t, repo, reviewed.ID))
	assert.Contains(t, body, "Descartada", "the badge of the discarded task")
}

func TestResetOfAMergedTaskFromTheDetailIsRefusedAndTheTabStays(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	merged := newTask(t, workspaceID, "reset-merged", models.StatusMerged)
	insertTask(t, repo, merged)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/tasks/"+merged.ID.String()+"/reset", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `text-error`, "a merge cannot be undone")
	assert.NotContains(t, body, "data-dock-leave", "the tab stays open")
	assert.Equal(t, string(models.StatusMerged), statusOf(t, repo, merged.ID))
}
