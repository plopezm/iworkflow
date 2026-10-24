//go:build unix

package terminal_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/terminal/infrastructure/di"
)

// wsServer mounts the WebSocket transport of a freshly built module, the way the composition root will.
type wsServer struct {
	module *di.Module
	server *httptest.Server
}

func newWSServer(t *testing.T) *wsServer {
	t.Helper()
	module := di.NewModule(di.Dependencies{
		WorkspaceResolver: func(context.Context, uuid.UUID) (*models.WorkspaceTarget, error) { return nil, nil },
		TaskResolver:      func(context.Context, uuid.UUID) (*models.TaskTarget, error) { return nil, nil },
		HomeDir:           t.TempDir(),
		Shell:             "sh",
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
		defer cancel()
		require.NoError(t, module.Shutdown(ctx))
	})
	socket := inbound.NewTerminalWebSocket(module.AttachTerminal, module.WriteTerminal, module.ResizeTerminal, slog.New(slog.DiscardHandler))
	router := chi.NewRouter()
	router.Get("/terminal/ws/session/{session_id}", socket.ServeHTTP)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &wsServer{module: module, server: server}
}

// openSession opens a home terminal through the use case and returns its id.
func (s *wsServer) openSession(t *testing.T) models.TerminalSessionID {
	t.Helper()
	session, err := s.module.OpenHomeTerminal.Execute(context.Background(), testSize)
	require.NoError(t, err)
	return session.ID
}

// url returns the WebSocket address of a session, with an optional query string.
func (s *wsServer) url(id models.TerminalSessionID, query string) string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + "/terminal/ws/session/" + id.String() + query
}

// dial connects to the session; the handshake is bounded by the test timeout.
func (s *wsServer) dial(t *testing.T, id models.TerminalSessionID, query string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, s.url(id, query), nil) //nolint:bodyclose // a successful upgrade hands the body to the connection; there is none to close
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() }) // the test is over; a failed close needs no action
	return conn
}

// readUntilCount reads binary frames until the accumulated output holds needle at least count times.
func readUntilCount(t *testing.T, conn *websocket.Conn, needle string, count int) string {
	t.Helper()
	var collected strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	for strings.Count(collected.String(), needle) < count {
		_, payload, err := conn.Read(ctx)
		require.NoError(t, err, "wanted %d x %q, got %q", count, needle, collected.String())
		collected.Write(payload)
	}
	return collected.String()
}

func TestWebSocketSendsOutputAndAcceptsInput(t *testing.T) {
	server := newWSServer(t)
	conn := server.dial(t, server.openSession(t), "")

	require.NoError(t, conn.Write(context.Background(), websocket.MessageBinary, []byte("echo ws_marker\n")))
	readUntilCount(t, conn, "ws_marker", 2)
}

func TestWebSocketTextFrameResizesTheSession(t *testing.T) {
	server := newWSServer(t)
	conn := server.dial(t, server.openSession(t), "")

	require.NoError(t, conn.Write(context.Background(), websocket.MessageText, []byte(`{"type":"resize","cols":100,"rows":30}`)))
	// An unknown control frame is ignored and must not close the socket.
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText, []byte(`{"type":"unknown"}`)))
	require.NoError(t, conn.Write(context.Background(), websocket.MessageBinary, []byte("stty size | sed 's/^/size=/'\n")))
	readUntilCount(t, conn, "size=30 100", 1)
}

func TestWebSocketInitialSizeQueryResizesBeforeTheFirstOutput(t *testing.T) {
	server := newWSServer(t)
	conn := server.dial(t, server.openSession(t), "?cols=120&rows=40")

	require.NoError(t, conn.Write(context.Background(), websocket.MessageBinary, []byte("stty size | sed 's/^/size=/'\n")))
	readUntilCount(t, conn, "size=40 120", 1)
}

func TestWebSocketReconnectReplaysScrollbackWithoutClosingTheSession(t *testing.T) {
	server := newWSServer(t)
	id := server.openSession(t)
	first := server.dial(t, id, "")
	require.NoError(t, first.Write(context.Background(), websocket.MessageBinary, []byte("echo previous_marker\n")))
	readUntilCount(t, first, "previous_marker", 2)
	require.NoError(t, first.CloseNow())

	// The echo of the command and the output are the two occurrences that the scrollback replays.
	second := server.dial(t, id, "")
	replayed := readUntilCount(t, second, "previous_marker", 2)
	require.Equal(t, 2, strings.Count(replayed, "previous_marker"))
	require.NoError(t, second.Write(context.Background(), websocket.MessageBinary, []byte("echo after_reconnect\n")))
	readUntilCount(t, second, "after_reconnect", 2)

	sessions, err := server.module.ListTerminals.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, models.TerminalStatusRunning, sessions[0].Status)
}

func TestWebSocketClosesWhenTheShellExits(t *testing.T) {
	server := newWSServer(t)
	conn := server.dial(t, server.openSession(t), "")
	require.NoError(t, conn.Write(context.Background(), websocket.MessageBinary, []byte("exit\n")))

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	var err error
	for err == nil {
		_, _, err = conn.Read(ctx)
	}
	require.Equal(t, websocket.StatusNormalClosure, websocket.CloseStatus(err), "the socket should close normally when the shell ends: %v", err)
}

func TestWebSocketRejectsForeignOriginsBeforeUpgrading(t *testing.T) {
	server := newWSServer(t)
	id := server.openSession(t)
	request, err := http.NewRequest(http.MethodGet, server.server.URL+"/terminal/ws/session/"+id.String(), nil)
	require.NoError(t, err)
	request.Header.Set("Origin", "http://evil.example")

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close() //nolint:errcheck // the body is read for the assertion below

	require.Equal(t, http.StatusForbidden, response.StatusCode)
	require.Equal(t, "origen no permitido", bodyText(t, response))
}

func TestWebSocketReportsUnknownSessionsAndMalformedIDs(t *testing.T) {
	server := newWSServer(t)

	unknown, err := http.Get(server.server.URL + "/terminal/ws/session/" + models.NewTerminalSessionID().String())
	require.NoError(t, err)
	defer unknown.Body.Close() //nolint:errcheck // the body is read for the assertion below
	require.Equal(t, http.StatusNotFound, unknown.StatusCode)
	require.Equal(t, "la sesión de terminal no existe", bodyText(t, unknown))

	malformed, err := http.Get(server.server.URL + "/terminal/ws/session/nope")
	require.NoError(t, err)
	defer malformed.Body.Close() //nolint:errcheck // the body is read for the assertion below
	require.Equal(t, http.StatusBadRequest, malformed.StatusCode)
	require.Equal(t, "id de sesión inválido", bodyText(t, malformed))
}

// bodyText reads a short response body and trims it.
func bodyText(t *testing.T, response *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return strings.TrimSpace(string(body))
}
