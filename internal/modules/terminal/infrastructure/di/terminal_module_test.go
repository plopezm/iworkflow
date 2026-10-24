package di

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/domain/models"
)

// newTestModule builds the module without any workspace or task, which is enough to check the wiring.
func newTestModule(t *testing.T) *Module {
	t.Helper()
	module := NewModule(Dependencies{
		WorkspaceResolver: func(context.Context, uuid.UUID) (*models.WorkspaceTarget, error) { return nil, nil },
		TaskResolver:      func(context.Context, uuid.UUID) (*models.TaskTarget, error) { return nil, nil },
		HomeDir:           t.TempDir(),
		Shell:             "sh",
	})
	t.Cleanup(func() {
		require.NoError(t, module.Shutdown(context.Background()))
	})
	return module
}

func TestModuleExposesTheFourTerminalTabs(t *testing.T) {
	module := newTestModule(t)

	names := make([]string, 0, 4)
	for _, route := range module.Tabs() {
		names = append(names, route.Name)
	}

	require.Equal(t, []string{"terminals", "terminal_session", "workspace_terminal", "task_terminal"}, names)
}

func TestRoutesMountTheHTMXEndpointsAndTheWebSocket(t *testing.T) {
	module := newTestModule(t)
	router := chi.NewRouter()
	module.Routes(router)

	listing := httptest.NewRecorder()
	router.ServeHTTP(listing, httptest.NewRequest(http.MethodGet, "/terminal/sessions", nil))
	require.Equal(t, http.StatusOK, listing.Code)

	socket := httptest.NewRecorder()
	router.ServeHTTP(socket, httptest.NewRequest(http.MethodGet, "/terminal/ws/session/nope", nil))
	require.Equal(t, http.StatusBadRequest, socket.Code, "the WebSocket route must reach the transport")
}

func TestShutdownWithoutSessionsSucceeds(t *testing.T) {
	module := newTestModule(t)

	require.NoError(t, module.Shutdown(context.Background()))
}
