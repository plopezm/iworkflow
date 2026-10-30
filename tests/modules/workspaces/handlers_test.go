package workspaces_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/infrastructure/di"
)

// httpFixture is a workspaces module mounted on a chi router, with the UI in Spanish (the language the
// e2e scripts run in). The home folder is a temporary folder the browser may list and create into.
type httpFixture struct {
	t      *testing.T
	module *di.Module
	router http.Handler
	home   string
}

func newHTTPFixture(t *testing.T) *httpFixture {
	t.Helper()
	home := t.TempDir()
	module := di.NewModule(di.Deps{
		DB:          dbtest.New(t),
		HomeDir:     home,
		Tasks:       noTasks{},
		AgentsFiles: noAgentsFiles{},
	})
	mux := chi.NewRouter()
	module.Routes(mux)
	return &httpFixture{t: t, module: module, router: withSpanish(mux), home: home}
}

// withSpanish stores the Spanish locale in each request context, as the locale middleware does.
func withSpanish(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := i18n.State{Locale: i18n.Es, Chosen: true}
		next.ServeHTTP(w, r.WithContext(i18n.WithState(r.Context(), state)))
	})
}

func (f *httpFixture) post(target string, form url.Values) *httptest.ResponseRecorder {
	f.t.Helper()
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	return recorder
}

func (f *httpFixture) get(target string) *httptest.ResponseRecorder {
	f.t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("HX-Request", "true")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	return recorder
}

// register registers the repository at root and returns the response body of the registration.
func (f *httpFixture) register(name string, root string) string {
	f.t.Helper()
	response := f.post("/workspaces/register", url.Values{"name": {name}, "path": {root}, "base_branch": {"main"}})
	require.Equal(f.t, http.StatusOK, response.Code)
	return response.Body.String()
}

// registeredID returns the identifier of the single workspace the fixture has registered.
func (f *httpFixture) registeredID() string {
	f.t.Helper()
	workspaces, err := f.module.List.Execute(context.Background())
	require.NoError(f.t, err)
	require.Len(f.t, workspaces, 1)
	return workspaces[0].ID.String()
}

func TestRegisterFragmentReplacesTheFormAndRefreshesTheTable(t *testing.T) {
	fixture := newHTTPFixture(t)
	repo := newGitRepo(t)

	body := fixture.register("Demo", repo)

	assert.Contains(t, body, `id="workspace-register-slot"`, "the form slot is replaced with a fresh one")
	assert.Contains(t, body, `id="workspace-table"`)
	assert.Contains(t, body, `hx-swap-oob="true"`, "the table is refreshed out of band")
	assert.Contains(t, body, "Demo")
	assert.NotContains(t, body, "workspace-register-prompt", "a successful registration asks nothing")
}

func TestRegisterOfMissingFolderAsksForConfirmationAndResubmitsWithInitialize(t *testing.T) {
	fixture := newHTTPFixture(t)
	missing := filepath.Join(t.TempDir(), "not-yet")

	response := fixture.post("/workspaces/register", url.Values{"name": {"Nuevo"}, "path": {missing}, "base_branch": {"main"}})

	body := response.Body.String()
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, body, "no existe. ¿Crearla e inicializar un repositorio git?", "the Spanish prompt is shown")
	assert.Contains(t, body, "Crear e inicializar", "the confirm label matches the reason")
	assert.Contains(t, body, `hx-vals="{&#34;agents_file_id&#34;:&#34;&#34;,&#34;base_branch&#34;:&#34;main&#34;,&#34;initialize&#34;:&#34;true&#34;,&#34;name&#34;:&#34;Nuevo&#34;,&#34;path&#34;:&#34;`+missing+`&#34;}"`,
		"confirming sends the same registration with initialize=true")
	_, statErr := os.Stat(missing)
	assert.True(t, os.IsNotExist(statErr), "nothing is created before the user confirms")

	confirmed := fixture.post("/workspaces/register", url.Values{
		"name": {"Nuevo"}, "path": {missing}, "base_branch": {"main"}, "initialize": {"true"},
	})
	assert.Contains(t, confirmed.Body.String(), `hx-swap-oob="true"`)
	assert.DirExists(t, filepath.Join(missing, ".git"))
}

