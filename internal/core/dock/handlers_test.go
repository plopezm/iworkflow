package dock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func testRouter(t *testing.T) chi.Router {
	t.Helper()
	registry, err := NewRegistry(
		Route{Name: "board", Pattern: "/", Icon: "▦", Title: func(context.Context, Params) string { return "Board" },
			Render: func(context.Context, Params) templ.Component { return templ.Raw("<p>board page</p>") }},
		Route{Name: "jira", Pattern: "/jira", Icon: "◈", Title: func(context.Context, Params) string { return "Jira" },
			Render: func(context.Context, Params) templ.Component { return templ.Raw("<p>jira page</p>") }},
	)
	require.NoError(t, err)
	router := chi.NewRouter()
	NewHandlers(registry, nil, nil).Routes(router)
	return router
}

func post(t *testing.T, router http.Handler, target string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRestoreRendersTheSavedLayoutAndTheClientCopy(t *testing.T) {
	saved := New()
	saved.Open("/jira", "Jira", 0)
	savedJSON, err := saved.ToJSON()
	require.NoError(t, err)

	rec := post(t, testRouter(t), "/dock/layout", url.Values{"layout": {savedJSON}})

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `id="dock"`)
	require.Contains(t, body, `data-dock-tab-button`)
	require.Contains(t, body, "Jira")
	require.Contains(t, body, `data-path="/jira"`)
	require.Contains(t, body, "data-layout=", "the client stores this copy")
}

func TestRestoreOpensTheBootPathExceptTheRoot(t *testing.T) {
	router := testRouter(t)

	rec := post(t, router, "/dock/layout", url.Values{"boot": {"/jira"}})
	require.Contains(t, rec.Body.String(), `data-path="/jira"`)

	rec = post(t, router, "/dock/layout", url.Values{"boot": {"/"}})
	require.NotContains(t, rec.Body.String(), "data-dock-tab-button", "the root opens nothing")
}

func TestRestoreStartsFromAnEmptyLayoutWhenTheSavedOneIsBroken(t *testing.T) {
	rec := post(t, testRouter(t), "/dock/layout", url.Values{"layout": {"not json"}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `data-dock-group="1"`)
}

func TestActionAppliesTheOperationAndReturnsTheNewLayout(t *testing.T) {
	rec := post(t, testRouter(t), "/dock/action/open", url.Values{"path": {"/jira"}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `data-path="/jira"`)
	require.Contains(t, rec.Body.String(), `"Jira"`, "the title is the route default")
}

func TestActionRejectsUnknownOperationsAndBadNumbers(t *testing.T) {
	router := testRouter(t)
	require.Equal(t, http.StatusBadRequest, post(t, router, "/dock/action/explode", url.Values{}).Code)
	require.Equal(t, http.StatusBadRequest, post(t, router, "/dock/action/close_tab", url.Values{"tab": {"x"}}).Code)
}

func TestPaneRendersThePageOfTheTab(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dock/pane?path=%2Fjira", nil)
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "<p>jira page</p>", rec.Body.String())
}

func TestPaneOfAnUnknownPathRendersNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dock/pane?path=%2Fnope", nil)
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, req)
	require.Contains(t, rec.Body.String(), "Page not found.")
}
