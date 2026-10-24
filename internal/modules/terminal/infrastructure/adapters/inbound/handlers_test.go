package inbound_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/application"
	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/outbound/resolver"
)

// fakePty is an in-memory PtyPort. It keeps the sessions it spawned and records the spawn arguments.
type fakePty struct {
	spawnErr error
	sessions []models.TerminalSession
	spawned  []spawnArgs
}

type spawnArgs struct {
	workspaceID uuid.UUID
	name        string
	cwd         string
}

func (f *fakePty) Spawn(_ context.Context, workspaceID uuid.UUID, name string, cwd string, _ models.TerminalSize) (models.TerminalSession, error) {
	f.spawned = append(f.spawned, spawnArgs{workspaceID: workspaceID, name: name, cwd: cwd})
	if f.spawnErr != nil {
		return models.TerminalSession{}, f.spawnErr
	}
	session := models.TerminalSession{
		ID:            models.NewTerminalSessionID(),
		WorkspaceID:   workspaceID,
		WorkspaceName: name,
		CreatedAt:     time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		Status:        models.TerminalStatusRunning,
	}
	f.sessions = append(f.sessions, session)
	return session, nil
}

func (f *fakePty) List(context.Context) ([]models.TerminalSession, error) {
	return f.sessions, nil
}

func (f *fakePty) Attach(context.Context, models.TerminalSessionID) (models.TerminalAttachment, error) {
	return models.TerminalAttachment{}, terminalerrors.ErrSessionNotFound
}

func (f *fakePty) Write(context.Context, models.TerminalSessionID, []byte) error {
	return nil
}

func (f *fakePty) Resize(context.Context, models.TerminalSessionID, models.TerminalSize) error {
	return nil
}

func (f *fakePty) Close(_ context.Context, id models.TerminalSessionID) error {
	for index, session := range f.sessions {
		if session.ID == id {
			f.sessions = append(f.sessions[:index], f.sessions[index+1:]...)
			return nil
		}
	}
	return terminalerrors.ErrSessionNotFound
}

// nothingFound resolves no workspace and no task.
func nothingFound(context.Context, uuid.UUID) (*models.WorkspaceTarget, error) { return nil, nil }

func noTask(context.Context, uuid.UUID) (*models.TaskTarget, error) { return nil, nil }

// newRouter mounts the handlers over the given fake, the way the module does.
func newRouter(t *testing.T, pty *fakePty, workspaces resolver.WorkspaceResolver, tasks resolver.TaskResolver) *chi.Mux {
	t.Helper()
	handlers := inbound.NewHandlers(inbound.UseCases{
		List:          application.NewListTerminals(pty),
		OpenWorkspace: application.NewOpenTerminal(pty, resolver.NewWorkspacePathAdapter(workspaces)),
		OpenTask:      application.NewOpenTaskTerminal(pty, resolver.NewTaskPathAdapter(tasks)),
		OpenHome:      application.NewOpenHomeTerminal(pty, "/home/u"),
		Close:         application.NewCloseTerminal(pty),
	})
	router := chi.NewRouter()
	handlers.Mount(router)
	return router
}

// send runs one request against the router and returns the status and the body.
func send(t *testing.T, router http.Handler, method string, target string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder.Code, recorder.Body.String()
}

func TestOpenHomeRedirectsTheTabToTheNewSession(t *testing.T) {
	pty := &fakePty{}
	router := newRouter(t, pty, nothingFound, noTask)

	status, body := send(t, router, http.MethodPost, "/terminal/home")

	require.Equal(t, http.StatusOK, status)
	require.Len(t, pty.sessions, 1)
	require.Equal(t, `<span data-dock-redirect="/terminals/`+pty.sessions[0].ID.String()+`"></span>`, body)
	require.Equal(t, "/home/u", pty.spawned[0].cwd)
	require.Equal(t, uuid.Nil, pty.spawned[0].workspaceID)
}

func TestOpenHomeAnswersTheErrorAsAFragmentWithStatusOK(t *testing.T) {
	pty := &fakePty{spawnErr: terminalerrors.Pty(errors.New("boom"))}
	router := newRouter(t, pty, nothingFound, noTask)

	status, body := send(t, router, http.MethodPost, "/terminal/home")

	require.Equal(t, http.StatusOK, status, "htmx swaps 200 responses only; the error must be a fragment")
	require.Contains(t, body, "error del pseudo-terminal: boom")
}

func TestOpenWorkspaceRedirectsTheTabWhenTheWorkspaceExists(t *testing.T) {
	workspaceID := uuid.New()
	pty := &fakePty{}
	workspaces := func(context.Context, uuid.UUID) (*models.WorkspaceTarget, error) {
		return &models.WorkspaceTarget{Name: "api", Path: "/repos/api"}, nil
	}
	router := newRouter(t, pty, workspaces, noTask)

	status, body := send(t, router, http.MethodPost, "/terminal/workspaces/"+workspaceID.String()+"/open")

	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `data-dock-redirect="/terminals/`)
	require.Equal(t, spawnArgs{workspaceID: workspaceID, name: "api", cwd: "/repos/api"}, pty.spawned[0])
}

