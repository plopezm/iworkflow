package tasks_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/tasks/domain/models"
)

// formPNG is a one-pixel PNG the create and edit forms upload.
var formPNG = mustBase64("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

func mustBase64(value string) []byte {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return data
}

// doMultipart posts a multipart form to the router, with the images in the images field.
func doMultipart(t *testing.T, router http.Handler, target string, fields url.Values, images map[string][]byte) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, values := range fields {
		for _, value := range values {
			require.NoError(t, writer.WriteField(name, value))
		}
	}
	for name, data := range images {
		part, err := writer.CreateFormFile("images", name)
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("HX-Request", "true")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder, recorder.Body.String()
}

// taskCount counts the tasks of the database, so a test can tell that a rejected form stored nothing.
func taskCount(t *testing.T, repo *Repo) int {
	t.Helper()
	var count int
	require.NoError(t, repo.DB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM tasks").Scan(&count))
	return count
}

// storedTitle reads the title of a task straight from the database.
func storedTitle(t *testing.T, repo *Repo, id models.TaskID) string {
	t.Helper()
	var title string
	require.NoError(t, repo.DB.QueryRowContext(context.Background(), "SELECT title FROM tasks WHERE id = ?", id.String()).Scan(&title))
	return title
}

// attachmentCount counts the images stored for a task.
func attachmentCount(t *testing.T, repo *Repo, id models.TaskID) int {
	t.Helper()
	rows, err := repo.Attachments.ListByTask(context.Background(), id)
	require.NoError(t, err)
	return len(rows)
}

// createFields are the fields of a valid create form for the workspace.
func createFields(workspaceID string) url.Values {
	return url.Values{
		"workspace_id": {workspaceID},
		"title":        {"Fix the login"},
		"prompt":       {"Investigate the failing login"},
		"priority":     {"high"},
	}
}

func TestCreateTaskWithAnImageStoresTheImage(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	recorder, body := doMultipart(t, router, "/board/tasks", createFields(workspaceID), map[string][]byte{"shot.png": formPNG})

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"))
	assert.NotContains(t, body, `name="prompt"`, "a created task closes the dialog")
	require.Equal(t, 1, taskCount(t, repo))
	var taskID string
	require.NoError(t, repo.DB.QueryRowContext(context.Background(), "SELECT id FROM tasks").Scan(&taskID))
	id, err := models.ParseTaskID(taskID)
	require.NoError(t, err)
	assert.Equal(t, 1, attachmentCount(t, repo, id))
}

func TestCreateTaskRejectsEveryInvalidFormAndStoresNothing(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		change func(url.Values)
		images map[string][]byte
		want   string
	}{
		"unknown priority":             {change: func(v url.Values) { v.Set("priority", "eventually") }, want: "eventually"},
		"memory is not a number":       {change: func(v url.Values) { v.Set("show_advanced", "on"); v.Set("memory_limit_mb", "abc") }, want: "la RAM máxima"},
		"cpu is zero":                  {change: func(v url.Values) { v.Set("show_advanced", "on"); v.Set("cpu_limit_percent", "0") }, want: "la CPU máxima"},
		"branch reads as a git option": {change: func(v url.Values) { v.Set("base_branch", "--upload-pack=x") }, want: "la rama base no es válida"},
		"workspace is not an id":       {change: func(v url.Values) { v.Set("workspace_id", "nope") }, want: "el workspace no es válido"},
		"empty prompt":                 {change: func(v url.Values) { v.Set("prompt", "") }, want: "el prompt es obligatorio"},
		"six images":                   {images: sixImages(), want: "como máximo 5 imágenes"},
		"text instead of an image": {
			images: map[string][]byte{"notes.png": []byte("just text, not a picture")},
			want:   "image/",
		},
		"image over the size limit": {
			images: map[string][]byte{"huge.png": append(append([]byte{}, formPNG...), make([]byte, 4*1024*1024)...)},
			want:   "huge.png",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := newRepo(t)
			workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
			_, router := boardRouter(t, repo)
			fields := createFields(workspaceID)
			if test.change != nil {
				test.change(fields)
			}
			images := test.images
			if images == nil {
				images = map[string][]byte{}
			}

			recorder, body := doMultipart(t, router, "/board/tasks", fields, images)

			require.Equal(t, http.StatusOK, recorder.Code, "htmx swaps only 2xx responses: the error is the fragment")
			assert.Empty(t, recorder.Header().Get("HX-Trigger"), "a rejected form does not refresh the board")
			assert.Contains(t, body, test.want)
			assert.Contains(t, body, `name="prompt"`, "the dialog stays open")
			assert.Equal(t, 0, taskCount(t, repo), "nothing is stored from a rejected form")
		})
	}
}

// sixImages are one more image than a message admits.
func sixImages() map[string][]byte {
	images := make(map[string][]byte, models.MaxImagesPerMessage+1)
	for range models.MaxImagesPerMessage + 1 {
		images[uuid.NewString()+".png"] = formPNG
	}
	return images
}

