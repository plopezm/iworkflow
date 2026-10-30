package preferences_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/preferences/infrastructure/di"
)

// fakeNetwork reports fixed interfaces, like the operating system would.
type fakeNetwork struct {
	interfaces   []models.NetworkInterface
	preferred    string
	hasPreferred bool
}

func (f *fakeNetwork) Interfaces(context.Context) []models.NetworkInterface {
	return f.interfaces
}

func (f *fakeNetwork) PreferredAddress(context.Context) (string, bool) {
	return f.preferred, f.hasPreferred
}

// fakeRuntime is a settable running-server description.
type fakeRuntime struct {
	status models.ServerStatus
}

func (f *fakeRuntime) Status() models.ServerStatus {
	return f.status
}

// fakeRestarter counts restart requests.
type fakeRestarter struct {
	requests int
}

func (f *fakeRestarter) RequestRestart() {
	f.requests++
}

// settings is the wired module: the settings routes, the same routes behind the access guard, and
// the fakes the tests steer.
type settings struct {
	module    *di.Module
	router    http.Handler
	guarded   http.Handler
	guard     *inbound.AccessGuard
	network   *fakeNetwork
	restarter *fakeRestarter
	db        *sql.DB
}

func newTestToken(t *testing.T) models.AccessToken {
	t.Helper()
	token, err := models.GenerateAccessToken()
	require.NoError(t, err)
	return token
}

func newSettings(t *testing.T, token models.AccessToken) settings {
	t.Helper()
	db := dbtest.New(t)
	guard := inbound.NewAccessGuard(token, true)
	network := &fakeNetwork{}
	restarter := &fakeRestarter{}
	module := di.NewModule(di.Deps{
		DB:        db,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Guard:     guard,
		Network:   network,
		Runtime:   &fakeRuntime{status: models.ServerStatus{Port: 3000}},
		Restarter: restarter,
	})
	router := chi.NewRouter()
	module.Routes(router)
	return settings{
		module:    module,
		router:    web.Locale(router),
		guarded:   inbound.RequireAccess(guard)(router),
		guard:     guard,
		network:   network,
		restarter: restarter,
		db:        db,
	}
}

// setNetworkAccess stores the saved network choice, failing the test on error.
func (s settings) setNetworkAccess(t *testing.T, enabled bool) {
	t.Helper()
	_, err := s.module.SaveNetworkAccess.Execute(context.Background(), enabled)
	require.NoError(t, err)
}

// send performs one request against handler and returns the recorded response.
func serve(t *testing.T, handler http.Handler, method string, target string, form url.Values, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// send performs a request as a Spanish-speaking browser (the tests assert Spanish texts).
func (s settings) send(t *testing.T, method string, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, s.router, method, target, form, map[string]string{"Cookie": "iw-lang=es"})
}

func TestPrivateCardShowsTheStatusAndNoSecret(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodGet, "/preferences/network", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `id="network-access"`)
	require.Contains(t, body, "Solo este equipo")
	require.Contains(t, body, "Permitir el acceso desde otros dispositivos de la red")
	require.Contains(t, body, "Puerto del servidor")
	require.NotContains(t, body, "Generar un token nuevo", "a private server has no token section")
	require.NotContains(t, body, "Exigir el token de acceso", "nothing to require while private")
}

func TestEnablingNetworkAccessPersistsAndAsksForARestart(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/network", url.Values{"enabled": {"true"}})

	require.Equal(t, http.StatusOK, rec.Code)
	enabled, err := s.module.GetNetworkAccess.Execute(context.Background())
	require.NoError(t, err)
	require.True(t, enabled)
	body := rec.Body.String()
	require.Contains(t, body, "Reinicia el servidor para aplicar el cambio.")
	require.Contains(t, body, "Reiniciar ahora")
	require.Contains(t, body, "Generar un token nuevo", "enabled access puts the token in play")
	require.Contains(t, body, "Exigir el token de acceso")
}

func TestNetworkToggleRejectsAMalformedRequest(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/network", url.Values{"enabled": {"maybe"}})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	enabled, err := s.module.GetNetworkAccess.Execute(context.Background())
	require.NoError(t, err)
	require.False(t, enabled, "nothing is stored for a malformed request")
}

func TestAnInvalidPortIsShownInTheCardAndNeverStored(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/port", url.Values{"port": {"80"}})

	require.Equal(t, http.StatusOK, rec.Code, "business errors are rendered inside the card")
	require.Contains(t, rec.Body.String(), "puerto no válido: 80 (usa un puerto entre 1024 y 65535)")
	stored, err := s.module.GetListenPort.Execute(context.Background())
	require.NoError(t, err)
	require.Nil(t, stored)
}

func TestAPortTypedAsGarbageIsReportedAsPortZero(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/port", url.Values{"port": {"abc"}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "puerto no válido: 0 (usa un puerto entre 1024 y 65535)")
}

