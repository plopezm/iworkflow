package app_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitRepoWithCommit makes a repository with one commit on main in a fresh temporary folder, so it can be registered.
func gitRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	return dir
}

// postForm posts an urlencoded form to the composed router.
func postForm(t *testing.T, handler http.Handler, target string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request)
	return rec
}

// registerWorkspace registers a repository through the composed router and returns the id of the new workspace,
// read from the table's edit link.
func registerWorkspace(t *testing.T, handler http.Handler, name string, path string, groupID string) string {
	t.Helper()
	values := url.Values{"name": {name}, "path": {path}}
	if groupID != "" {
		values.Set("group_id", groupID)
	}
	rec := postForm(t, handler, "/workspaces/register", values)
	require.Equal(t, http.StatusOK, rec.Code)
	match := regexp.MustCompile(`/workspaces/([0-9a-f-]{36})/edit`).FindStringSubmatch(get(t, handler, "/workspaces/table").Body.String())
	require.Len(t, match, 2, "the registered workspace is in the table")
	return match[1]
}

// dockPane opens the create tab at path the way the dock does: the tab address is query-escaped.
func dockPane(t *testing.T, handler http.Handler, tabPath string) string {
	t.Helper()
	return get(t, handler, "/dock/pane?path="+url.QueryEscape(tabPath)).Body.String()
}

func TestCreateTabOpenedFromTheDashboardKeepsTheOriginAsReturn(t *testing.T) {
	handler := newRouter(t)

	body := dockPane(t, handler, "/board/new/from/%2F")

	assert.Contains(t, body, `<input type="hidden" name="return_to" value="/">`)
	assert.Contains(t, body, `data-dock-path="/"`, "Cancel goes back to the origin")
}

func TestCreateTabOpenedFromABoardKeepsTheOriginAndTheWorkspace(t *testing.T) {
	handler := newRouter(t)
	workspaceID := "8f1d1c2e-9a4b-4f3e-8c2d-1a2b3c4d5e6f"

	body := dockPane(t, handler, "/board/"+workspaceID+"/new/from/%2Fboard%2F"+workspaceID)

	assert.Contains(t, body, `<input type="hidden" name="return_to" value="/board/`+workspaceID+`">`)
	assert.Contains(t, body, `data-dock-path="/board/`+workspaceID+`"`, "Cancel goes back to the origin board")
}

func TestCreateTabWithAnUnsafeOriginRendersNoReturn(t *testing.T) {
	handler := newRouter(t)

	body := dockPane(t, handler, "/board/new/from/%2F%2Fevil.com")

	assert.NotContains(t, body, `name="return_to"`)
	assert.NotContains(t, body, "evil.com")
}

func TestCreatedTaskReturnsToItsOriginOrToTheBoard(t *testing.T) {
	cases := []struct {
		name     string
		returnTo string
		want     func(workspaceID string) string
	}{
		{name: "dashboard origin", returnTo: "/", want: func(string) string { return "/" }},
		{name: "no origin falls back to the board", returnTo: "", want: func(id string) string { return "/board/" + id }},
		{name: "unsafe origin falls back to the board", returnTo: "//evil.com", want: func(id string) string { return "/board/" + id }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newRouter(t)
			workspaceID := registerWorkspace(t, handler, "demo", gitRepoWithCommit(t), "")
			values := url.Values{"workspace_id": {workspaceID}, "title": {"demo task"}, "prompt": {"do the thing"}}
			if tc.returnTo != "" {
				values.Set("return_to", tc.returnTo)
			}

			rec := postForm(t, handler, "/board/tasks", values)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), `data-dock-leave="`+tc.want(workspaceID)+`"`)
		})
	}
}

func TestRegisteringWithAGroupPutsTheWorkspaceInIt(t *testing.T) {
	handler := newRouter(t)
	created := postForm(t, handler, "/workspace-groups", url.Values{"name": {"Backend"}})
	require.Equal(t, http.StatusNoContent, created.Code)
	groupID := regexp.MustCompile(`data-group-id="([^"]+)"`).FindStringSubmatch(get(t, handler, "/workspace-groups/list").Body.String())
	require.Len(t, groupID, 2, "the created group is listed")
	registerWorkspace(t, handler, "grupo-demo", gitRepoWithCommit(t), groupID[1])

	members := get(t, handler, "/workspace-groups/"+groupID[1]+"/members").Body.String()

	assert.Contains(t, members, "grupo-demo", "the new workspace is a member of the chosen group")
}