func TestCreateTaskIgnoresTheLimitsWhileTheAdvancedOptionsAreHidden(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	fields := createFields(workspaceID)
	fields.Set("memory_limit_mb", "abc")

	recorder, body := doMultipart(t, router, "/board/tasks", fields, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, body, "la RAM máxima", "the hidden field is not validated")
	assert.Equal(t, 1, taskCount(t, repo), "the task is created with the default limits")
}

func TestCreateLinkedTaskDependsOnTheAnchorAndWaits(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	anchor := newTask(t, workspaceID, "anchor", models.StatusBacklog)
	insertTask(t, repo, anchor)
	_, router := boardRouter(t, repo)
	fields := createFields(workspaceID)
	fields.Set("link_kind", "next")
	fields.Set("link_anchor", anchor.ID.String())
	fields.Set("start_now", "on")

	recorder, body := doMultipart(t, router, "/board/tasks", fields, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, body, "text-error", "the link was added")
	require.Equal(t, 2, taskCount(t, repo))
	dependencies, err := repo.Graph.ListDependencies(context.Background(), &workspaceID)
	require.NoError(t, err)
	require.Len(t, dependencies, 1)
	assert.Equal(t, anchor.ID, dependencies[0].Predecessor)
	var status string
	require.NoError(t, repo.DB.QueryRowContext(context.Background(), "SELECT status FROM tasks WHERE id <> ?", anchor.ID.String()).Scan(&status))
	assert.Equal(t, string(models.StatusBacklog), status, "a linked task never starts on creation")
}

func TestCreateLinkedTaskShowsAnAnchorThatIsNotATask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	fields := createFields(workspaceID)
	fields.Set("link_kind", "next")
	fields.Set("link_anchor", "not-a-task")

	_, body := doMultipart(t, router, "/board/tasks", fields, nil)

	assert.Contains(t, body, "la tarea enlazada no es válida")
	assert.Equal(t, 0, taskCount(t, repo))
}

func TestNewTaskDialogCarriesTheLinkOfTheAddress(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	anchor := uuid.NewString()
	_, router := boardRouter(t, repo)

	_, body := doRequest(t, router, http.MethodGet, "/board/tasks/new?workspace="+workspaceID+"&link_kind=parallel&link_anchor="+anchor, nil)

	assert.Contains(t, body, `name="link_kind" value="parallel"`)
	assert.Contains(t, body, `name="link_anchor" value="`+anchor+`"`)
	assert.NotContains(t, body, `name="start_now"`, "a linked dialog does not offer the start toggle")
}

func TestBranchesOfTheChosenWorkspaceFillTheSelect(t *testing.T) {
	repo := newRepo(t)
	dir := newGitRepository(t)
	gitCmd(t, dir, "branch", "rama-extra")
	workspaceID := insertWorkspace(t, repo.DB, dir)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodGet, "/board/tasks/branches?workspace_id="+workspaceID, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `id="board-branch-slot"`)
	assert.Contains(t, body, `value="rama-extra"`)
	assert.Contains(t, body, "Workspace branch (main)", "the workspace base branch is the first choice")
}

func TestBranchesOfAWorkspaceThatCannotBeReadSayWhy(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)

	_, body := doRequest(t, router, http.MethodGet, "/board/tasks/branches?workspace_id="+workspaceID, nil)

	assert.Contains(t, body, `id="board-branch-slot"`)
	assert.Contains(t, body, `text-error`, "the failure is shown, not an empty select")
}

// insertWithImage creates a backlog task with one stored image through the create form and returns its id.
func insertWithImage(t *testing.T, repo *Repo, router http.Handler, workspaceID string, title string) models.TaskID {
	t.Helper()
	fields := createFields(workspaceID)
	fields.Set("title", title)
	recorder, _ := doMultipart(t, router, "/board/tasks", fields, map[string][]byte{"first.png": formPNG})
	require.Equal(t, http.StatusOK, recorder.Code)
	var raw string
	require.NoError(t, repo.DB.QueryRowContext(context.Background(), "SELECT id FROM tasks WHERE title = ?", title).Scan(&raw))
	id, err := models.ParseTaskID(raw)
	require.NoError(t, err)
	return id
}

func TestEditTabOpensABacklogTaskWithItsFieldsAndImages(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	module, router := boardRouter(t, repo)
	id := insertWithImage(t, repo, router, workspaceID, "edit-me")

	page := renderTab(t, module, "task_edit", dock.Params{"id": id.String()})

	assert.Contains(t, page, "Edit task")
	assert.Contains(t, page, `value="edit-me"`)
	assert.Contains(t, page, "Investigate the failing login", "the prompt is loaded")
	assert.Contains(t, page, `name="remove_attachment_ids" value="`, "the stored image can be removed")
}

func TestEditTabRefusesATaskThatLeftBacklog(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "running", models.StatusRunning)
	insertTask(t, repo, running)
	module, _ := boardRouter(t, repo)

	page := renderTab(t, module, "task_edit", dock.Params{"id": running.ID.String()})

	assert.Contains(t, page, "This task can no longer be edited.")
	assert.NotContains(t, page, `name="prompt"`, "the form is not offered for a running task")
}

