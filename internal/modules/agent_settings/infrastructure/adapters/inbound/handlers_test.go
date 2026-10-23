package inbound_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/agent_settings/application"
	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
)

// fakeRepository keeps the settings in memory; saveErr makes every save fail.
type fakeRepository struct {
	settings models.AgentSettings
	saveErr  error
}

func (f *fakeRepository) Get(context.Context) (models.AgentSettings, error) { return f.settings, nil }

func (f *fakeRepository) Save(_ context.Context, settings models.AgentSettings) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.settings = settings
	return nil
}

// fakeModels offers a fixed list for every runtime.
type fakeModels struct{ list []models.ModelRef }

func (f *fakeModels) List(context.Context, models.AgentRuntime) ([]models.ModelRef, error) {
	return f.list, nil
}

// fakeProfiles offers a fixed catalog.
type fakeProfiles struct{ catalog models.ProfileCatalog }

func (f *fakeProfiles) List(context.Context) (models.ProfileCatalog, error) { return f.catalog, nil }

type harness struct {
	router     http.Handler
	repository *fakeRepository
}

func newHarness(t *testing.T, repository *fakeRepository) harness {
	t.Helper()
	active := "Claude"
	catalogModels := &fakeModels{list: []models.ModelRef{{Provider: "anthropic", Model: "claude-5"}}}
	profiles := &fakeProfiles{catalog: models.ProfileCatalog{Active: &active, Names: []string{"Claude", "Gentleman"}}}
	handlers := inbound.NewHandlers(
		application.NewGetAgentSettings(repository),
		application.NewUpdateAgentSettings(repository, catalogModels, profiles),
		application.NewListModels(repository, catalogModels),
		application.NewListProfiles(profiles),
	)
	router := chi.NewRouter()
	handlers.Mount(router)
	// The app wraps every route in the locale middleware; the tests do the same.
	return harness{router: web.Locale(router), repository: repository}
}

func (h harness) post(t *testing.T, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func (h harness) get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func (h harness) send(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rec
}

func TestBarRendersTheFourSelectorsWithTheStoredValues(t *testing.T) {
	pi := models.RuntimePi
	h := newHarness(t, &fakeRepository{settings: models.AgentSettings{Runtime: &pi}})

	rec := h.get(t, "/agent-settings/bar")

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `id="agent-settings-bar"`)
	require.Contains(t, body, `<option value="pi" selected>pi</option>`)
	require.Contains(t, body, `hx-post="/agent-settings/runtime"`)
	require.Contains(t, body, `hx-post="/agent-settings/model"`)
	require.Contains(t, body, `hx-post="/agent-settings/thinking"`)
	require.Contains(t, body, `hx-post="/agent-settings/profile"`)
	require.Contains(t, body, "Applies to new tasks.", "no cookie means English")
}

func TestBarIsTranslatedFromTheLanguageCookie(t *testing.T) {
	h := newHarness(t, &fakeRepository{})
	req := httptest.NewRequest(http.MethodGet, "/agent-settings/bar", nil)
	req.Header.Set("Cookie", "iw-lang=es")
	rec := httptest.NewRecorder()

	h.router.ServeHTTP(rec, req)

	body := rec.Body.String()
	require.Contains(t, body, "Se aplica a las tareas nuevas.")
	require.Contains(t, body, `<span class="sr-only">Razonamiento</span>`)
	require.Contains(t, body, `<option value="" selected>Por defecto</option>`)
	require.Contains(t, body, `<option value="off">No reasoning</option>`)
}

func TestBarReportsTheSavedModelAsSelectedAndGroupsItByProvider(t *testing.T) {
	model := models.ModelRef{Provider: "anthropic", Model: "claude-5"}
	h := newHarness(t, &fakeRepository{settings: models.AgentSettings{Model: &model}})

	body := h.get(t, "/agent-settings/bar").Body.String()

	require.Contains(t, body, `<optgroup label="anthropic">`)
	require.Contains(t, body, `selected>claude-5</option>`)
}

