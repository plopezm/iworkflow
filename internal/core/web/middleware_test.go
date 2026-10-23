package web_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
)

func TestLocaleMiddlewareReadsTheCookie(t *testing.T) {
	var got i18n.State
	handler := web.Locale(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = i18n.StateFrom(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", "iw-lang=fr")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, i18n.State{Locale: i18n.Fr, Chosen: true}, got)
}

func TestBodyLimitRejectsOversizedBodies(t *testing.T) {
	handler := web.BodyLimit(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		}
	}))

	small := httptest.NewRecorder()
	handler.ServeHTTP(small, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345678")))
	large := httptest.NewRecorder()
	handler.ServeHTTP(large, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("123456789")))

	require.Equal(t, http.StatusOK, small.Code)
	require.Equal(t, http.StatusRequestEntityTooLarge, large.Code)
}

func TestServerFnBodyLimitFitsFiveImages(t *testing.T) {
	require.Equal(t, 30*1024*1024, web.ServerFnMaxBodyBytes)
}

func TestRecoverTurnsPanicsIntoInternalServerError(t *testing.T) {
	handler := web.Recover(slog.New(slog.NewTextHandler(io.Discard, nil)))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSSEWritesIDEventAndData(t *testing.T) {
	rec := httptest.NewRecorder()
	stream, ok := web.NewSSE(rec)
	require.True(t, ok)

	require.NoError(t, stream.Send(t.Context(), "7", "task", `{"a":1}`))

	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, "id: 7\nevent: task\ndata: {\"a\":1}\n\n", rec.Body.String())
}

func TestSSESendFailsAfterContextCancel(t *testing.T) {
	stream, _ := web.NewSSE(httptest.NewRecorder())
	canceled, stop := context.WithCancel(t.Context())
	stop()

	require.Error(t, stream.Send(canceled, "", "e", "d"))
}
