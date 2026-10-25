package tasks_test

import (
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// The detail header: the status badge, the branch line and the failures of the task.

func TestDetailOfABacklogTaskOffersStartEditAndDeleteWithItsPrompt(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "backlog-detail", models.StatusBacklog)
	insertTask(t, repo, task)
	module, _ := boardRouter(t, repo)
	id := task.ID.String()

	body := renderDetailTab(t, module, i18n.En, id)

	assert.Contains(t, body, "To do", "the status badge")
	assert.Contains(t, body, "haz algo", "the prompt of the task")
	assert.Contains(t, body, "/tasks/"+id+"/start", "start is offered from backlog")
	assert.Contains(t, body, `href="/tasks/`+id+`/edit"`, "edit is offered from backlog")
	assert.Contains(t, body, `hx-confirm="Delete this task from the to-do list?"`, "delete asks first")
	assert.NotContains(t, body, "Branch ", "a backlog task has no branch yet")
}

func TestDetailOfARunningTaskShowsItsShortBranchAndNoBacklogActions(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "running-detail", models.StatusRunning)
	task.Branch = "pi/fix-login-3f2a9c1e-5b7d-4e8a-9c0b-1a2b3c4d5e6f"
	insertTask(t, repo, task)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.En, task.ID.String())

	assert.Contains(t, body, "Working", "the status badge of a running task")
	assert.Contains(t, body, "Branch ")
	assert.Contains(t, body, `<code class="font-mono" title="pi/fix-login-3f2a9c1e-5b7d-4e8a-9c0b-1a2b3c4d5e6f">pi/fix-login</code>`,
		"the uuid suffix is dropped from the shown branch and kept in its title")
	assert.Contains(t, body, " from ")
	assert.Contains(t, body, `<code class="font-mono">main</code>`, "the base branch")
	assert.NotContains(t, body, "Delete this task", "delete is only offered from backlog")
	assert.NotContains(t, body, "/start")
}

func TestDetailShowsTheTaskErrorAndTheUsageLimitResume(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "limited-detail", models.StatusRateLimited)
	failure := "el agente terminó inesperadamente (código 1)"
	task.Error = &failure
	resumeAt := time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC)
	task.UsageLimit = models.UsageLimitState{ResumeAt: &resumeAt}
	insertTask(t, repo, task)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.En, task.ID.String())

	assert.Contains(t, body, "Usage limit")
	assert.Contains(t, body, "Resumes automatically at 2026-10-25 14:30 UTC")
	assert.Contains(t, body, failure)
}

func TestStatusBadgeNamesEveryStatusInTheRequestLanguage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status  models.TaskStatus
		en      string
		es      string
		variant string
	}{
		{models.StatusBacklog, "To do", "Por hacer", "badge-muted"},
		{models.StatusStarting, "Starting", "Arrancando", "badge-info"},
		{models.StatusRunning, "Working", "Trabajando", "badge-info"},
		{models.StatusIdle, "Waiting", "Esperando", "badge-warn"},
		{models.StatusFinishing, "Finishing", "Finalizando", "badge-info"},
		{models.StatusAwaitingReview, "Awaiting review", "Pendiente de revisión", "badge-warn"},
		{models.StatusMerged, "Merged", "Integrada", "badge-ok"},
		{models.StatusDiscarded, "Discarded", "Descartada", "badge-muted"},
		{models.StatusConflict, "Conflict", "Conflicto", "badge-error"},
		{models.StatusFailed, "Failed", "Fallida", "badge-error"},
		{models.StatusInterrupted, "Interrupted", "Interrumpida", "badge-error"},
		{models.StatusRateLimited, "Usage limit", "Límite de uso", "badge-warn"},
	}
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	module, _ := boardRouter(t, repo)
	for _, tc := range cases {
		task := newTask(t, workspaceID, "badge-"+string(tc.status), tc.status)
		insertTask(t, repo, task)

		assert.Contains(t, renderDetailTab(t, module, i18n.En, task.ID.String()), tc.en+"</span>", string(tc.status))
		assert.Contains(t, renderDetailTab(t, module, i18n.Es, task.ID.String()), tc.es+"</span>", string(tc.status))
		assert.Contains(t, renderDetailTab(t, module, i18n.En, task.ID.String()), tc.variant, string(tc.status))
	}
}

func TestStatusBadgeOfARunningTaskIsWrittenInEveryShippedLanguage(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "running-languages", models.StatusRunning)
	insertTask(t, repo, task)
	module, _ := boardRouter(t, repo)
	wants := map[i18n.Locale]string{
		i18n.En: "Working",
		i18n.Es: "Trabajando",
		i18n.Zh: "工作中",
		i18n.De: "Arbeitet",
		i18n.Fr: "En cours",
	}
	for locale, want := range wants {
		assert.Contains(t, renderDetailTab(t, module, locale, task.ID.String()), want, string(locale))
	}
}

// The backlog actions of the detail page answer the same page instance and leave it when the task is gone.

func TestStartFromTheDetailStartsTheTaskAndAnswersThePageOfThisInstance(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("the fake agent needs python3")
	}
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, gitWorkspace(t))
	backlog := newTask(t, workspaceID, "detail-startable", models.StatusBacklog)
	insertTask(t, repo, backlog)
	router := startRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/tasks/"+backlog.ID.String()+"/start?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"), "the board refreshes so the card moves")
	assert.Contains(t, body, `id="task-detail-abc123"`, "the answer replaces the page of this instance")
	assert.NotContains(t, body, `text-error`, "a started task shows no failure")
	assert.NotEqual(t, string(models.StatusBacklog), statusOf(t, repo, backlog.ID), "the task left backlog")
}

func TestDeleteFromTheDetailRemovesABacklogTaskAndClosesItsTab(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	backlog := newTask(t, workspaceID, "detail-deletable", models.StatusBacklog)
	insertTask(t, repo, backlog)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/tasks/"+backlog.ID.String()+"/delete?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"))
	assert.Contains(t, body, `data-dock-leave="/board/`+workspaceID+`"`, "the tab closes and opens the board")
	assert.Equal(t, 0, taskCount(t, repo))
}

func TestDeleteOfARunningTaskFromTheDetailKeepsItAndSaysWhy(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "detail-running", models.StatusRunning)
	insertTask(t, repo, running)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/tasks/"+running.ID.String()+"/delete?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `text-error`, "the refusal is shown")
	assert.NotContains(t, body, "data-dock-leave", "the tab stays open")
	assert.Equal(t, 1, taskCount(t, repo))
}

func TestStartFromTheDetailShowsWhyATaskDidNotStartAndKeepsItInBacklog(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	// A folder that is not a repository: the worktree of the task cannot be created.
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	backlog := newTask(t, workspaceID, "detail-unstartable", models.StatusBacklog)
	insertTask(t, repo, backlog)
	router := startRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/tasks/"+backlog.ID.String()+"/start?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `id="task-detail-abc123"`)
	assert.Contains(t, body, `text-error`, "the failure of the start is shown on the page")
	assert.Equal(t, string(models.StatusBacklog), statusOf(t, repo, backlog.ID), "the task stays in backlog")
}

func TestDetailOfALimitedTaskWithoutAResumeTimeShowsNoResumeLine(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	limited := newTask(t, workspaceID, "limited-no-time", models.StatusRateLimited)
	insertTask(t, repo, limited)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.En, limited.ID.String())

	assert.Contains(t, body, "Usage limit")
	assert.NotContains(t, body, "Resumes automatically at")
}