func TestBarChangeSavesTheRuntimeAndReturnsTheBarWithoutNotice(t *testing.T) {
	repository := &fakeRepository{}
	h := newHarness(t, repository)

	rec := h.post(t, "/agent-settings/runtime", url.Values{"value": {"claude"}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repository.settings.Runtime)
	require.Equal(t, models.RuntimeClaude, *repository.settings.Runtime)
	require.Contains(t, rec.Body.String(), `id="agent-settings-bar"`)
	require.NotContains(t, rec.Body.String(), "hx-swap-oob")
}

func TestRuntimeChangeClearsTheSavedModel(t *testing.T) {
	model := models.ModelRef{Provider: "anthropic", Model: "claude-5"}
	repository := &fakeRepository{settings: models.AgentSettings{Model: &model}}
	h := newHarness(t, repository)

	h.post(t, "/agent-settings/runtime", url.Values{"value": {"pi"}})

	require.Nil(t, repository.settings.Model)
}

func TestModelChangeKeepsTheRuntime(t *testing.T) {
	pi := models.RuntimePi
	repository := &fakeRepository{settings: models.AgentSettings{Runtime: &pi}}
	h := newHarness(t, repository)

	rec := h.post(t, "/agent-settings/model", url.Values{"value": {dto.EncodeModelValue("anthropic", "claude-5")}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repository.settings.Model)
	require.Equal(t, "claude-5", repository.settings.Model.Model)
	require.Equal(t, &pi, repository.settings.Runtime)
}

func TestRejectedChangeReturnsTheStoredBarWithAnOutOfBandNotice(t *testing.T) {
	repository := &fakeRepository{}
	h := newHarness(t, repository)

	rec := h.post(t, "/agent-settings/profile", url.Values{"value": {"Ghost"}})

	require.Equal(t, http.StatusOK, rec.Code, "htmx swaps only successful responses")
	body := rec.Body.String()
	require.Contains(t, body, `hx-swap-oob="beforeend:#toasts"`)
	require.Contains(t, body, "perfil no encontrado: Ghost")
	require.Contains(t, body, `id="agent-settings-bar"`, "the bar is returned from the store")
	require.Nil(t, repository.settings.Profile)
}

func TestInvalidThinkingLevelIsRejectedWithItsMessage(t *testing.T) {
	h := newHarness(t, &fakeRepository{})

	body := h.post(t, "/agent-settings/thinking", url.Values{"value": {"ultra"}}).Body.String()

	require.Contains(t, body, "nivel de razonamiento inválido: ultra")
}

func TestFailedSaveIsReportedInTheNotice(t *testing.T) {
	h := newHarness(t, &fakeRepository{saveErr: agenterrors.Repository(errors.New("locked"))})

	body := h.post(t, "/agent-settings/runtime", url.Values{"value": {"pi"}}).Body.String()

	require.Contains(t, body, "error de persistencia: locked")
}

func TestLimitsCardShowsTheStoredCapsAndDisablesThemWhenOff(t *testing.T) {
	memory := uint32(2048)
	cpu := uint32(100)
	h := newHarness(t, &fakeRepository{settings: models.AgentSettings{
		MemoryLimitMB: &memory, CPULimitPercent: &cpu, LimitsDisabled: true,
	}})

	body := h.get(t, "/agent-settings/limits").Body.String()

	require.Contains(t, body, `id="agent-settings-limits"`)
	require.Contains(t, body, `value="2048"`)
	require.Contains(t, body, `value="100"`)
	require.Equal(t, 2, strings.Count(body, "disabled"), "both caps are disabled while the switch is off")
}

func TestLimitChangesStoreTheCapAndReturnTheCard(t *testing.T) {
	repository := &fakeRepository{}
	h := newHarness(t, repository)

	rec := h.post(t, "/agent-settings/limits/memory", url.Values{"value": {"512"}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repository.settings.MemoryLimitMB)
	require.Equal(t, uint32(512), *repository.settings.MemoryLimitMB)
	require.Contains(t, rec.Body.String(), `id="agent-settings-limits"`)
}

func TestZeroCapRemovesTheLimit(t *testing.T) {
	memory := uint32(512)
	repository := &fakeRepository{settings: models.AgentSettings{MemoryLimitMB: &memory}}
	h := newHarness(t, repository)

	h.post(t, "/agent-settings/limits/memory", url.Values{"value": {"0"}})

	require.Nil(t, repository.settings.MemoryLimitMB)
}

func TestLimitsSwitchIsOnlyOffWhenTheCheckboxIsUnchecked(t *testing.T) {
	repository := &fakeRepository{}
	h := newHarness(t, repository)

	h.post(t, "/agent-settings/limits/enabled", url.Values{})
	require.True(t, repository.settings.LimitsDisabled)

	h.post(t, "/agent-settings/limits/enabled", url.Values{"enabled": {"true"}})
	require.False(t, repository.settings.LimitsDisabled)
}

func TestFailedLimitChangeKeepsTheStoredValuesAndShowsTheNotice(t *testing.T) {
	memory := uint32(256)
	repository := &fakeRepository{
		settings: models.AgentSettings{MemoryLimitMB: &memory},
		saveErr:  agenterrors.Repository(errors.New("locked")),
	}
	h := newHarness(t, repository)

	body := h.post(t, "/agent-settings/limits/memory", url.Values{"value": {"999"}}).Body.String()

	require.Contains(t, body, "error de persistencia: locked")
	require.Contains(t, body, `value="256"`)
}

func TestJSONGetReturnsTheStoredSettings(t *testing.T) {
	pi := models.RuntimePi
	h := newHarness(t, &fakeRepository{settings: models.AgentSettings{Runtime: &pi}})

	rec := h.get(t, "/api/agent-settings")

	require.Equal(t, http.StatusOK, rec.Code)
	var got dto.AgentSettingsDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "pi", *got.Runtime)
}

func TestJSONPutSavesAndReturnsTheSettings(t *testing.T) {
	repository := &fakeRepository{}
	h := newHarness(t, repository)

	rec := h.send(t, http.MethodPut, "/api/agent-settings", `{"runtime":"claude","thinking":"max"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repository.settings.Runtime)
	require.Equal(t, models.RuntimeClaude, *repository.settings.Runtime)
	require.JSONEq(t, `{"runtime":"claude","model_provider":null,"model_id":null,"profile":null,
		"thinking":"max","memory_limit_mb":null,"cpu_limit_percent":null,"limits_disabled":false}`, rec.Body.String())
}

func TestJSONPutMapsDomainErrorsToStatusAndMessage(t *testing.T) {
	cases := map[string]struct {
		body    string
		status  int
		message string
	}{
		"invalid runtime":   {`{"runtime":"codex"}`, http.StatusBadRequest, "runtime de agente inválido: codex"},
		"invalid profile":   {`{"profile":"bad name"}`, http.StatusBadRequest, "nombre de perfil inválido: bad name"},
		"zero limit":        {`{"memory_limit_mb":0}`, http.StatusBadRequest, "límite de recursos inválido: el límite de memoria debe ser mayor que 0"},
		"unknown profile":   {`{"profile":"Ghost"}`, http.StatusUnprocessableEntity, "perfil no encontrado: Ghost"},
		"unknown model":     {`{"model_provider":"x","model_id":"y"}`, http.StatusUnprocessableEntity, "modelo no disponible: x/y"},
		"malformed payload": {`{"runtime":`, http.StatusBadRequest, ""},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, &fakeRepository{})

			rec := h.send(t, http.MethodPut, "/api/agent-settings", testCase.body)

			require.Equal(t, testCase.status, rec.Code)
			var payload map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			if testCase.message != "" {
				require.Equal(t, testCase.message, payload["error"])
			}
		})
	}
}

func TestJSONListsReplaceTheRustServerFunctions(t *testing.T) {
	h := newHarness(t, &fakeRepository{})

	runtimes := h.get(t, "/api/agent-settings/runtimes")
	require.JSONEq(t, `["gentle-shell","pi","claude","cursor"]`, runtimes.Body.String())

	levels := h.get(t, "/api/agent-settings/thinking-levels?runtime=claude")
	require.JSONEq(t, `["low","medium","high","xhigh","max"]`, levels.Body.String())

	badLevels := h.get(t, "/api/agent-settings/thinking-levels?runtime=codex")
	require.Equal(t, http.StatusBadRequest, badLevels.Code)

	models := h.get(t, "/api/agent-settings/models")
	require.JSONEq(t, `[{"provider":"anthropic","model":"claude-5"}]`, models.Body.String())

	profiles := h.get(t, "/api/agent-settings/profiles")
	require.JSONEq(t, `{"active":"Claude","names":["Claude","Gentleman"]}`, profiles.Body.String())
}
