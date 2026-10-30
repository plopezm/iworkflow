package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/jira/infrastructure/di"
)

func TestConvertHandlers_GetConvertPage(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	comp := f.issueHandlers.ConvertPage(f.ctx(), "A-1")
	rec := httptest.NewRecorder()
	err := comp.Render(f.ctx(), rec)
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "Convertir A-1 en tarea de iworkflow")
	assert.Contains(t, body, "Revisa cómo se mapea la tarea de Jira")
	assert.Contains(t, body, `name="workspace_id"`)
	assert.Contains(t, body, `id="conversion-branch-container"`)
	assert.Contains(t, body, `name="title"`)
	assert.Contains(t, body, `name="prompt"`)
	assert.Contains(t, body, `name="priority"`)
	assert.Contains(t, body, `name="start_now"`)
	assert.Contains(t, body, "Crear tarea")
}

func TestConvertHandlers_GetConvertPageNotFound(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	comp := f.issueHandlers.ConvertPage(f.ctx(), "UNKNOWN-999")
	rec := httptest.NewRecorder()
	err := comp.Render(f.ctx(), rec)
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "UNKNOWN-999")
}

type emptyWorkspaces struct{}

func (emptyWorkspaces) Workspaces(context.Context) ([]models.WorkspaceOption, error) {
	return []models.WorkspaceOption{}, nil
}

func (emptyWorkspaces) Branches(context.Context, string) ([]models.BranchOption, error) {
	return []models.BranchOption{}, nil
}

func TestConvertHandlers_GetConvertPageNoWorkspaces(t *testing.T) {
	fake := newFakeJira(t)
	db := dbtest.New(t)
	tasks := &stubTasks{}
	module := di.NewModule(di.Deps{
		DB:         db,
		Tasks:      tasks,
		Workspaces: emptyWorkspaces{},
	})
	issueHandlers := inbound.NewIssueHandlers(
		module.GetIssue,
		module.ListComments,
		module.ListTransitions,
		module.AddComment,
		module.TransitionIssue,
		module.SetIssueIgnored,
		module.PrepareConversion,
		module.ListConversionBranches,
		module.ConvertIssue,
	)
	f := &issueFixture{t: t, module: module, fakeJira: fake, tasks: tasks, issueHandlers: issueHandlers}
	f.connectAndSync()

	comp := f.issueHandlers.ConvertPage(f.ctx(), "A-1")
	rec := httptest.NewRecorder()
	err := comp.Render(f.ctx(), rec)
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "Registra primero un workspace en")
	assert.Contains(t, body, "/workspaces")
	assert.Contains(t, body, "disabled")
}

func TestConvertHandlers_GetBranches(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	req := httptest.NewRequest(http.MethodGet, "/jira/A-1/convert/branches?workspace_id=w1", nil)
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, `id="conversion-branch-container"`)
	assert.Contains(t, body, `name="base_branch"`)
	assert.Contains(t, body, "main")
}

func TestConvertHandlers_ConvertSuccess(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	form := url.Values{}
	form.Set("workspace_id", "w1")
	form.Set("title", "Tarea convertida")
	form.Set("prompt", "Prompt para el agente")
	form.Set("priority", "high")
	form.Set("start_now", "true")

	req := httptest.NewRequest(http.MethodPost, "/jira/A-1/convert", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, "Tarea creada")
	assert.Contains(t, body, "/tasks/task-42")
	assert.Contains(t, body, "/jira/A-1")
	require.Len(t, f.tasks.created, 1)
	assert.Equal(t, "Tarea convertida", f.tasks.created[0].Title)
	assert.True(t, f.tasks.created[0].StartNow)
}

func TestConvertHandlers_ConvertError(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	// Empty title is invalid
	form := url.Values{}
	form.Set("workspace_id", "w1")
	form.Set("title", "  ")
	form.Set("prompt", "Prompt válido")

	req := httptest.NewRequest(http.MethodPost, "/jira/A-1/convert", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, "el título de la tarea no puede estar vacío")

	// Convert again when already converted
	formValid := url.Values{}
	formValid.Set("workspace_id", "w1")
	formValid.Set("title", "Título válido")
	formValid.Set("prompt", "Prompt válido")

	req1 := httptest.NewRequest(http.MethodPost, "/jira/A-1/convert", strings.NewReader(formValid.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res1 := f.serve(req1)
	assert.Equal(t, http.StatusOK, res1.Code)
	assert.Contains(t, res1.Body.String(), "Tarea creada")

	// Now submitting again should report already converted error
	req2 := httptest.NewRequest(http.MethodPost, "/jira/A-1/convert", strings.NewReader(formValid.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res2 := f.serve(req2)
	assert.Equal(t, http.StatusOK, res2.Code)
	assert.Contains(t, res2.Body.String(), "ya se convirtió")
}

type stubTasksWithStartError struct{}

func (stubTasksWithStartError) Create(_ context.Context, _ outbound.NewLinkedTask) (outbound.CreatedLinkedTask, error) {
	errStr := "agente no disponible"
	return outbound.CreatedLinkedTask{TaskID: "task-99", StartError: &errStr}, nil
}

func TestConvertHandlers_ConvertWithStartError(t *testing.T) {
	fake := newFakeJira(t)
	db := dbtest.New(t)
	module := di.NewModule(di.Deps{
		DB:         db,
		Tasks:      stubTasksWithStartError{},
		Workspaces: stubWorkspaces{},
	})
	issueHandlers := inbound.NewIssueHandlers(
		module.GetIssue,
		module.ListComments,
		module.ListTransitions,
		module.AddComment,
		module.TransitionIssue,
		module.SetIssueIgnored,
		module.PrepareConversion,
		module.ListConversionBranches,
		module.ConvertIssue,
	)
	router := chi.NewRouter()
	module.Routes(router)
	issueHandlers.Mount(router)
	f := &issueFixture{t: t, router: router, module: module, fakeJira: fake, tasks: &stubTasks{}, issueHandlers: issueHandlers}
	f.connectAndSync()

	form := url.Values{}
	form.Set("workspace_id", "w1")
	form.Set("title", "Tarea con error de inicio")
	form.Set("prompt", "Prompt")

	req := httptest.NewRequest(http.MethodPost, "/jira/A-1/convert", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, "Tarea creada")
	assert.Contains(t, body, "agente no disponible")
	assert.Contains(t, body, "/tasks/task-99")
}
