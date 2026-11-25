package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspace_branches/application"
	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	"iworkflow/internal/modules/workspace_branches/domain/models"
)

type stubList struct {
	list    application.BranchList
	err     error
	filters []string
}

func (s *stubList) Execute(_ context.Context, _ string, filter string) (application.BranchList, error) {
	s.filters = append(s.filters, filter)
	return s.list, s.err
}

type stubDelete struct {
	results []application.DeleteResult
	err     error
	refs    [][]models.Ref
}

func (s *stubDelete) Execute(_ context.Context, _ string, refs []models.Ref) ([]application.DeleteResult, error) {
	s.refs = append(s.refs, refs)
	return s.results, s.err
}

// spanishContext is the request language the screens are checked in, as the workspaces tests do.
func spanishContext(ctx context.Context) context.Context {
	return i18n.WithState(ctx, i18n.State{Locale: i18n.Es, Chosen: true})
}

func sampleList() application.BranchList {
	return application.BranchList{
		Branches: []models.Branch{
			{Name: "feature/old"},
			{Name: "main", Current: true, CheckedOut: true, Protected: true, Reason: models.ReasonCheckedOut},
			{Name: "gone", Remote: true, RemoteName: "origin"},
		},
	}
}

func newRouter(list listUseCase, del deleteUseCase) chi.Router {
	r := chi.NewRouter()
	NewHandlers(list, del).Mount(r)
	return r
}

func serve(t *testing.T, r http.Handler, req *http.Request) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req.WithContext(spanishContext(req.Context())))
	return recorder.Code, recorder.Body.String()
}

func TestListFragmentAnswersTheListOnlyToHtmx(t *testing.T) {
	list := &stubList{list: sampleList()}
	r := newRouter(list, &stubDelete{})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/w1/branches?q=feat", nil)
	req.Header.Set("HX-Request", "true")
	code, body := serve(t, r, req)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"feat"}, list.filters, "the filter reaches the use case")
	assert.Contains(t, body, `id="branch-delete-form"`)
	assert.NotContains(t, body, `id="workspace-branches"`, "the htmx answer is the list, not the whole tab")
}

func TestListWithoutHtmxAnswersTheWholeTab(t *testing.T) {
	r := newRouter(&stubList{list: sampleList()}, &stubDelete{})

	_, body := serve(t, r, httptest.NewRequest(http.MethodGet, "/workspaces/w1/branches", nil))

	assert.Contains(t, body, `id="workspace-branches"`)
	assert.Contains(t, body, `id="branch-list"`)
}

func TestDeleteSendsEveryCheckedBranchAndReportsEachOutcome(t *testing.T) {
	del := &stubDelete{results: []application.DeleteResult{
		{Ref: models.Ref{Kind: models.KindLocal, Name: "feature/old"}, Outcome: application.OutcomeDeleted},
		{Ref: models.Ref{Kind: models.KindRemote, Name: "origin/gone"}, Outcome: application.OutcomeFailed, Err: errors.New("remote rejected")},
		{Ref: models.Ref{Kind: models.KindLocal, Name: "main"}, Outcome: application.OutcomeRefused, Reason: models.ReasonCheckedOut},
	}}
	list := &stubList{list: sampleList()}
	r := newRouter(list, del)

	form := url.Values{"branch": {"local:feature/old", "remote:origin/gone", "local:feature/old"}, "q": {"feat"}}
	req := httptest.NewRequest(http.MethodPost, "/workspaces/w1/branches/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	code, body := serve(t, r, req)

	assert.Equal(t, http.StatusOK, code)
	require.Len(t, del.refs, 1)
	assert.Equal(t, []models.Ref{
		{Kind: models.KindLocal, Name: "feature/old"},
		{Kind: models.KindRemote, Name: "origin/gone"},
	}, del.refs[0], "duplicates are sent once")
	assert.Equal(t, []string{"feat"}, list.filters, "the refreshed list keeps the filter")
	assert.Contains(t, body, "Eliminada: feature/old")
	assert.Contains(t, body, "No se pudo eliminar origin/gone: remote rejected")
	assert.Contains(t, body, "No se eliminó main: Rama actual")
}

func TestPerRowDeleteReadsTheReferenceFromTheQuery(t *testing.T) {
	del := &stubDelete{}
	r := newRouter(&stubList{list: sampleList()}, del)

	req := httptest.NewRequest(http.MethodPost, "/workspaces/w1/branches/delete?branch=remote%3Aorigin%2Fgone&q=", nil)
	req.Header.Set("HX-Request", "true")
	serve(t, r, req)

	require.Len(t, del.refs, 1)
	assert.Equal(t, []models.Ref{{Kind: models.KindRemote, Name: "origin/gone"}}, del.refs[0])
}

func TestDeleteWithNothingCheckedDeletesNothingAndSaysSo(t *testing.T) {
	del := &stubDelete{}
	r := newRouter(&stubList{list: sampleList()}, del)

	req := httptest.NewRequest(http.MethodPost, "/workspaces/w1/branches/delete", strings.NewReader("q="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	_, body := serve(t, r, req)

	assert.Empty(t, del.refs)
	assert.Contains(t, body, "Selecciona al menos una rama.")
}

func TestMalformedBranchValueIsReportedNotDeleted(t *testing.T) {
	del := &stubDelete{results: []application.DeleteResult{
		{Ref: models.ParseRef("-D"), Outcome: application.OutcomeFailed, Err: werrors.ErrInvalidBranch},
	}}
	r := newRouter(&stubList{list: sampleList()}, del)

	form := url.Values{"branch": {"-D"}}
	req := httptest.NewRequest(http.MethodPost, "/workspaces/w1/branches/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	_, body := serve(t, r, req)

	require.Len(t, del.refs, 1)
	assert.Equal(t, models.Kind(""), del.refs[0][0].Kind)
	assert.Contains(t, body, "Nombre de rama no válido: -D")
}

func TestListFailureIsShownAsTheMessageOfTheFragment(t *testing.T) {
	r := newRouter(&stubList{err: errors.New("workspace not found")}, &stubDelete{})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/w1/branches", nil)
	req.Header.Set("HX-Request", "true")
	code, body := serve(t, r, req)

	assert.Equal(t, http.StatusOK, code, "htmx swaps only 2xx responses, so the message is the fragment")
	assert.Contains(t, body, "workspace not found")
}

func TestListTabRendersTheScreenOfTheWorkspace(t *testing.T) {
	var list application.BranchList
	list.Workspace.Name = "alpha"
	list.Workspace.BaseBranch = "develop"
	handlers := NewHandlers(&stubList{list: list}, &stubDelete{})
	ctx := spanishContext(context.Background())

	component := handlers.ListTab(ctx, map[string]string{"id": "w1"})
	var buffer strings.Builder
	require.NoError(t, component.Render(ctx, &buffer))

	assert.Contains(t, buffer.String(), `id="workspace-branches"`)
	assert.Contains(t, buffer.String(), "alpha")
}
