package app_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/app"
	"iworkflow/internal/core/config"
	"iworkflow/internal/core/db/dbtest"
)

func newRouter(t *testing.T) http.Handler {
	t.Helper()
	static := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(static, "app.css"), []byte("body{}"), 0o600))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return app.New(dbtest.New(t), config.Config{}, logger, static).Router()
}

func TestHealthzAnswersNoContent(t *testing.T) {
	rec := httptest.NewRecorder()

	newRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestStaticAssetsAreServed(t *testing.T) {
	rec := httptest.NewRecorder()

	newRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/app.css", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "body{}", rec.Body.String())
}
