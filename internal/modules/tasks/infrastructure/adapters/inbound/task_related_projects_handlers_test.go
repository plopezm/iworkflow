package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

type setRelatedFake struct {
	calls int
	ids   []string
	err   error
}

func (f *setRelatedFake) Execute(_ context.Context, _ models.TaskID, ids []string) (application.SetRelatedProjectsResult, error) {
	f.calls++
	f.ids = ids
	return application.SetRelatedProjectsResult{}, f.err
}

type listRelatedFake struct{ stored []models.WorkspaceRef }

func (f listRelatedFake) Execute(context.Context, models.TaskID) ([]models.WorkspaceRef, error) {
	return f.stored, nil
}

type workspacesFake struct{ all []models.WorkspaceRef }

func (f workspacesFake) List(context.Context) ([]models.WorkspaceRef, error) { return f.all, nil }

func relatedDetail(t *testing.T, setter *setRelatedFake) (*DetailHandlers, models.Task) {
	t.Helper()
	task := newDiffTaskForTest(t)
	task.Status = models.StatusRunning
	api := models.WorkspaceRef{ID: "ws-api", Name: "api", Path: "/repos/api"}
	web := models.WorkspaceRef{ID: "ws-web", Name: "web", Path: "/repos/web"}
	own := models.WorkspaceRef{ID: task.WorkspaceID, Name: "own", Path: "/repos/own"}
	return NewDetailHandlers(DetailUseCases{
		Get:                 &tabsTaskFake{task: task},
		Events:              detailHistoryFake{},
		ListAttachments:     detailAttachmentsFake{},
		SetRelatedProjects:  setter,
		ListRelatedProjects: listRelatedFake{stored: []models.WorkspaceRef{api}},
		Workspaces:          workspacesFake{all: []models.WorkspaceRef{own, api, web}},
	}, time.Now), task
}

func postRelated(t *testing.T, h *DetailHandlers, task models.Task, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	h.Mount(router)
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+task.ID.String()+"/related-projects?scope=abc123", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(inLanguage(i18n.En))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestSetRelatedProjectsSendsTheCheckedProjectsAndAnswersThePage(t *testing.T) {
	t.Parallel()
	setter := &setRelatedFake{}
	h, task := relatedDetail(t, setter)

	rec := postRelated(t, h, task, url.Values{"related_project_id": {"ws-api", "ws-web"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"ws-api", "ws-web"}, setter.ids)
	assert.Contains(t, rec.Body.String(), `id="task-detail-abc123"`)
}

func TestSetRelatedProjectsWithNothingCheckedClearsTheList(t *testing.T) {
	t.Parallel()
	setter := &setRelatedFake{}
	h, task := relatedDetail(t, setter)

	rec := postRelated(t, h, task, url.Values{})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, setter.calls)
	assert.Empty(t, setter.ids)
}

func TestSetRelatedProjectsShowsARefusalInThePage(t *testing.T) {
	t.Parallel()
	setter := &setRelatedFake{err: errors.New("proyecto relacionado inválido")}
	h, task := relatedDetail(t, setter)

	rec := postRelated(t, h, task, url.Values{"related_project_id": {"x"}})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "proyecto relacionado inválido")
	assert.Contains(t, rec.Body.String(), "text-error")
}

func TestActionsTabOffersTheRelatedProjectsChooserWithoutTheOwnProject(t *testing.T) {
	t.Parallel()
	h, task := relatedDetail(t, &setRelatedFake{})

	rec := postRelated(t, h, task, url.Values{})
	body := rec.Body.String()

	assert.Contains(t, body, "data-task-related-projects")
	assert.Contains(t, body, `hx-post="/tasks/`+task.ID.String()+`/related-projects?scope=abc123"`)
	assert.Contains(t, body, `value="ws-web"`)
	assert.Regexp(t, `value="ws-api"[^>]*checked`, body, "the stored project is marked")
	assert.NotContains(t, body, `value="`+task.WorkspaceID+`"`, "a task does not relate to its own project")
}

func TestActionsTabHidesTheChooserOfAClosedTask(t *testing.T) {
	t.Parallel()
	h, task := relatedDetail(t, &setRelatedFake{})
	task.Status = models.StatusMerged
	h.uc.Get = &tabsTaskFake{task: task}

	rec := postRelated(t, h, task, url.Values{})

	assert.NotContains(t, rec.Body.String(), "data-task-related-projects")
}