func TestAValidPortIsStoredAndShownAsConfigured(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/port", url.Values{"port": {"9100"}})

	require.Equal(t, http.StatusOK, rec.Code)
	stored, err := s.module.GetListenPort.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, uint16(9100), stored.Port())
	require.Contains(t, rec.Body.String(), `value="9100"`)
}

func TestRequireTokenIsSavedAndOffShowsTheWarning(t *testing.T) {
	s := newSettings(t, newTestToken(t))
	s.setNetworkAccess(t, true)

	rec := s.send(t, http.MethodPost, "/preferences/require-token", url.Values{"required": {"false"}})

	require.Equal(t, http.StatusOK, rec.Code)
	required, err := s.module.GetRequireAccessToken.Execute(context.Background())
	require.NoError(t, err)
	require.False(t, required)
	require.Contains(t, rec.Body.String(), "Sin el token, cualquiera en la red puede usar iworkflow")
}

// TestRegenerateRotatesTheTokenAndHandsItToThisBrowser checks the guard end to end: after the
// rotation the old cookie is refused and the cookie this browser got is accepted.
func TestRegenerateRotatesTheTokenAndHandsItToThisBrowser(t *testing.T) {
	oldToken := newTestToken(t)
	s := newSettings(t, oldToken)
	s.setNetworkAccess(t, true)

	rec := s.send(t, http.MethodPost, "/preferences/token/regenerate", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	setCookie := rec.Header().Get("Set-Cookie")
	require.True(t, strings.HasPrefix(setCookie, "iw-token="), setCookie)
	fresh, _, _ := strings.Cut(strings.TrimPrefix(setCookie, "iw-token="), ";")
	require.NotEqual(t, oldToken.Value(), fresh)
	require.Contains(t, rec.Body.String(), fresh, "the card shows the new token")

	stale := serve(t, s.guarded, http.MethodGet, "/preferences/network", nil,
		map[string]string{"Cookie": "iw-token=" + oldToken.Value()})
	require.Equal(t, http.StatusUnauthorized, stale.Code, "the old token stops working at once")
	current := serve(t, s.guarded, http.MethodGet, "/preferences/network", nil,
		map[string]string{"Cookie": "iw-token=" + fresh})
	require.Equal(t, http.StatusOK, current.Code)
}

func TestRestartIsAcceptedAndTheCardPollsUntilTheServerIsBack(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/restart", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, s.restarter.requests)
	body := rec.Body.String()
	require.Contains(t, body, "Reiniciando… esta página se recargará cuando el servidor vuelva.")
	require.Contains(t, body, `hx-trigger="every 2s"`)
	require.NotContains(t, body, "Reiniciar ahora", "no second restart while one is running")
}

func TestLanguageChoiceStoresItSetsTheCookieAndReloads(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/language/de", nil)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "true", rec.Header().Get("HX-Refresh"))
	require.Equal(t, "iw-lang=de; Path=/; Max-Age=31536000; SameSite=Lax", rec.Header().Get("Set-Cookie"))
	language, err := s.module.GetLanguage.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, language)
	require.Equal(t, "de", language.Code())
}

func TestUnsupportedLanguageIsRejectedWithoutStoringOrCookie(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/language/pt", nil)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, rec.Header().Get("Set-Cookie"))
	language, err := s.module.GetLanguage.Execute(context.Background())
	require.NoError(t, err)
	require.Nil(t, language)
}

func TestTheTargetEndpointRendersTheLinkAndQRForTheChosenInterface(t *testing.T) {
	s := newSettings(t, newTestToken(t))
	s.setNetworkAccess(t, true)
	s.network.interfaces = []models.NetworkInterface{
		{Name: "eth0", Address: "192.168.1.5"},
		{Name: "tailscale0", Address: "100.64.0.7"},
	}

	rec := s.send(t, http.MethodGet, "/preferences/network/target?address=100.64.0.7", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `id="network-target"`)
	require.Contains(t, body, "http://100.64.0.7:3000/?token=")
	require.Contains(t, body, "<svg")
}

func TestTheCardListsEveryInterfaceAndTheFirstIsPreselected(t *testing.T) {
	s := newSettings(t, newTestToken(t))
	s.setNetworkAccess(t, true)
	s.network.interfaces = []models.NetworkInterface{
		{Name: "eth0", Address: "192.168.1.5"},
		{Name: "tailscale0", Address: "100.64.0.7"},
	}

	rec := s.send(t, http.MethodGet, "/preferences/network", nil)

	body := rec.Body.String()
	require.Contains(t, body, `value="192.168.1.5" selected`)
	require.Contains(t, body, `value="100.64.0.7"`)
	require.NotContains(t, body, `value="100.64.0.7" selected`)
}

func TestAStorageFailureIsShownAsAnErrorCard(t *testing.T) {
	s := newSettings(t, newTestToken(t))
	require.NoError(t, s.db.Close())

	rec := s.send(t, http.MethodGet, "/preferences/network", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "error de almacenamiento:")
	require.Contains(t, rec.Body.String(), `id="network-access"`)
}
