package usagelimits_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/usage_limits/infrastructure/di"
)

// TestRoutesMountTheBarAndAnswers200 wires the module with providers that have no credentials and no
// sessions, so nothing is read from the machine: the bar is answered empty and hidden, still with 200.
func TestRoutesMountTheBarAndAnswers200(t *testing.T) {
	module := di.NewModule(di.Deps{
		ClaudeCredentialsPath: filepath.Join(t.TempDir(), "missing.json"),
		CodexSessionsDir:      t.TempDir(),
	})
	router := chi.NewRouter()
	module.Routes(router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage-limits/bar", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `id="usage-bar"`)
	assert.Contains(t, rec.Body.String(), `class="hidden"`)
}
