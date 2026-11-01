package inbound_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

func TestRuntimesEndpointListsOnlyTheInstalledRuntimes(t *testing.T) {
	h := newHarnessWith(t, &fakeRepository{}, onlyInstalled{models.RuntimePi: true, models.RuntimeCursor: true})
	rec := httptest.NewRecorder()

	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/agent-settings/runtimes", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `["pi","cursor"]`, rec.Body.String())
}

func TestRuntimesEndpointIsAnEmptyArrayWhenNothingIsInstalled(t *testing.T) {
	h := newHarnessWith(t, &fakeRepository{}, onlyInstalled{})
	rec := httptest.NewRecorder()

	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/agent-settings/runtimes", nil))

	require.JSONEq(t, `[]`, rec.Body.String())
}

func TestBarOffersOnlyTheInstalledRuntimes(t *testing.T) {
	h := newHarnessWith(t, &fakeRepository{}, onlyInstalled{models.RuntimePi: true})
	rec := httptest.NewRecorder()

	h.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/agent-settings/bar", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `value="pi"`)
	require.NotContains(t, rec.Body.String(), `value="claude"`)
	require.NotContains(t, rec.Body.String(), `value="cursor"`)
}
