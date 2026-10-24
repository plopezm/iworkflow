package agent_profiles_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	agentprofilesdi "iworkflow/internal/modules/agent_profiles/infrastructure/di"
)

// fakeModelOptions is the model catalog the composition root would provide.
type fakeModelOptions struct{}

func (fakeModelOptions) List(_ context.Context) ([]outbound.ModelOption, error) {
	return []outbound.ModelOption{{ID: "p/m", Label: "p/m"}, {ID: "a/b", Label: "a/b"}}, nil
}

// fixture is one module mounted on a router, with its folders in a temp directory.
type fixture struct {
	t        *testing.T
	router   chi.Router
	profiles string
	agents   string
}

// newFixture mounts the module with Spanish as the request language, as the e2e suite runs.
func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	profiles := filepath.Join(root, "gentle-pi")
	agents := filepath.Join(root, "agents")
	module := agentprofilesdi.NewModule(agentprofilesdi.Deps{
		ProfilesDir:  profiles,
		AgentsDir:    agents,
		ModelOptions: fakeModelOptions{},
	})
	router := chi.NewRouter()
	router.Use(spanish)
	module.Routes(router)
	return fixture{t: t, router: router, profiles: profiles, agents: agents}
}

func spanish(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := i18n.State{Locale: i18n.Es, Chosen: true}
		next.ServeHTTP(w, r.WithContext(i18n.WithState(r.Context(), state)))
	})
}

// form sends a method with url-encoded values and returns the recorded response.
func (f fixture) form(method string, path string, values url.Values) *httptest.ResponseRecorder {
	f.t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	return recorder
}

// get sends a GET and returns the recorded response.
func (f fixture) get(path string) *httptest.ResponseRecorder {
	f.t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("HX-Request", "true")
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	return recorder
}

// writeProfiles seeds profiles.json, so a test starts from a known store.
func (f fixture) writeProfiles(content string) {
	f.t.Helper()
	require.NoError(f.t, os.MkdirAll(f.profiles, 0o755))
	require.NoError(f.t, os.WriteFile(filepath.Join(f.profiles, "profiles.json"), []byte(content), 0o600))
}

func (f fixture) readProfiles() string {
	f.t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.profiles, "profiles.json"))
	require.NoError(f.t, err)
	return string(raw)
}

func (f fixture) createProfile(name string) {
	f.t.Helper()
	recorder := f.form(http.MethodPost, "/agent-profiles/profiles", url.Values{"name": {name}})
	require.Equal(f.t, http.StatusOK, recorder.Code, recorder.Body.String())
}

func TestCreateProfileRefreshesListAndDetailAndNotifies(t *testing.T) {
	f := newFixture(t)

	recorder := f.form(http.MethodPost, "/agent-profiles/profiles", url.Values{"name": {"ap-create"}})

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, "Perfil creado")
	require.Contains(t, body, `hx-swap-oob="beforeend:#toasts"`)
	require.Contains(t, body, `<div id="profiles-list" class="stack" hx-swap-oob="true">`)
	require.Contains(t, body, `<div id="profiles-detail" class="stack" hx-swap-oob="true">`)
	require.Contains(t, body, `<div id="profiles-dialog"></div>`)
	require.Contains(t, body, "ap-create")
}

func TestCreateProfileRejectsInvalidNameWithDialogError(t *testing.T) {
	f := newFixture(t)

	recorder := f.form(http.MethodPost, "/agent-profiles/profiles", url.Values{"name": {"bad name"}})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), `<div id="profiles-dialog">`)
	require.Contains(t, recorder.Body.String(), "nombre de perfil inválido: bad name")
}

func TestCreateProfileTakenNameIsConflict(t *testing.T) {
	f := newFixture(t)
	f.createProfile("taken")

	recorder := f.form(http.MethodPost, "/agent-profiles/profiles", url.Values{"name": {"taken"}})

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "el perfil ya existe: taken")
}

func TestNameHintDisablesSubmitWhileNameBreaksARule(t *testing.T) {
	f := newFixture(t)

	invalid := f.get("/agent-profiles/name-hint?name=-x")
	valid := f.get("/agent-profiles/name-hint?name=fast")

	require.Equal(t, http.StatusOK, invalid.Code)
	require.Contains(t, invalid.Body.String(), "Debe empezar por una letra o un dígito")
	require.Contains(t, invalid.Body.String(), `id="profiles-name-submit"`)
	require.Contains(t, invalid.Body.String(), "disabled")
	require.NotContains(t, valid.Body.String(), "Debe empezar")
	require.NotContains(t, valid.Body.String(), "disabled")
}

func TestNewDialogIsEmptyAndDisabledUntilTyped(t *testing.T) {
	f := newFixture(t)

	recorder := f.get("/agent-profiles/dialogs/new")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `id="profiles-dialog"`)
	require.Contains(t, recorder.Body.String(), "Nombre del perfil")
	require.Contains(t, recorder.Body.String(), "disabled")
}

