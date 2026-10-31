package logx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/logx"
	"iworkflow/internal/core/web"
)

func newLogger(out *bytes.Buffer) *slog.Logger {
	return slog.New(logx.NewStackHandler(slog.NewTextHandler(out, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

func explode() { panic("boom") }

func TestErrorRecordsCarryAStack(t *testing.T) {
	var out bytes.Buffer
	newLogger(&out).Error("unexpected failure", "error", "disk full")
	require.Contains(t, out.String(), "stack=")
	require.Contains(t, out.String(), "TestErrorRecordsCarryAStack")
}

func TestLowerLevelsHaveNoStack(t *testing.T) {
	var out bytes.Buffer
	logger := newLogger(&out)
	logger.Warn("invalid input", "error", "name is empty")
	logger.Info("started")
	logger.Debug("request")
	require.NotContains(t, out.String(), "stack=")
}

func TestStackSurvivesWithAttrsAndWithGroup(t *testing.T) {
	var out bytes.Buffer
	newLogger(&out).With("module", "tasks").WithGroup("g").Error("failed")
	require.Contains(t, out.String(), "module=tasks")
	require.Contains(t, out.String(), "stack=")
}

func TestRecoveredPanicStackPointsAtThePanickingFunction(t *testing.T) {
	var out bytes.Buffer
	handler := web.Recover(newLogger(&out))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { explode() }))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, out.String(), "logx_test.explode")
}
