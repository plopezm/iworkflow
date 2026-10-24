package code_hosting_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/code_hosting/application"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/infrastructure/di"
)

const (
	// projectPath is the GitLab project of the fixture repository, as the API expects it encoded.
	projectPath = "/api/v4/projects/grupo%2Fproyecto"
	// findOpenPath is the list the panel reads for the task branch.
	findOpenPath = "GET " + projectPath + "/merge_requests?source_branch=pi%2Ffeature&order_by=created_at&sort=desc"
	taskID       = "task-1"
)

// stubTasks answers every lookup with one task source.
type stubTasks struct {
	source *models.TaskSource
}

func (s stubTasks) Find(_ context.Context, _ string) (*models.TaskSource, error) {
	return s.source, nil
}

// fixture is a module with a GitLab fake behind it and a task whose origin points at that fake.
type fixture struct {
	router chi.Router
	module *di.Module
	gitlab *fakeProvider
}

// newFixture builds the module over a migrated database. The task branch is one commit ahead of main
// and its origin is the GitLab fake, so the panel reads and acts on the scripted answers.
func newFixture(t *testing.T, routes ...scriptedRoute) fixture {
	t.Helper()
	gitlab := newFakeProvider(t, routes...)
	repo := gitRepo(t)
	runGit(t, repo, "remote", "add", "origin", gitlab.URL()+"/grupo/proyecto.git")
	runGit(t, repo, "checkout", "-q", "-b", "pi/feature")
	runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "work")
	module := di.NewModule(di.Deps{
		DB: dbtest.New(t),
		Tasks: stubTasks{source: &models.TaskSource{
			RepoPath:    repo,
			Branch:      "pi/feature",
			BaseBranch:  "main",
			Title:       "Mi tarea",
			Description: "Prompt de la tarea",
		}},
	})
	router := chi.NewRouter()
	module.Routes(router)
	return fixture{router: router, module: module, gitlab: gitlab}
}

// saveGitLab stores the GitLab settings through the use case, as a previous save would have.
func (f fixture) saveGitLab(t *testing.T, token string) {
	t.Helper()
	_, err := f.module.SaveSettings.Execute(context.Background(), application.SaveCodeHostSettingsInput{
		Provider: models.GitLab,
		BaseURL:  f.gitlab.URL(),
		Token:    token,
	})
	require.NoError(t, err)
}