func TestRegisterDomainErrorIsShownAsTheFragmentMessage(t *testing.T) {
	fixture := newHTTPFixture(t)
	repo := newGitRepo(t)
	fixture.register("Demo", repo)

	response := fixture.post("/workspaces/register", url.Values{"name": {"Otro"}, "path": {repo}, "base_branch": {"main"}})

	assert.Equal(t, http.StatusOK, response.Code, "domain errors are shown as fragments with status 200")
	assert.Contains(t, response.Body.String(), "ya existe un workspace para la ruta")
	assert.Contains(t, response.Body.String(), `id="workspace-register-slot"`, "the form keeps its slot so the user can fix it")
	assert.Contains(t, response.Body.String(), `value="Otro"`, "the submitted name is echoed back")
}

func TestRegisterRejectsBlankNameWithoutSaving(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.post("/workspaces/register", url.Values{"name": {"  "}, "path": {newGitRepo(t)}})

	assert.Contains(t, response.Body.String(), "el nombre es obligatorio")
	workspaces, err := fixture.module.List.Execute(context.Background())
	require.NoError(t, err)
	assert.Empty(t, workspaces)
}

func TestDismissPromptAnswersAnEmptyPromptSlot(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.get("/workspaces/register/dismiss")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, `<div id="workspace-register-prompt"></div>`, response.Body.String())
}

func TestUpdateSavesAndRedirectsTheTabToTheList(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.register("Demo", newGitRepo(t))
	id := fixture.registeredID()

	response := fixture.post("/workspaces/"+id+"/update", url.Values{"name": {"Renombrado"}, "base_branch": {"main"}})

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, `<span data-dock-redirect="/workspaces"></span>`, response.Body.String())
	workspaces, err := fixture.module.List.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Renombrado", workspaces[0].Name)
}

func TestUpdateErrorsAreShownWithoutRedirecting(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.register("Demo", newGitRepo(t))
	id := fixture.registeredID()

	invalidName := fixture.post("/workspaces/"+id+"/update", url.Values{"name": {""}, "base_branch": {"main"}})
	unknownBranch := fixture.post("/workspaces/"+id+"/update", url.Values{"name": {"Demo"}, "base_branch": {"nope"}})
	badID := fixture.post("/workspaces/not-a-uuid/update", url.Values{"name": {"Demo"}})

	assert.Contains(t, invalidName.Body.String(), "el nombre es obligatorio")
	assert.Contains(t, unknownBranch.Body.String(), "la rama nope no existe en el repositorio")
	assert.Contains(t, badID.Body.String(), "identificador de workspace inválido: not-a-uuid")
	for _, response := range []*httptest.ResponseRecorder{invalidName, unknownBranch, badID} {
		assert.NotContains(t, response.Body.String(), "data-dock-redirect")
	}
}

func TestDeleteAnswersTheRemainingTable(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.register("Demo", newGitRepo(t))
	id := fixture.registeredID()

	response := fixture.post("/workspaces/"+id+"/delete", nil)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `id="workspace-table"`)
	assert.Contains(t, response.Body.String(), "Aún no hay workspaces registrados.")
	assert.NotContains(t, response.Body.String(), "article", "no card remains")
}

func TestDeleteOfUnknownWorkspaceShowsTheErrorInTheTable(t *testing.T) {
	fixture := newHTTPFixture(t)
	unknown, err := models.NewWorkspaceID()
	require.NoError(t, err)

	response := fixture.post("/workspaces/"+unknown.String()+"/delete", nil)

	assert.Contains(t, response.Body.String(), "workspace no encontrado: "+unknown.String())
}

