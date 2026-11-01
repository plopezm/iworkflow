package open_with_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	openwithdi "iworkflow/internal/modules/open_with/infrastructure/di"
)

type worktrees struct{ dir string }

func (w worktrees) Path(_ context.Context, taskID string) (string, error) {
	if taskID == "gone" {
		return "", openerrors.ErrWorktreeNotFound
	}
	return w.dir, nil
}

func serve(t *testing.T) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	router := chi.NewRouter()
	openwithdi.NewModule(openwithdi.Deps{DB: dbtest.New(t), Worktrees: worktrees{dir: dir}}).Routes(router)
	return router, dir
}

func call(router http.Handler, method string, target string, form url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestDialogWithoutApplicationsInvitesToAddOne(t *testing.T) {
	router, _ := serve(t)

	response := call(router, http.MethodGet, "/open-with/tasks/t1?scope=s", nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `id="task-open-with-s"`)
	require.Contains(t, response.Body.String(), `name="path"`)
	require.NotContains(t, response.Body.String(), "data-open-with-list")
}

func TestAddedApplicationAppearsAndInvalidOneShowsTheErrorKeepingTheInput(t *testing.T) {
	router, _ := serve(t)

	bad := call(router, http.MethodPost, "/open-with/tasks/t1", url.Values{"name": {"Editor"}, "path": {" "}})
	require.Contains(t, bad.Body.String(), "the path is required")
	require.Contains(t, bad.Body.String(), `value="Editor"`)

	good := call(router, http.MethodPost, "/open-with/tasks/t1", url.Values{"name": {"Editor"}, "path": {"/bin/true"}})
	require.Contains(t, good.Body.String(), "data-open-with-list")
	require.Contains(t, good.Body.String(), "/bin/true")
}

func TestLaunchRunsTheApplicationInTheWorktreeAndReportsFailures(t *testing.T) {
	router, dir := serve(t)
	call(router, http.MethodPost, "/open-with/tasks/t1", url.Values{"name": {"Touch"}, "path": {"/bin/sh"}, "args": {"-c\ntouch started"}})
	id := firstLauncherID(t, call(router, http.MethodGet, "/open-with/tasks/t1", nil).Body.String())

	ok := call(router, http.MethodPost, "/open-with/tasks/t1/launchers/"+id+"/launch", nil)
	require.Contains(t, ok.Body.String(), "badge-ok")
	require.Eventually(t, func() bool { return fileExists(dir + "/started") }, 2e9, 20e6)

	noTree := call(router, http.MethodPost, "/open-with/tasks/gone/launchers/"+id+"/launch", nil)
	require.Contains(t, noTree.Body.String(), "the task has no worktree")
}

func TestDeleteRemovesTheApplication(t *testing.T) {
	router, _ := serve(t)
	call(router, http.MethodPost, "/open-with/tasks/t1", url.Values{"name": {"Editor"}, "path": {"/bin/true"}})
	id := firstLauncherID(t, call(router, http.MethodGet, "/open-with/tasks/t1", nil).Body.String())

	response := call(router, http.MethodPost, "/open-with/tasks/t1/launchers/"+id+"/delete", nil)

	require.NotContains(t, response.Body.String(), "data-open-with-list")
}

func TestBrowseOpensThePickerAtTheTypedFileFolderAndKeepsTheForm(t *testing.T) {
	router, dir := serve(t)
	require.NoError(t, os.WriteFile(dir+"/myapp", nil, 0o600))
	require.NoError(t, os.Mkdir(dir+"/sub", 0o750))
	query := url.Values{"name": {"Mine"}, "path": {dir + "/myapp"}, "args": {"--x"}}

	response := call(router, http.MethodGet, "/open-with/tasks/t1/browse?"+query.Encode(), nil)

	body := response.Body.String()
	require.Contains(t, body, "data-open-with-picker")
	require.Contains(t, body, `<code class="text-[11px] text-muted break-all">`+dir+`</code>`, "the picker shows the folder of the typed file")
	require.Contains(t, body, `{&#34;file&#34;:&#34;`+dir+`/myapp&#34;}`, "the file of that folder is listed")
	require.Contains(t, body, `{&#34;dir&#34;:&#34;`+dir+`/sub&#34;,&#34;filter&#34;:&#34;&#34;}`, "its folders are listed")
	require.Contains(t, body, `value="Mine"`, "what the user typed survives")
	require.Contains(t, body, "--x")
}

func TestBrowseFiltersAndReportsABadFolderInsideThePicker(t *testing.T) {
	router, dir := serve(t)
	require.NoError(t, os.WriteFile(dir+"/alpha", nil, 0o600))
	require.NoError(t, os.WriteFile(dir+"/beta", nil, 0o600))

	filtered := call(router, http.MethodGet, "/open-with/tasks/t1/browse?"+url.Values{"dir": {dir}, "filter": {"alp"}}.Encode(), nil)
	require.Contains(t, filtered.Body.String(), "alpha")
	require.NotContains(t, filtered.Body.String(), "beta")

	bad := call(router, http.MethodGet, "/open-with/tasks/t1/browse?"+url.Values{"dir": {dir + "/missing"}}.Encode(), nil)
	require.Contains(t, bad.Body.String(), "could not read the folder")
}

func TestBrowseFallsBackToHomeWhenTheTypedPathLeadsNowhere(t *testing.T) {
	router, _ := serve(t)

	response := call(router, http.MethodGet, "/open-with/tasks/t1/browse?"+url.Values{"path": {"/no/such/place/app"}}.Encode(), nil)

	require.Contains(t, response.Body.String(), "data-open-with-picker")
	require.NotContains(t, response.Body.String(), "could not read the folder")
}

func TestChooseFillsThePathAndClosesThePickerOrCancelsKeepingTheText(t *testing.T) {
	router, _ := serve(t)

	chosen := call(router, http.MethodGet, "/open-with/tasks/t1/choose?"+url.Values{"name": {"E"}, "path": {"typed"}, "file": {"/usr/bin/code"}}.Encode(), nil)
	require.Contains(t, chosen.Body.String(), `value="/usr/bin/code"`)
	require.NotContains(t, chosen.Body.String(), "data-open-with-picker")

	cancelled := call(router, http.MethodGet, "/open-with/tasks/t1/choose?"+url.Values{"path": {"typed"}}.Encode(), nil)
	require.Contains(t, cancelled.Body.String(), `value="typed"`)
}