func TestDuplicateDialogPrefillsCopySuffix(t *testing.T) {
	f := newFixture(t)

	recorder := f.get("/agent-profiles/dialogs/duplicate?source=a&selected=a")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `value="a-copia"`)
	require.Contains(t, recorder.Body.String(), "Duplicar perfil")
}

func TestDuplicateProfileCopiesRouting(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{"orchestrator":"p/m"}}}`)

	recorder := f.form(http.MethodPost, "/agent-profiles/profiles/a/duplicate", url.Values{"name": {"b"}, "selected": {"a"}})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Perfil duplicado")
	require.Contains(t, f.readProfiles(), `"b": {`)
}

func TestRenameMovesActiveMarkerAndSelection(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{},"c":{}}}`)

	recorder := f.form(http.MethodPost, "/agent-profiles/profiles/a/rename", url.Values{"name": {"z"}, "selected": {"a"}})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Perfil renombrado")
	require.Contains(t, f.readProfiles(), `"active": "z"`)
	require.Contains(t, recorder.Body.String(), `<h2 class="card-title">z</h2>`)
}

func TestDeleteActiveProfileIsRefusedInTheDetail(t *testing.T) {
	f := newFixture(t)
	seeded := `{"active":"a","profiles":{"a":{}}}`
	f.writeProfiles(seeded)

	recorder := f.form(http.MethodDelete, "/agent-profiles/profiles/a?selected=a", nil)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), `id="profiles-detail"`)
	require.Contains(t, recorder.Body.String(), "no se puede eliminar el perfil activo: a")
	require.Equal(t, seeded, f.readProfiles())
}

func TestDeleteSelectedProfileMovesToNeighbour(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{},"b":{},"c":{}}}`)

	recorder := f.form(http.MethodDelete, "/agent-profiles/profiles/b?selected=b", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Perfil eliminado")
	require.Contains(t, recorder.Body.String(), `<h2 class="card-title">c</h2>`)
	require.NotContains(t, f.readProfiles(), `"b": {`)
}

func TestDetailConfirmShowsDeletePanel(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{},"b":{}}}`)

	recorder := f.get("/agent-profiles/profiles/b/detail?confirm=delete")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Eliminar el perfil «b»?")
}

func TestSelectListsProfilesWithActiveBadge(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{},"b":{}}}`)

	recorder := f.get("/agent-profiles/select?selected=b")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, `<div id="profiles-list" class="stack">`)
	require.Contains(t, body, "chip chip-selected")
	require.Contains(t, body, "Activo")
	require.Contains(t, body, `<div id="profiles-detail" class="stack" hx-swap-oob="true">`)
}

func TestSaveProfileRoutingPersistsAndKeepsDraftOnError(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{}}}`)

	saved := f.form(http.MethodPost, "/agent-profiles/profiles/a/routing", url.Values{
		"agent":    {"orchestrator"},
		"model":    {"p/m"},
		"thinking": {"high"},
	})

	require.Equal(t, http.StatusOK, saved.Code)
	require.Contains(t, saved.Body.String(), "Enrutado guardado")
	require.Contains(t, f.readProfiles(), `"thinking": "high"`)

	rejected := f.form(http.MethodPost, "/agent-profiles/profiles/a/routing", url.Values{
		"agent":    {"sdd-apply"},
		"model":    {"p/m"},
		"thinking": {"ultra"},
	})

	require.Equal(t, http.StatusBadRequest, rejected.Code)
	require.Contains(t, rejected.Body.String(), "nivel de razonamiento inválido: ultra")
	require.Contains(t, rejected.Body.String(), `value="sdd-apply"`)
}

func TestProfileRoutingRowsAddAndRemove(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{}}}`)

	added := f.form(http.MethodPost, "/agent-profiles/profiles/a/routing/rows", url.Values{
		"action":    {"add"},
		"add_agent": {"orchestrator"},
	})
	require.Equal(t, http.StatusOK, added.Code)
	require.Contains(t, added.Body.String(), `value="orchestrator"`)

	removed := f.form(http.MethodPost, "/agent-profiles/profiles/a/routing/rows", url.Values{
		"action": {"remove"},
		"target": {"orchestrator"},
		"agent":  {"orchestrator"},
		"model":  {""},
	})
	require.Equal(t, http.StatusOK, removed.Code)
	require.NotContains(t, removed.Body.String(), `<li class="card`)
}

func TestExportDownloadsNamedEnvelope(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{"orchestrator":"p/m"}}}`)

	recorder := f.get("/agent-profiles/profiles/a/export")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, `attachment; filename=a.profile.json`, recorder.Header().Get("Content-Disposition"))
	require.Contains(t, recorder.Body.String(), `"kind": "gentle-pi.agent_model_profile"`)
}

func TestExportNoticeShowsExportedToast(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{}}}`)

	recorder := f.get("/agent-profiles/profiles/a/export-notice")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Exportado a.profile.json")
}

