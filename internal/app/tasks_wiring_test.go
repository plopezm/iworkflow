package app_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/app"
	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/listen"
	"iworkflow/internal/core/restart"
)

// composeServer composes the whole server over a migrated temporary database. Nothing is started yet: the tests call
// Start and Shutdown themselves.
func composeServer(t *testing.T) *app.App {
	t.Helper()
	application, err := app.New(app.Deps{
		DB:        dbtest.New(t),
		Config:    app.NewTestConfig(t),
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		StaticDir: t.TempDir(),
		Listen:    listen.Info{Addr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 3000)},
		Restart:   restart.NewHandle(),
	})
	require.NoError(t, err)
	return application
}

// Rule from source: the task streams and the attachment content are served by the tasks module, at the Rust paths,
// behind the same router as the other modules.
func TestTaskRoutesAreMountedAtTheRustPaths(t *testing.T) {
	t.Parallel()
	router, isRoutes := composeServer(t).Router().(chi.Routes)
	require.True(t, isRoutes)

	routes := map[string]bool{}
	require.NoError(t, chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = true
		return nil
	}))

	assert.True(t, routes["GET /api/tasks/{id}/events"], "task events stream")
	assert.True(t, routes["GET /api/tasks/events"], "board events stream")
	assert.True(t, routes["GET /api/tasks/{task_id}/attachments/{attachment_id}"], "attachment content")
}

// The board stream is live: a served request answers with an event stream and stays open until the client leaves.
func TestBoardEventsStreamIsLive(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(composeServer(t).Router())
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/tasks/events", nil)
	require.NoError(t, err)

	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, response.Header.Get("Content-Type"), "text/event-stream")
}

// Rule from source: Start runs the reconciliation and the sweep, then the graph and resume loops; they keep sweeping
// until Shutdown, which stops them and waits for them, so no sweep happens after it returns.
func TestStartRunsTheLoopsUntilShutdown(t *testing.T) {
	t.Parallel()
	application := composeServer(t)
	application.SetLoopIntervals(time.Millisecond, time.Millisecond)
	require.NoError(t, application.Start(context.Background()))

	require.Eventually(t, func() bool {
		graph, resume := application.LoopSweeps()
		return graph >= 2 && resume >= 2
	}, 10*time.Second, 5*time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, application.Shutdown(stopCtx))
	graphAtStop, resumeAtStop := application.LoopSweeps()
	time.Sleep(50 * time.Millisecond)
	graph, resume := application.LoopSweeps()
	assert.Equal(t, graphAtStop, graph, "graph loop swept after shutdown")
	assert.Equal(t, resumeAtStop, resume, "resume loop swept after shutdown")
}

// Start owns the loops once: a second start is refused instead of leaking the first pair of goroutines.
func TestStartRefusesASecondStart(t *testing.T) {
	t.Parallel()
	application := composeServer(t)
	application.SetLoopIntervals(time.Hour, time.Hour)
	require.NoError(t, application.Start(context.Background()))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		assert.NoError(t, application.Shutdown(stopCtx))
	})

	err := application.Start(context.Background())

	require.ErrorContains(t, err, "already running")
}

// A server that never started shuts down cleanly: the stop steps have nothing to stop.
func TestShutdownWithoutStartIsClean(t *testing.T) {
	t.Parallel()
	application := composeServer(t)
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, application.Shutdown(stopCtx))
}