func TestUpdateSavesTheBacklogTaskAndLeavesThePage(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	id := insertWithImage(t, repo, router, workspaceID, "before")

	recorder, body := doMultipart(t, router, "/tasks/"+id.String()+"/update", url.Values{
		"title":    {"after"},
		"prompt":   {"new prompt"},
		"priority": {"urgent"},
	}, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "after", storedTitle(t, repo, id))
	assert.Contains(t, body, `data-dock-leave="/board/`+workspaceID+`"`, "the page closes and returns to the board")
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"))
}

func TestUpdateReplacesAStoredImageWithANewOne(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	id := insertWithImage(t, repo, router, workspaceID, "swap-image")
	stored, err := repo.Attachments.ListByTask(context.Background(), id)
	require.NoError(t, err)
	require.Len(t, stored, 1)

	recorder, _ := doMultipart(t, router, "/tasks/"+id.String()+"/update", url.Values{
		"title":                 {"swap-image"},
		"prompt":                {"p"},
		"remove_attachment_ids": {stored[0].ID},
	}, map[string][]byte{"second.png": formPNG})

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 1, attachmentCount(t, repo, id), "the removed image is gone and the new one is stored")
}

func TestUpdateRejectsAnInvalidFormAndKeepsTheStoredTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	_, router := boardRouter(t, repo)
	id := insertWithImage(t, repo, router, workspaceID, "keep-me")

	recorder, body := doMultipart(t, router, "/tasks/"+id.String()+"/update", url.Values{
		"title":           {"changed"},
		"prompt":          {"changed"},
		"show_advanced":   {"on"},
		"memory_limit_mb": {"lots"},
	}, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "la RAM máxima", "the error is shown in the page")
	assert.Equal(t, "keep-me", storedTitle(t, repo, id), "a rejected form changes nothing")
	assert.Empty(t, recorder.Header().Get("HX-Trigger"))
}

func TestUpdateOfARunningTaskIsRejectedAndChangesNothing(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "running-task", models.StatusRunning)
	insertTask(t, repo, running)
	_, router := boardRouter(t, repo)

	recorder, body := doMultipart(t, router, "/tasks/"+running.ID.String()+"/update", url.Values{
		"title":  {"sneaky"},
		"prompt": {"sneaky"},
	}, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "This task can no longer be edited.")
	assert.Equal(t, "running-task", storedTitle(t, repo, running.ID))
}

func TestStartFromTheCardShowsAFailureAndKeepsTheTaskInBacklog(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	backlog := newTask(t, workspaceID, "not-startable", models.StatusBacklog)
	insertTask(t, repo, backlog)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/board/tasks/"+backlog.ID.String()+"/start", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"), "the list refreshes either way")
	assert.Contains(t, body, `id="board-notice" hx-swap-oob="true"`)
	assert.Contains(t, body, `text-error`, "the start failure is shown")
	assert.Equal(t, string(models.StatusBacklog), statusOf(t, repo, backlog.ID))
}

func TestStartOfAnUnknownIdentifierIsRejected(t *testing.T) {
	t.Parallel()
	_, router := boardRouter(t, newRepo(t))

	recorder, body := doRequest(t, router, http.MethodPost, "/board/tasks/not-an-id/start", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "identificador de tarea inválido")
	assert.Empty(t, recorder.Header().Get("HX-Trigger"))
}

func TestDeleteFromTheCardRemovesABacklogTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	backlog := newTask(t, workspaceID, "delete-me", models.StatusBacklog)
	insertTask(t, repo, backlog)
	_, router := boardRouter(t, repo)

	recorder, _ := doRequest(t, router, http.MethodPost, "/board/tasks/"+backlog.ID.String()+"/delete", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"))
	assert.Equal(t, 0, taskCount(t, repo))
}

func TestDeleteOfARunningTaskIsRefused(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "keep-running", models.StatusRunning)
	insertTask(t, repo, running)
	_, router := boardRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/board/tasks/"+running.ID.String()+"/delete", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `text-error`)
	assert.Equal(t, 1, taskCount(t, repo))
}

func TestBacklogCardOffersOnlyTheActionsThatAreAllowed(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	backlog := newTask(t, workspaceID, "card-backlog", models.StatusBacklog)
	insertTask(t, repo, backlog)
	module, _ := boardRouter(t, repo)

	page := renderTab(t, module, "board", dock.Params{"id": workspaceID})

	assert.Contains(t, page, `hx-post="/board/tasks/`+backlog.ID.String()+`/start"`)
	assert.Contains(t, page, `href="/tasks/`+backlog.ID.String()+`/edit"`)
	assert.Contains(t, page, `role="button"`, "the edit control reads as a button, as the Rust card's does")
	assert.Contains(t, page, `hx-post="/board/tasks/`+backlog.ID.String()+`/delete"`)
	assert.Contains(t, page, `hx-confirm="Delete this task from the pending list?"`)
}

func statusOf(t *testing.T, repo *Repo, id models.TaskID) string {
	t.Helper()
	var status string
	require.NoError(t, repo.DB.QueryRowContext(context.Background(), "SELECT status FROM tasks WHERE id = ?", id.String()).Scan(&status))
	return status
}
