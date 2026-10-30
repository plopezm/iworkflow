package agent_settings_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/agent_settings/infrastructure/di"
	"iworkflow/internal/modules/agent_settings/presentation/pages"
)

// httpRouter mounts the module the way the composition root does: the locale middleware, then the
// module routes. The module is built on a fresh database and a gentle-pi home without profiles.
func httpRouter(t *testing.T) (http.Handler, *di.Module) {
	t.Helper()
	module, _ := newTestModule(t, "")
	router := chi.NewRouter()
	router.Use(web.Locale)
	module.Routes(router)
	return router, module
}

func send(t *testing.T, router http.Handler, method, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestBarChangeThroughHTTPIsPersistedAndVisibleInTheJSONAPI(t *testing.T) {
	router, _ := httpRouter(t)

	change := send(t, router, http.MethodPost, "/agent-settings/runtime", url.Values{"value": {"claude"}})
	require.Equal(t, http.StatusOK, change.Code)
	require.Contains(t, change.Body.String(), `id="agent-settings-bar"`)

	api := send(t, router, http.MethodGet, "/api/agent-settings", nil)
	require.Equal(t, http.StatusOK, api.Code)
	var settings dto.AgentSettingsDTO
	require.NoError(t, json.Unmarshal(api.Body.Bytes(), &settings))
	require.Equal(t, "claude", *settings.Runtime)
}

func TestClaudeModelsCanBeSavedThroughTheJSONAPIAndAreRejectedWhenUnknown(t *testing.T) {
	router, _ := httpRouter(t)
	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/agent-settings", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	ok := put(`{"runtime":"claude","model_provider":"anthropic","model_id":"opus"}`)
	unknown := put(`{"runtime":"claude","model_provider":"anthropic","model_id":"gpt-9"}`)

	require.Equal(t, http.StatusOK, ok.Code)
	require.Equal(t, http.StatusUnprocessableEntity, unknown.Code)
	require.Contains(t, unknown.Body.String(), "modelo no disponible: anthropic/gpt-9")
}

func TestModelListForClaudeComesFromTheStaticAliases(t *testing.T) {
	router, _ := httpRouter(t)
	claude := send(t, router, http.MethodPost, "/agent-settings/runtime", url.Values{"value": {"claude"}})
	require.Equal(t, http.StatusOK, claude.Code)

	rec := send(t, router, http.MethodGet, "/api/agent-settings/models", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"model":"opus"`)
}

func TestLimitsRoundTripThroughTheCard(t *testing.T) {
	router, _ := httpRouter(t)

	require.Equal(t, http.StatusOK, send(t, router, http.MethodPost, "/agent-settings/limits/memory",
		url.Values{"value": {"4096"}}).Code)
	require.Equal(t, http.StatusOK, send(t, router, http.MethodPost, "/agent-settings/limits/enabled",
		url.Values{}).Code)

	card := send(t, router, http.MethodGet, "/agent-settings/limits", nil).Body.String()

	require.Contains(t, card, `value="4096"`)
	require.Equal(t, 2, strings.Count(card, "disabled"), "the switch is off, so both caps are disabled")
}

func TestSettingsTabRendersThroughTheDockRegistryWithTheModuleSection(t *testing.T) {
	_, module := httpRouter(t)
	language := pages.SettingsSection{
		ID:      "language",
		Label:   func(context.Context) string { return "Idioma" },
		Content: func(context.Context) templ.Component { return templ.Raw("<p>language</p>") },
	}
	registry, err := dock.NewRegistry(module.Tabs([]pages.SettingsSection{
		language,
		module.ResourcesSection(func(ctx context.Context) string { return pages.SettingsTitle(ctx) }),
	})...)
	require.NoError(t, err)

	var page bytes.Buffer
	require.NoError(t, registry.Render(context.Background(), "/settings/resources").Render(context.Background(), &page))

	require.Contains(t, page.String(), `id="settings-page"`)
	require.Contains(t, page.String(), `id="agent-settings-limits"`)
	require.Contains(t, page.String(), `aria-selected="true"`)
}

func TestSavedSettingsAreVisibleToTheTaskLaunch(t *testing.T) {
	// The launch reads what the bar saved: the runtime chosen in the UI reaches PrepareLaunch.
	router, module := httpRouter(t)
	require.Equal(t, http.StatusOK, send(t, router, http.MethodPost, "/agent-settings/runtime",
		url.Values{"value": {"pi"}}).Code)

	preferences, err := module.PrepareLaunch.Execute(context.Background(), gitRepo(t))

	require.NoError(t, err)
	require.NotNil(t, preferences.Runtime)
	require.Equal(t, models.RuntimePi, *preferences.Runtime)
}