func TestImportPastedEnvelopeCreatesProfile(t *testing.T) {
	f := newFixture(t)
	envelope := `{"kind":"gentle-pi.agent_model_profile","version":1,"name":"ap-import","config":{"orchestrator":{"model":"p/m","thinking":"medium"}}}`
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("raw", envelope))
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/agent-profiles/profiles/import", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	f.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Perfil importado")
	require.Contains(t, f.readProfiles(), `"ap-import": {`)
}

func TestImportInvalidEnvelopeIsBadRequestWithDialogError(t *testing.T) {
	f := newFixture(t)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("raw", "not json"))
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/agent-profiles/profiles/import", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	f.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "importación inválida")
}

func TestCreateRoleWritesFileAndRefreshesRolesAndSelect(t *testing.T) {
	f := newFixture(t)
	f.writeProfiles(`{"active":"a","profiles":{"a":{}}}`)
	form := url.Values{
		"scope":       {"profiles"},
		"selected":    {"a"},
		"name":        {"sdd-review"},
		"description": {"Reviews changes"},
		"prompt":      {"Be strict."},
	}

	created := f.form(http.MethodPost, "/agent-profiles/roles", form)

	require.Equal(t, http.StatusOK, created.Code)
	body := created.Body.String()
	require.Contains(t, body, "Rol creado")
	require.Contains(t, body, `<div id="profiles-roles" class="stack" hx-swap-oob="true">`)
	require.Contains(t, body, `<select id="profiles-add-agent" name="add_agent" class="input w-auto" aria-label="Agente a añadir" hx-swap-oob="true">`)
	require.FileExists(t, filepath.Join(f.agents, "sdd-review.md"))

	duplicated := f.form(http.MethodPost, "/agent-profiles/roles", form)
	require.Equal(t, http.StatusConflict, duplicated.Code)
	require.Contains(t, duplicated.Body.String(), "el rol ya existe: sdd-review")
}

func TestCreateRoleFromModelsRefreshesModelsSelect(t *testing.T) {
	f := newFixture(t)

	recorder := f.form(http.MethodPost, "/agent-profiles/roles", url.Values{
		"scope":       {"models"},
		"name":        {"reviewer"},
		"description": {"d"},
		"prompt":      {"p"},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `id="models-add-agent"`)
	require.NotContains(t, recorder.Body.String(), `id="profiles-roles"`)
}

func TestRoleDialogRejectsUnknownScope(t *testing.T) {
	f := newFixture(t)

	recorder := f.get("/agent-profiles/dialogs/role?scope=elsewhere")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestSaveGlobalRoutingNamesTheSavedAgents(t *testing.T) {
	f := newFixture(t)

	one := f.form(http.MethodPost, "/agent-profiles/models/routing", url.Values{"agent": {"orchestrator"}, "model": {"p/m"}, "thinking": {""}})
	two := f.form(http.MethodPost, "/agent-profiles/models/routing", url.Values{
		"agent":    {"orchestrator", "sdd-apply"},
		"model":    {"p/m", ""},
		"thinking": {"", "high"},
	})

	require.Equal(t, http.StatusOK, one.Code)
	require.Contains(t, one.Body.String(), "Routing global guardado (1 agente)")
	require.Contains(t, two.Body.String(), "Routing global guardado (2 agentes)")
	require.Contains(t, two.Body.String(), `id="models-routing"`)
}

func TestTabsExposeProfilesAndModelsScreens(t *testing.T) {
	f := newFixture(t)
	module := agentprofilesdi.NewModule(agentprofilesdi.Deps{
		ProfilesDir:  f.profiles,
		AgentsDir:    f.agents,
		ModelOptions: fakeModelOptions{},
	})
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})

	routes := module.Tabs()

	require.Len(t, routes, 2)
	require.Equal(t, "/profiles", routes[0].Pattern)
	require.Equal(t, "Perfiles", routes[0].Title(ctx, nil))
	require.Equal(t, "/models", routes[1].Pattern)
	require.Equal(t, "Modelos", routes[1].Title(ctx, nil))

	var page bytes.Buffer
	require.NoError(t, routes[1].Render(ctx, nil).Render(ctx, &page))
	require.Contains(t, page.String(), `id="models-dialog"`)
	require.Contains(t, page.String(), `id="models-routing"`)
}

// The hint request sent by a field that was bound late carries the value typed before the binding. A
// valid name enables the submit button out of band; an invalid one keeps it disabled.
func TestNameHintLoadRequestEnablesSubmitForTypedValue(t *testing.T) {
	f := newFixture(t)

	valid := f.get("/agent-profiles/name-hint?name=ap-typed-early")
	invalid := f.get("/agent-profiles/name-hint?name=ap%20typed")

	require.Equal(t, http.StatusOK, valid.Code)
	require.Contains(t, valid.Body.String(), `id="profiles-name-submit"`)
	require.Contains(t, valid.Body.String(), `hx-swap-oob="true"`)
	require.NotContains(t, valid.Body.String(), "disabled")
	require.Contains(t, invalid.Body.String(), "Solo letras, dígitos")
	require.Contains(t, invalid.Body.String(), "disabled")
}