func TestOpenWorkspaceRejectsAMalformedIDWithTheBackLink(t *testing.T) {
	pty := &fakePty{}
	router := newRouter(t, pty, nothingFound, noTask)

	status, body := send(t, router, http.MethodPost, "/terminal/workspaces/nope/open")

	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "id de workspace inválido")
	require.Contains(t, body, `href="/workspaces"`)
	require.Empty(t, pty.spawned, "a rejected id must not reach the PTY")
}

func TestOpenWorkspaceReportsAMissingWorkspace(t *testing.T) {
	router := newRouter(t, &fakePty{}, nothingFound, noTask)

	_, body := send(t, router, http.MethodPost, "/terminal/workspaces/"+uuid.New().String()+"/open")

	require.Contains(t, body, "el workspace no existe")
}

func TestOpenTaskRedirectsTheTabWhenTheTaskHasAWorktree(t *testing.T) {
	taskID := uuid.New()
	workspaceID := uuid.New()
	pty := &fakePty{}
	tasks := func(context.Context, uuid.UUID) (*models.TaskTarget, error) {
		return &models.TaskTarget{WorkspaceID: workspaceID, Name: "fix bug", Path: "/wt"}, nil
	}
	router := newRouter(t, pty, nothingFound, tasks)

	_, body := send(t, router, http.MethodPost, "/terminal/tasks/"+taskID.String()+"/open")

	require.Contains(t, body, `data-dock-redirect="/terminals/`)
	require.Equal(t, spawnArgs{workspaceID: workspaceID, name: "fix bug", cwd: "/wt"}, pty.spawned[0])
}

func TestOpenTaskFailsWithTheBoardLinkWhenTheTaskIsMissing(t *testing.T) {
	router := newRouter(t, &fakePty{}, nothingFound, noTask)

	status, body := send(t, router, http.MethodPost, "/terminal/tasks/"+uuid.New().String()+"/open")

	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "la tarea no existe o no tiene worktree")
	require.Contains(t, body, `href="/"`)
}

func TestOpenTaskRejectsAMalformedID(t *testing.T) {
	router := newRouter(t, &fakePty{}, nothingFound, noTask)

	_, body := send(t, router, http.MethodPost, "/terminal/tasks/nope/open")

	require.Contains(t, body, "id de tarea inválido")
}

func TestListRendersTheSessionsWithTheirRows(t *testing.T) {
	pty := &fakePty{}
	router := newRouter(t, pty, nothingFound, noTask)
	_, _ = send(t, router, http.MethodPost, "/terminal/home")

	status, body := send(t, router, http.MethodGet, "/terminal/sessions")

	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `id="terminal-sessions"`)
	require.Contains(t, body, "<table")
	require.Contains(t, body, "Home")
	require.Contains(t, body, "02/01/2026 03:04 UTC")
	require.Contains(t, body, pty.sessions[0].ID.String()[:8])
}

func TestListWithoutSessionsSaysSo(t *testing.T) {
	router := newRouter(t, &fakePty{}, nothingFound, noTask)

	_, body := send(t, router, http.MethodGet, "/terminal/sessions")

	require.Contains(t, body, "No open terminals.")
	require.NotContains(t, body, "<table")
}

func TestCloseFromListRemovesTheSessionAndAnswersTheTable(t *testing.T) {
	pty := &fakePty{}
	router := newRouter(t, pty, nothingFound, noTask)
	_, _ = send(t, router, http.MethodPost, "/terminal/home")
	id := pty.sessions[0].ID.String()

	status, body := send(t, router, http.MethodPost, "/terminal/sessions/"+id+"/close")

	require.Equal(t, http.StatusOK, status)
	require.Empty(t, pty.sessions)
	require.Contains(t, body, "No open terminals.")
}

func TestCloseFromListShowsTheErrorAboveTheRefreshedTable(t *testing.T) {
	router := newRouter(t, &fakePty{}, nothingFound, noTask)

	status, body := send(t, router, http.MethodPost, "/terminal/sessions/"+uuid.New().String()+"/close")

	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "la sesión de terminal no existe")
	require.Contains(t, body, `id="terminal-sessions"`, "the list is refreshed even when the close failed")
}

func TestCloseFromListRejectsAMalformedID(t *testing.T) {
	router := newRouter(t, &fakePty{}, nothingFound, noTask)

	_, body := send(t, router, http.MethodPost, "/terminal/sessions/nope/close")

	require.Contains(t, body, "id de sesión inválido")
}

func TestCloseFromThePageRedirectsTheTabToTheList(t *testing.T) {
	pty := &fakePty{}
	router := newRouter(t, pty, nothingFound, noTask)
	_, _ = send(t, router, http.MethodPost, "/terminal/home")
	id := pty.sessions[0].ID.String()

	status, body := send(t, router, http.MethodPost, "/terminal/sessions/"+id+"/close/page")

	require.Equal(t, http.StatusOK, status)
	require.Equal(t, `<span data-dock-redirect="/terminals"></span>`, body)
	require.Empty(t, pty.sessions)
}

func TestCloseFromThePageShowsTheErrorWithoutRedirecting(t *testing.T) {
	router := newRouter(t, &fakePty{}, nothingFound, noTask)

	_, body := send(t, router, http.MethodPost, "/terminal/sessions/"+uuid.New().String()+"/close/page")

	require.Contains(t, body, "la sesión de terminal no existe")
	require.NotContains(t, body, "data-dock-redirect")
}
