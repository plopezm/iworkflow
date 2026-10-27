package app_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/app"
	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/listen"
	"iworkflow/internal/core/restart"
)

// newRouter builds the composed server on a migrated temporary database. Access is not enforced
// (the token is empty and the listen info is loopback), so the tests reach every route.
func newRouter(t *testing.T) http.Handler {
	t.Helper()
	static := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(static, "app.css"), []byte("body{}"), 0o600))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := app.New(app.Deps{
		DB:        dbtest.New(t),
		Config:    app.NewTestConfig(t),
		Logger:    logger,
		StaticDir: static,
		Listen:    listen.Info{Addr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 3000)},
		Restart:   restart.NewHandle(),
	})
	require.NoError(t, err)
	return application.Router()
}

func get(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.AddCookie(&http.Cookie{Name: i18n.LocaleCookie, Value: "es"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request)
	return rec
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

func TestShellRendersDockAndMigratedNavigation(t *testing.T) {
	rec := get(t, newRouter(t), "/")

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `id="dock"`)
	require.Contains(t, body, `id="toasts"`)
	require.Contains(t, body, `aria-label="Navegación"`)
	require.Contains(t, body, `href="/workspaces"`)
	require.Contains(t, body, `href="/agents-files"`)
	require.Contains(t, body, `href="/profiles"`)
	require.Contains(t, body, `href="/models"`)
	require.Contains(t, body, `href="/terminals"`)
	require.Contains(t, body, `href="/settings"`)
}

func TestDeepLinkServesTheShellForAnyPage(t *testing.T) {
	rec := get(t, newRouter(t), "/workspaces/some-id/edit")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `id="dock"`)
}

func TestMigratedTabsRenderThroughTheDockPane(t *testing.T) {
	handler := newRouter(t)

	for _, path := range []string{"/workspaces", "/agents-files", "/profiles", "/models", "/terminals"} {
		rec := get(t, handler, "/dock/pane?path="+path)
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.NotContains(t, rec.Body.String(), "todavía no se ha migrado", path)
	}
	// The settings screen still shows the placeholders of the sections that are not migrated.
	settings := get(t, handler, "/dock/pane?path=/settings")
	require.Equal(t, http.StatusOK, settings.Code)
	require.Contains(t, settings.Body.String(), "Límites de recursos")
}

func TestShutdownReleasesTheModules(t *testing.T) {
	application, err := app.New(app.Deps{
		DB:        dbtest.New(t),
		Config:    app.NewTestConfig(t),
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		StaticDir: t.TempDir(),
		Listen:    listen.Info{Addr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 3000)},
		Restart:   restart.NewHandle(),
	})
	require.NoError(t, err)

	require.NoError(t, application.Shutdown(context.Background()))
}

func TestUnmigratedTabsExplainTheirState(t *testing.T) {
	rec := get(t, newRouter(t), "/dock/pane?path=/jira")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "todavía no se ha migrado")
}

func TestSettingsSectionsAreRoutable(t *testing.T) {
	handler := newRouter(t)

	for _, section := range []string{"language", "resources", "network", "jira", "gitlab", "github"} {
		rec := get(t, handler, "/dock/pane?path=/settings/"+section)
		require.Equal(t, http.StatusOK, rec.Code, section)
	}
	// GitLab and GitHub are migrated: the settings screen renders their provider cards.
	settings := get(t, handler, "/dock/pane?path=/settings")
	require.Contains(t, settings.Body.String(), `id="code-host-settings-gitlab"`)
	require.Contains(t, settings.Body.String(), `id="code-host-settings-github"`)
}

func TestTheShellOpensOneTaskStreamAndTheBoardTabOpensNone(t *testing.T) {
	handler := newRouter(t)

	shell := get(t, handler, "/")
	require.Equal(t, http.StatusOK, shell.Code)
	require.Equal(t, 1, strings.Count(shell.Body.String(), `sse-connect="/api/tasks/events"`), "the sidebar stream serves the whole page")

	board := get(t, handler, "/dock/pane?path=/")
	require.Equal(t, http.StatusOK, board.Code)
	require.NotContains(t, board.Body.String(), `sse-connect=`, "a second stream on the board doubles every refresh")
}

func TestEveryNotMigratedPathResolvesAndTheHomeIsTheBoard(t *testing.T) {
	handler := newRouter(t)

	for _, path := range []string{"/jira", "/jira/some-id"} {
		rec := get(t, handler, "/dock/pane?path="+path)
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.Contains(t, rec.Body.String(), "todavía no se ha migrado", "%s keeps its not-migrated explanation", path)
	}
	// The subagents panel of a task is migrated: an unknown task answers its error, not the explanation.
	panel := get(t, handler, "/tasks/0b6b7a1e-5f5e-4c44-9a4e-2f1d0c9a7b11/subagents?scope=abc123")
	require.Equal(t, http.StatusOK, panel.Code)
	require.NotContains(t, panel.Body.String(), "todavía no se ha migrado")
	require.Contains(t, panel.Body.String(), "text-error")
	// The detail of a task is migrated; an unknown task says so in place of the not-migrated explanation.
	detail := get(t, handler, "/dock/pane?path=/tasks/0b6b7a1e-5f5e-4c44-9a4e-2f1d0c9a7b11")
	require.Equal(t, http.StatusOK, detail.Code)
	require.NotContains(t, detail.Body.String(), "todavía no se ha migrado")
	require.Contains(t, detail.Body.String(), "ya no existe")
	home := get(t, handler, "/dock/pane?path=/")
	require.Equal(t, http.StatusOK, home.Code)
	require.Contains(t, home.Body.String(), `id="board-list-`, "the home address is the board")
	require.NotContains(t, home.Body.String(), "todavía no se ha migrado")
}

// Rule from source: the Files tab of a task shows the file tree of the project tree, so the composition root must
// hand the project views to the tasks module; without them the tab answers that the files are not available.
func TestTheFilesTabOfATaskEmbedsTheProjectTree(t *testing.T) {
	handler := newRouter(t)

	rec := get(t, handler, "/dock/pane?path=/tasks/0b6b7a1e-5f5e-4c44-9a4e-2f1d0c9a7b11/files")

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "no están disponibles")
}
