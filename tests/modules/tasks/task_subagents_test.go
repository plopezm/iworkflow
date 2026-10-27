package tasks_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// The subagents panel of a task: the detail offers it, and the panel answers the subagents the runtime reports.

func TestDetailOfARunningTaskOffersTheSubagentsPanel(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "subagents-offered", models.StatusRunning)
	insertTask(t, repo, running)
	module, _ := boardRouter(t, repo)
	id := running.ID.String()

	body := renderDetailTab(t, module, i18n.En, id)

	assert.Contains(t, body, `hx-get="/tasks/`+id+`/subagents`, "the entry asks for the panel of this task")
	assert.Contains(t, body, ">Subagents", "the entry has its label")
}

func TestSubagentsOfATaskWithoutAnAgentAnswerTheEmptyPanel(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "subagents-idle", models.StatusRunning)
	insertTask(t, repo, running)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, inLocale(router, i18n.Es), http.MethodGet, "/tasks/"+running.ID.String()+"/subagents?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `id="task-subagents-abc123"`, "the answer is the panel of this instance")
	assert.Contains(t, body, "Este agente no ha lanzado subagentes.")
	assert.NotContains(t, body, "todavía no se ha migrado")
}

func TestSubagentsOfAnUnknownTaskShowTheErrorInPlace(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodGet, "/tasks/"+newIDForTest(t)+"/subagents?scope=abc123", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `text-error`, "the lookup failure is shown")
}
