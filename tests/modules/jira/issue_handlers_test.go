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
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/jira/infrastructure/di"
)

type issueFixture struct {
	t             *testing.T
	router        chi.Router
	module        *di.Module
	fakeJira      *fakeJira
	tasks         *stubTasks
	issueHandlers *inbound.IssueHandlers
}

func newIssueFixture(t *testing.T) *issueFixture {
	t.Helper()
	fake := newFakeJira(t)
	db := dbtest.New(t)
	tasks := &stubTasks{}
	module := di.NewModule(di.Deps{
		DB:         db,
		Tasks:      tasks,
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

	return &issueFixture{
		t:             t,
		router:        router,
		module:        module,
		fakeJira:      fake,
		tasks:         tasks,
		issueHandlers: issueHandlers,
	}
}

func (f *issueFixture) ctx() context.Context {
	state := i18n.State{Locale: i18n.Es, Chosen: true}
	return i18n.WithState(context.Background(), state)
}

func (f *issueFixture) serve(req *http.Request) *httptest.ResponseRecorder {
	reqWithLang := req.WithContext(f.ctx())
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, reqWithLang)
	return rec
}

func (f *issueFixture) connectAndSync() {
	ctx := context.Background()
	_, err := f.module.SaveConnection.Execute(ctx, application.SaveConnectionInput{
		BaseURL: f.fakeJira.url(),
		Token:   testToken,
	})
	require.NoError(f.t, err)

	_, err = f.module.SyncIssues.Execute(ctx)
	require.NoError(f.t, err)
}

func TestIssueHandlers_GetIssueDetail(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	comp := f.issueHandlers.IssuePage(f.ctx(), "A-1")
	rec := httptest.NewRecorder()
	err := comp.Render(f.ctx(), rec)
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "A-1")
	assert.Contains(t, body, "id=\"jira-issue-container\"")
	assert.Contains(t, body, "Luis")          // comment author from fake Jira
	assert.Contains(t, body, "Cerrar → Done") // transition from fake Jira
}

func TestIssueHandlers_GetIssueNotFound(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	comp := f.issueHandlers.IssuePage(f.ctx(), "NONEXISTENT-999")
	rec := httptest.NewRecorder()
	err := comp.Render(f.ctx(), rec)
	require.NoError(t, err)

	body := rec.Body.String()
	assert.Contains(t, body, "NONEXISTENT-999")
}

func TestIssueHandlers_AddComment(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	form := url.Values{}
	form.Set("body", "Comentario de prueba")
	req := httptest.NewRequest(http.MethodPost, "/jira/A-1/comments", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "id=\"jira-issue-container\"")

	// Empty comment error
	emptyForm := url.Values{}
	emptyForm.Set("body", "   ")
	reqEmpty := httptest.NewRequest(http.MethodPost, "/jira/A-1/comments", strings.NewReader(emptyForm.Encode()))
	reqEmpty.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqEmpty.Header.Set("HX-Request", "true")
	resEmpty := f.serve(reqEmpty)

	assert.Equal(t, http.StatusOK, resEmpty.Code)
	assert.Contains(t, resEmpty.Body.String(), "el comentario no puede estar vacío")
}

func TestIssueHandlers_Transition(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	form := url.Values{}
	form.Set("transition_id", "31")
	req := httptest.NewRequest(http.MethodPost, "/jira/A-1/transitions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "id=\"jira-issue-container\"")

	// Transition error on unknown issue or failure
	reqErr := httptest.NewRequest(http.MethodPost, "/jira/A-1/transitions", strings.NewReader("transition_id=bogus"))
	reqErr.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqErr.Header.Set("HX-Request", "true")
	resErr := f.serve(reqErr)
	assert.Equal(t, http.StatusOK, resErr.Code)
}

func TestIssueHandlers_IgnoreToggle(t *testing.T) {
	f := newIssueFixture(t)
	f.connectAndSync()

	// Ignore issue
	req := httptest.NewRequest(http.MethodPost, "/jira/A-1/ignore?ignored=true", nil)
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "Restaurar")

	// Restore issue
	reqRestore := httptest.NewRequest(http.MethodPost, "/jira/A-1/ignore?ignored=false", nil)
	reqRestore.Header.Set("HX-Request", "true")
	resRestore := f.serve(reqRestore)

	assert.Equal(t, http.StatusOK, resRestore.Code)
	assert.Contains(t, resRestore.Body.String(), "Ignorar")
}
