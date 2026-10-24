//go:build unix

package terminal_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/infrastructure/di"
)

// redirectPattern extracts the session id from the redirect the open endpoints answer with.
var redirectPattern = regexp.MustCompile(`data-dock-redirect="/terminals/([0-9a-f-]{36})"`)

// newRoutedServer mounts the module's routes the way the composition root does, over a real shell.
func newRoutedServer(t *testing.T) (*httptest.Server, *di.Module) {
	t.Helper()
	module := di.NewModule(di.Dependencies{
		WorkspaceResolver: func(context.Context, uuid.UUID) (*models.WorkspaceTarget, error) { return nil, nil },
		TaskResolver:      func(context.Context, uuid.UUID) (*models.TaskTarget, error) { return nil, nil },
		HomeDir:           t.TempDir(),
		Shell:             "sh",
		Logger:            slog.New(slog.DiscardHandler),
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
		defer cancel()
		require.NoError(t, module.Shutdown(ctx))
	})
	router := chi.NewRouter()
	module.Routes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, module
}

// post sends an empty POST and returns the body.
func post(t *testing.T, server *httptest.Server, path string) string {
	t.Helper()
	response, err := http.Post(server.URL+path, "text/plain", nil)
	require.NoError(t, err)
	defer response.Body.Close() //nolint:errcheck // the body is read for the assertion below
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return string(body)
}

// get sends a GET and returns the body.
func get(t *testing.T, server *httptest.Server, path string) string {
	t.Helper()
	response, err := http.Get(server.URL + path) //nolint:noctx // the test client needs no context
	require.NoError(t, err)
	defer response.Body.Close() //nolint:errcheck // the body is read for the assertion below
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return string(body)
}

func TestHomeTerminalFlowThroughTheHTTPRoutes(t *testing.T) {
	server, _ := newRoutedServer(t)

	opened := post(t, server, "/terminal/home")
	match := redirectPattern.FindStringSubmatch(opened)
	require.NotNil(t, match, "the open answer must redirect the tab to the session: %s", opened)
	sessionID := match[1]

	listing := get(t, server, "/terminal/sessions")
	require.Contains(t, listing, sessionID[:8])
	require.Contains(t, listing, "Home")

	socket, _, err := websocket.Dial(context.Background(), //nolint:bodyclose // a successful upgrade hands the body to the connection; there is none to close
		"ws"+strings.TrimPrefix(server.URL, "http")+"/terminal/ws/session/"+sessionID, nil)
	require.NoError(t, err)
	defer socket.CloseNow() //nolint:errcheck // the test is over; the socket is discarded
	require.NoError(t, socket.Write(context.Background(), websocket.MessageBinary, []byte("echo routed_marker\n")))
	readUntilCount(t, socket, "routed_marker", 2)

	closed := post(t, server, "/terminal/sessions/"+sessionID+"/close/page")
	require.Equal(t, `<span data-dock-redirect="/terminals"></span>`, closed)
	require.Contains(t, get(t, server, "/terminal/sessions"), "No open terminals.")
}

func TestOpeningAnUnknownWorkspaceShowsTheMessageNotAStatusError(t *testing.T) {
	server, _ := newRoutedServer(t)

	body := post(t, server, "/terminal/workspaces/"+uuid.New().String()+"/open")

	require.Contains(t, body, "el workspace no existe")
	require.NotContains(t, body, "data-dock-redirect")
}
