package tasks_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/tasks/domain/models"
)

// relatedTaskID reads the id of the only task of the database.
func relatedTaskID(t *testing.T, repo *Repo) models.TaskID {
	t.Helper()
	var raw string
	require.NoError(t, repo.DB.QueryRowContext(context.Background(), "SELECT id FROM tasks").Scan(&raw))
	id, err := models.ParseTaskID(raw)
	require.NoError(t, err)
	return id
}

// storedRelated reads the related workspaces of a task, in the order they are stored.
func storedRelated(t *testing.T, repo *Repo, id models.TaskID) []string {
	t.Helper()
	ids, err := repo.RelatedProjects.ListByTask(context.Background(), id)
	require.NoError(t, err)
	return ids
}

// checkedProject is the opening tag of a related project checkbox that is checked, as the page renders it.
func checkedProject(workspaceID string) string {
	return `data-workspace-id="` + workspaceID + `" class="h-5 w-5" checked>`
}

// uncheckedProject is the opening tag of a related project checkbox that is not checked.
func uncheckedProject(workspaceID string) string {
	return `data-workspace-id="` + workspaceID + `" class="h-5 w-5">`
}

func TestCreateTaskStoresTheChosenRelatedProjectsInOrder(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	own := insertWorkspace(t, repo.DB, t.TempDir())
	api := insertWorkspace(t, repo.DB, t.TempDir())
	web := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	fields := createFields(own)
	fields["related_project_id"] = []string{web, api}

	recorder, _ := doMultipart(t, router, "/board/tasks", fields, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, taskCount(t, repo))
	assert.Equal(t, []string{web, api}, storedRelated(t, repo, relatedTaskID(t, repo)))
}

func TestCreateTaskRefusesAnInvalidRelatedProjectAndKeepsTheTab(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	own := insertWorkspace(t, repo.DB, t.TempDir())
	api := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	fields := createFields(own)
	fields["related_project_id"] = []string{api, "no-es-uuid"}

	recorder, body := doMultipart(t, router, "/board/tasks", fields, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Header().Get("HX-Trigger"), "a rejected form does not refresh the board")
	assert.Contains(t, body, "proyecto relacionado inválido: no-es-uuid", "the error is shown in the create tab")
	assert.NotContains(t, body, "data-dock-leave", "the tab stays open, with the choices the user made")
	assert.Equal(t, 0, taskCount(t, repo), "nothing is stored from a rejected form")
}

// An unknown related workspace is refused before anything is written: no task row is stored.
func TestCreateTaskWithAnUnknownRelatedProjectStoresNoTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	own := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	fields := createFields(own)
	fields["related_project_id"] = []string{uuid.NewString()}

	recorder, _ := doMultipart(t, router, "/board/tasks", fields, nil)

	assert.NotEqual(t, http.StatusCreated, recorder.Code)
	assert.Empty(t, recorder.Header().Get("HX-Trigger"), "a rejected form does not refresh the board")
	assert.Equal(t, 0, taskCount(t, repo), "no task row is stored for an unknown related workspace")
}

func TestEditPageMarksTheStoredRelatedProjectsAndHidesTheOwnWorkspace(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	own := insertWorkspace(t, repo.DB, t.TempDir())
	api := insertWorkspace(t, repo.DB, t.TempDir())
	web := insertWorkspace(t, repo.DB, t.TempDir())
	module, router := boardRouter(t, repo)
	fields := createFields(own)
	fields["related_project_id"] = []string{api}
	doMultipart(t, router, "/board/tasks", fields, nil)
	id := relatedTaskID(t, repo)

	page := renderTab(t, module, "task_edit", dock.Params{"id": id.String()})

	assert.Contains(t, page, checkedProject(api), "the stored project is pre-checked")
	assert.Contains(t, page, uncheckedProject(web), "the other projects are offered unchecked")
	assert.NotContains(t, page, `data-workspace-id="`+own+`"`, "the task's own workspace is not offered")
}

func TestUpdateTaskReplacesTheRelatedProjects(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	own := insertWorkspace(t, repo.DB, t.TempDir())
	api := insertWorkspace(t, repo.DB, t.TempDir())
	web := insertWorkspace(t, repo.DB, t.TempDir())
	module, router := boardRouter(t, repo)
	fields := createFields(own)
	fields["related_project_id"] = []string{api}
	doMultipart(t, router, "/board/tasks", fields, nil)
	id := relatedTaskID(t, repo)

	recorder, _ := doMultipart(t, router, "/tasks/"+id.String()+"/update", url.Values{
		"title":              {"changed"},
		"prompt":             {"changed"},
		"related_project_id": {web},
	}, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{web}, storedRelated(t, repo, id), "the save replaces the list, it does not add to it")
	page := renderTab(t, module, "task_edit", dock.Params{"id": id.String()})
	assert.Contains(t, page, checkedProject(web))
	assert.Contains(t, page, uncheckedProject(api))
}

// Saving the edit page with no related project checked clears the stored list: the form sends no related_project_id.
func TestUpdateTaskWithNoCheckedProjectClearsTheStoredOnes(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	own := insertWorkspace(t, repo.DB, t.TempDir())
	api := insertWorkspace(t, repo.DB, t.TempDir())
	module, router := boardRouter(t, repo)
	fields := createFields(own)
	fields["related_project_id"] = []string{api}
	doMultipart(t, router, "/board/tasks", fields, nil)
	id := relatedTaskID(t, repo)

	recorder, _ := doMultipart(t, router, "/tasks/"+id.String()+"/update", url.Values{
		"title":  {"changed"},
		"prompt": {"changed"},
	}, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, storedRelated(t, repo, id), "unticking every project and saving clears the list")
	page := renderTab(t, module, "task_edit", dock.Params{"id": id.String()})
	assert.NotContains(t, page, checkedProject(api), "the cleared project is not pre-checked any more")
}

// A task created with no related project checked stores none.
func TestCreateTaskWithNoCheckedProjectStoresNone(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	own := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	recorder, _ := doMultipart(t, router, "/board/tasks", createFields(own), nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, taskCount(t, repo))
	assert.Empty(t, storedRelated(t, repo, relatedTaskID(t, repo)))
}

func TestUpdateTaskWithAnInvalidRelatedProjectKeepsTheStoredOnes(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	own := insertWorkspace(t, repo.DB, t.TempDir())
	api := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	fields := createFields(own)
	fields["related_project_id"] = []string{api}
	doMultipart(t, router, "/board/tasks", fields, nil)
	id := relatedTaskID(t, repo)

	_, body := doMultipart(t, router, "/tasks/"+id.String()+"/update", url.Values{
		"title":              {"changed"},
		"prompt":             {"changed"},
		"related_project_id": {"no-es-uuid"},
	}, nil)

	assert.Contains(t, body, "proyecto relacionado inválido: no-es-uuid")
	assert.Equal(t, []string{api}, storedRelated(t, repo, id), "a rejected save keeps the stored projects")
	assert.Equal(t, "Fix the login", storedTitle(t, repo, id), "the rest of the task is not saved either")
}