func TestOpenPickerListsFoldersAndMarksGitRepositories(t *testing.T) {
	fixture := newHTTPFixture(t)
	require.NoError(t, os.Mkdir(filepath.Join(fixture.home, "alpha"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(fixture.home, "beta", ".git"), 0o755))

	response := fixture.get("/workspaces/directories?path=" + url.QueryEscape(fixture.home))

	body := response.Body.String()
	assert.Contains(t, body, `id="modal-panel"`, "the picker opens")
	assert.Contains(t, body, "alpha")
	assert.Contains(t, body, "beta")
	assert.Contains(t, body, ">git</span>", "the git badge is shown for repositories")
}

func TestOpenPickerReportsRelativePathsInTheDialog(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.get("/workspaces/directories?path=relative/dir")

	assert.Contains(t, response.Body.String(), "la ruta debe ser absoluta: relative/dir")
}

func TestPickerEntriesAreFilteredByNameWithoutRedrawingTheDialog(t *testing.T) {
	fixture := newHTTPFixture(t)
	require.NoError(t, os.Mkdir(filepath.Join(fixture.home, "Alpha"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(fixture.home, "beta"), 0o755))

	response := fixture.get("/workspaces/directories/entries?path=" + url.QueryEscape(fixture.home) + "&filter=ALP")

	body := response.Body.String()
	assert.Contains(t, body, `id="picker-entries"`)
	assert.Contains(t, body, "Alpha")
	assert.NotContains(t, body, "beta")
	assert.NotContains(t, body, "modal-panel", "only the list is replaced")
}

func TestPickerEntriesWithoutMatchesSayThereAreNoSubfolders(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.get("/workspaces/directories/entries?path=" + url.QueryEscape(fixture.home) + "&filter=zzz")

	assert.Contains(t, response.Body.String(), "Sin subcarpetas")
}

func TestSelectDirectoryClosesThePickerAndSetsTheFormPath(t *testing.T) {
	fixture := newHTTPFixture(t)
	chosen := filepath.Join(fixture.home, "proyecto")

	response := fixture.get("/workspaces/directories/select?path=" + url.QueryEscape(chosen))

	body := response.Body.String()
	assert.Contains(t, body, `<div id="workspace-picker"></div>`, "the picker is closed")
	assert.Contains(t, body, `id="workspace-path"`)
	assert.Contains(t, body, `hx-swap-oob="true"`)
	assert.Contains(t, body, `value="`+chosen+`"`)
}

func TestCreateDirectoryNavigatesToTheNewFolder(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.post("/workspaces/directories/create", url.Values{"parent": {fixture.home}, "name": {"nueva"}})

	created := filepath.Join(fixture.home, "nueva")
	assert.DirExists(t, created)
	assert.Contains(t, response.Body.String(), "modal-panel")
	assert.Contains(t, response.Body.String(), `value="`+created+`"`)
}

func TestCreateDirectoryErrorStaysOnTheParentWithTheMessage(t *testing.T) {
	fixture := newHTTPFixture(t)
	require.NoError(t, os.Mkdir(filepath.Join(fixture.home, "dup"), 0o755))

	response := fixture.post("/workspaces/directories/create", url.Values{"parent": {fixture.home}, "name": {"dup"}})

	assert.Contains(t, response.Body.String(), "error de E/S")
	assert.Contains(t, response.Body.String(), `value="`+fixture.home+`"`)
}

func TestCreateDirectoryRejectsInvalidNamesInTheDialog(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.post("/workspaces/directories/create", url.Values{"parent": {fixture.home}, "name": {"a/b"}})

	assert.Contains(t, response.Body.String(), "nombre de carpeta inválido: a/b")
}

func TestWrongMethodIsRejected(t *testing.T) {
	fixture := newHTTPFixture(t)

	response := fixture.get("/workspaces/register")

	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
}

func TestWorkspacesTabRendersTheRegistrationAndTheList(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.register("Demo", newGitRepo(t))
	tab := fixture.module.Tabs()[0]

	body := renderTab(t, tab.Render(spanishContext(), dock.Params{}))

	assert.Contains(t, body, "<h1")
	assert.Contains(t, body, `id="workspace-register-slot"`)
	assert.Contains(t, body, `id="workspace-picker"`)
	assert.Contains(t, body, `href="/workspaces/`+fixture.registeredID()+`/edit"`, "each row links to its edit tab")
	assert.Contains(t, body, "Demo")
}

func TestEditTabPrefillsTheWorkspace(t *testing.T) {
	fixture := newHTTPFixture(t)
	repo := newGitRepo(t)
	fixture.register("Demo", repo)
	id := fixture.registeredID()
	tab := fixture.module.Tabs()[1]

	body := renderTab(t, tab.Render(spanishContext(), dock.Params{"id": id}))

	assert.Contains(t, body, `value="Demo"`, "the name is prefilled")
	assert.Contains(t, body, `value="main"`, "the base branch is prefilled")
	assert.Contains(t, body, repo, "the path is shown read-only")
	assert.Contains(t, body, `hx-post="/workspaces/`+id+`/update"`)
	assert.Contains(t, body, "Guardar cambios")
}

func TestEditTabOfUnknownWorkspaceShowsTheError(t *testing.T) {
	fixture := newHTTPFixture(t)
	tab := fixture.module.Tabs()[1]
	unknown, err := models.NewWorkspaceID()
	require.NoError(t, err)

	body := renderTab(t, tab.Render(spanishContext(), dock.Params{"id": unknown.String()}))

	assert.Contains(t, body, "workspace no encontrado: "+unknown.String())
	assert.NotContains(t, body, "hx-post")
}

func renderTab(t *testing.T, component templ.Component) string {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, component.Render(spanishContext(), &buffer))
	return buffer.String()
}

func spanishContext() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
}