// send performs a request through the router and returns the recorded answer.
func send(t *testing.T, router http.Handler, method string, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request := httptest.NewRequest(method, target, body)
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestSaveSettingsReturnsTheCardWithoutTheToken(t *testing.T) {
	f := newFixture(t)

	recorder := send(t, f.router, http.MethodPost, "/code-host/settings/gitlab", url.Values{
		"base_url": {f.gitlab.URL()},
		"token":    {"glpat-secret"},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, `id="code-host-settings-gitlab"`)
	require.Contains(t, body, `value="`+f.gitlab.URL()+`"`)
	require.Contains(t, body, "(leave empty to keep it)")
	require.NotContains(t, body, "glpat-secret")
}

func TestSaveSettingsKeepsTheTypedURLAndShowsWhyItWasRejected(t *testing.T) {
	f := newFixture(t)

	recorder := send(t, f.router, http.MethodPost, "/code-host/settings/gitlab", url.Values{
		"base_url": {"git.typo"},
		"token":    {"glpat-secret"},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "URL inválida: git.typo")
	require.Contains(t, recorder.Body.String(), `value="git.typo"`)
	require.NotContains(t, recorder.Body.String(), "glpat-secret")
}

func TestSaveSettingsRejectsAnUnknownProviderInsideTheFragment(t *testing.T) {
	f := newFixture(t)

	recorder := send(t, f.router, http.MethodPost, "/code-host/settings/bitbucket", url.Values{"base_url": {"https://x"}})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "proveedor de repositorios desconocido: bitbucket")
}

func TestTestConnectionShowsTheAuthenticatedUser(t *testing.T) {
	f := newFixture(t, scriptedRoute{target: "GET /api/v4/user", status: 200, body: `{"username":"e2e-user"}`})
	f.saveGitLab(t, "glpat-secret")

	recorder := send(t, f.router, http.MethodPost, "/code-host/settings/gitlab/test", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Connected as e2e-user")
}

func TestTestConnectionShowsTheRefusalWithoutTheToken(t *testing.T) {
	f := newFixture(t, scriptedRoute{target: "GET /api/v4/user", status: 401, body: `{"message":"401 Unauthorized"}`})
	f.saveGitLab(t, "glpat-secret")

	recorder := send(t, f.router, http.MethodPost, "/code-host/settings/gitlab/test", nil)

	require.Contains(t, recorder.Body.String(), "401 Unauthorized")
	require.NotContains(t, recorder.Body.String(), "glpat-secret")
}

func TestPanelOffersToOpenARequestWhenTheBranchHasNone(t *testing.T) {
	f := newFixture(t,
		scriptedRoute{target: findOpenPath, status: 200, body: "[]"},
		scriptedRoute{target: "GET " + projectPath + "/repository/branches/main", status: 200, body: `{"name":"main"}`},
		scriptedRoute{target: "GET " + projectPath + "/repository/branches/pi%2Ffeature", status: 200, body: `{"name":"pi/feature"}`},
	)
	f.saveGitLab(t, "glpat-secret")

	recorder := send(t, f.router, http.MethodGet, "/code-host/tasks/"+taskID+"/merge-request", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "This branch has no merge/pull request yet.")
	require.Contains(t, recorder.Body.String(), "Create merge/pull request")
}

func TestPanelShowsAnOpenRequestWithItsActions(t *testing.T) {
	f := newFixture(t,
		scriptedRoute{target: findOpenPath, status: 200, body: "[" + gitLabMR + "]"},
		scriptedRoute{target: "GET " + projectPath + "/merge_requests/7/pipelines", status: 200, body: "[]"},
	)
	f.saveGitLab(t, "glpat-secret")

	recorder := send(t, f.router, http.MethodGet, "/code-host/tasks/"+taskID+"/merge-request", nil)

	body := recorder.Body.String()
	require.Contains(t, body, "!7 T")
	require.Contains(t, body, "Merge MR")
	require.Contains(t, body, "Push branch")
	require.Contains(t, body, "No CI pipeline.")
}

// TestMergeAsksForConfirmationAndStopsWhenThePushFails checks the order of a merge: the confirmation
// shows nothing, and the branch is pushed before the provider is asked to merge. The fake GitLab does
// not speak git, so the push fails and the merge must not happen (as in the Rust version).
func TestMergeAsksForConfirmationAndStopsWhenThePushFails(t *testing.T) {
	f := newFixture(t,
		scriptedRoute{target: findOpenPath, status: 200, body: "[" + gitLabMR + "]"},
		scriptedRoute{target: "GET " + projectPath + "/merge_requests/7/pipelines", status: 200, body: "[]"},
		scriptedRoute{target: "PUT " + projectPath + "/merge_requests/7/merge", status: 200, body: gitLabMR},
	)
	f.saveGitLab(t, "glpat-secret")

	confirm := send(t, f.router, http.MethodGet, "/code-host/tasks/"+taskID+"/merge-request?confirm=1", nil)
	require.Contains(t, confirm.Body.String(), `hx-post="/code-host/tasks/task-1/merge"`)
	require.Empty(t, mergeCalls(f.gitlab), "showing the confirmation must not merge")

	merged := send(t, f.router, http.MethodPost, "/code-host/tasks/"+taskID+"/merge", nil)
	require.Equal(t, http.StatusOK, merged.Code)
	require.Contains(t, merged.Body.String(), "no se pudo publicar la rama pi/feature en origin")
	require.Empty(t, mergeCalls(f.gitlab), "a failed push must prevent the merge")
}

// mergeCalls returns the merge requests the fake received.
func mergeCalls(provider *fakeProvider) []recordedRequest {
	var calls []recordedRequest
	for _, seen := range provider.requests() {
		if seen.method == http.MethodPut && strings.HasSuffix(seen.target, "/merge") {
			calls = append(calls, seen)
		}
	}
	return calls
}

func TestCreateKeepsTheTypedTitleWhenThePushFails(t *testing.T) {
	f := newFixture(t,
		scriptedRoute{target: findOpenPath, status: 200, body: "[]"},
		scriptedRoute{target: "GET " + projectPath + "/repository/branches/main", status: 200, body: `{"name":"main"}`},
		scriptedRoute{target: "GET " + projectPath + "/repository/branches/pi%2Ffeature", status: 404, body: `{"message":"404 Branch Not Found"}`},
	)
	f.saveGitLab(t, "glpat-secret")

	recorder := send(t, f.router, http.MethodPost, "/code-host/tasks/"+taskID+"/merge-request", url.Values{
		"title": {"Mi título"},
	})

	body := recorder.Body.String()
	require.Contains(t, body, "no se pudo publicar la rama pi/feature en origin")
	require.Contains(t, body, `value="Mi título"`)
	require.NotContains(t, body, "glpat-secret")
	for _, seen := range f.gitlab.requests() {
		require.NotEqual(t, http.MethodPost, seen.method, "no request must be opened when the push fails")
	}
}

func TestPushFailureIsShownWithoutTheToken(t *testing.T) {
	f := newFixture(t,
		scriptedRoute{target: findOpenPath, status: 200, body: "[" + gitLabMR + "]"},
		scriptedRoute{target: "GET " + projectPath + "/merge_requests/7/pipelines", status: 200, body: "[]"},
	)
	f.saveGitLab(t, "glpat-secret")

	recorder := send(t, f.router, http.MethodPost, "/code-host/tasks/"+taskID+"/push", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "Branch pushed to origin.")
	require.NotContains(t, recorder.Body.String(), "glpat-secret")
}

func TestPanelWithoutAConfiguredProviderShowsOnlyTheError(t *testing.T) {
	f := newFixture(t)

	recorder := send(t, f.router, http.MethodGet, "/code-host/tasks/"+taskID+"/merge-request", nil)

	body := recorder.Body.String()
	require.Contains(t, body, "no coincide con ningún GitLab ni GitHub configurado")
	require.NotContains(t, body, "Create merge/pull request")
}

func TestModuleEmbedsItsScreensWithoutData(t *testing.T) {
	f := newFixture(t)

	var panel strings.Builder
	require.NoError(t, f.module.TaskPanel(taskID).Render(context.Background(), &panel))
	require.Contains(t, panel.String(), `hx-get="/code-host/tasks/task-1/merge-request"`)

	var card bytes.Buffer
	require.NoError(t, f.module.SettingsContent(models.GitLab)(context.Background()).Render(context.Background(), &card))
	require.Contains(t, card.String(), `id="code-host-settings-gitlab"`)
}
