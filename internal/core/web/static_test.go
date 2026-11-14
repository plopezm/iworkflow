package web_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/web"
)

func serveStatic(t *testing.T, name string, body string) *httptest.ResponseRecorder {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	rec := httptest.NewRecorder()
	web.StaticHandler(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/"+name, nil))
	return rec
}

func TestStaticHandlerServesTheWebManifestWithItsOwnMediaType(t *testing.T) {
	rec := serveStatic(t, "manifest.webmanifest", `{"name":"x"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/manifest+json", rec.Header().Get("Content-Type"))
	require.JSONEq(t, `{"name":"x"}`, rec.Body.String())
}

func TestStaticHandlerKeepsTheMediaTypeOfOtherAssets(t *testing.T) {
	rec := serveStatic(t, "app.css", `a{}`)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/css")
}
