package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
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
	application, err := app.New(context.Background(), app.Deps{
		DB:        dbtest.New(t),
		Config:    app.NewTestConfig(t),
		Logger:    logger,
		StaticDir: static,
		Listen:    listen.Info{Addr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 3000)},
		Restart:   restart.NewHandle(),
		Version:   "9.8.7",
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

func TestShellShowsTheBuildVersionInTheSidebar(t *testing.T) {
	rec := get(t, newRouter(t), "/")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `data-app-version>v9.8.7<`)
}

func TestShellShowsTheVersionNextToTheBrandAtTheTopOfTheSidebar(t *testing.T) {
	body := get(t, newRouter(t), "/").Body.String()

	brand := strings.Index(body, `tracking-tight">iworkflow`)
	version := strings.Index(body, `data-app-version`)
	menu := strings.Index(body, `aria-label="Navegación"`)
	require.True(t, brand > 0 && version > 0 && menu > 0, "brand, version and menu must be rendered")
	require.Less(t, brand, version, "the version follows the brand")
	require.Less(t, version, menu, "the version sits in the header, above the menu")
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
	require.Contains(t, body, `href="/terminals"`)
	require.Contains(t, body, `href="/settings"`)

	menuIdx := strings.Index(body, `aria-label="Navegación"`)
	boardSidebarIdx := strings.Index(body, `id="board-sidebar"`)
	require.True(t, menuIdx > 0 && boardSidebarIdx > 0, "both menu and board-sidebar must be rendered")
	require.Less(t, menuIdx, boardSidebarIdx, "the menu section must precede the workspaces board sidebar")
}

func TestDeepLinkServesTheShellForAnyPage(t *testing.T) {
	rec := get(t, newRouter(t), "/workspaces/some-id/edit")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `id="dock"`)
}

func TestMigratedTabsRenderThroughTheDockPane(t *testing.T) {
	handler := newRouter(t)

	for _, path := range []string{"/workspaces", "/agents-files", "/terminals"} {
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
	application, err := app.New(context.Background(), app.Deps{
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
	rec := get(t, newRouter(t), "/dock/pane?path=/tasks/some-id/extra/tab")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "todavía no se ha migrado")
}

func TestJiraTabRendersTheConnectionCard(t *testing.T) {
	rec := get(t, newRouter(t), "/dock/pane?path=/jira")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Conexión con Jira")
}

func TestSettingsSectionsAreRoutable(t *testing.T) {
	handler := newRouter(t)

	for _, section := range []string{"language", "resources", "network", "jira", "gitlab", "github"} {
		rec := get(t, handler, "/dock/pane?path=/settings/"+section)
		require.Equal(t, http.StatusOK, rec.Code, section)
	}
	// GitLab, GitHub and Jira are migrated: the settings screen renders their provider cards.
	settings := get(t, handler, "/dock/pane?path=/settings")
	require.Contains(t, settings.Body.String(), `id="jira-connection-card"`)
	require.Contains(t, settings.Body.String(), `id="code-host-settings-gitlab"`)
	require.Contains(t, settings.Body.String(), `id="code-host-settings-github"`)
}

func TestTheShellOpensOneTaskStreamAndTheBoardTabOpensNone(t *testing.T) {
	handler := newRouter(t)

	shell := get(t, handler, "/")
	require.Equal(t, http.StatusOK, shell.Code)
	require.Equal(t, 1, strings.Count(shell.Body.String(), `sse-connect="/api/tasks/events"`), "the sidebar stream serves the whole page")
	require.Contains(t, shell.Body.String(), `data-dock-path="/jira"`, "the sidebar menu includes Jira")

	board := get(t, handler, "/dock/pane?path=/")
	require.Equal(t, http.StatusOK, board.Code)
	require.NotContains(t, board.Body.String(), `sse-connect=`, "a second stream on the board doubles every refresh")
}

func TestEveryNotMigratedPathResolvesAndTheHomeIsTheDashboard(t *testing.T) {
	handler := newRouter(t)

	for _, path := range []string{"/tasks/some-id/extra/tab"} {
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
	require.Contains(t, home.Body.String(), `id="dashboard-`, "the home address is the dashboard")
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

func TestTranscriptionsEndpointIsMountedAndReportsAMissingModel(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/transcriptions", strings.NewReader("not a wav"))
	rec := httptest.NewRecorder()

	newRouter(t).ServeHTTP(rec, request)

	// The body is rejected before the (absent) model is looked for: the route exists and answers JSON.
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestTheBoardSidebarIsRenderedInsideTheMorphExtensionScope(t *testing.T) {
	handler := newRouter(t)

	body := get(t, handler, "/").Body.String()

	// hx-ext is inherited by descendants: the sidebar swaps with morph:innerHTML, which only works under it.
	extension := strings.Index(body, `hx-ext="sse,morph"`)
	sidebar := strings.Index(body, `id="board-sidebar"`)
	require.True(t, extension >= 0 && sidebar > extension, "the board sidebar must sit under an element that loads the morph extension")
	// The swap must be on the sidebar element itself: the board list carries the same attribute, so a page-wide
	// check would pass even if the sidebar stopped using it.
	opening := body[sidebar:]
	opening = opening[:strings.Index(opening, ">")]
	require.Contains(t, opening, `hx-swap="morph:innerHTML"`)
}

func TestTheDataHomeCardRendersBeforeTheResourceLimits(t *testing.T) {
	rec := get(t, newRouter(t), "/dock/pane?path=/settings/resources")

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	home := strings.Index(body, `id="data-home-card"`)
	limits := strings.Index(body, `id="agent-settings-limits"`)
	require.NotEqual(t, -1, home, "the data folder card is missing from the resources section")
	require.NotEqual(t, -1, limits, "the resource limits card is missing from the resources section")
	require.Less(t, home, limits, "the data folder card must come before the resource limits card")
}

func TestTheDataHomeCardRouteIsMounted(t *testing.T) {
	rec := get(t, newRouter(t), "/data-home/card")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `id="data-home-card"`)
}

// The dashboard refreshes through its own address, and its menu entry comes before Workspaces.
func TestTheDashboardRefreshAddressAnswersAndLeadsTheMenu(t *testing.T) {
	handler := newRouter(t)

	rec := get(t, handler, "/board/dashboard?scope=abc123")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `class="page-title"`)

	page := get(t, handler, "/").Body.String()
	home := strings.Index(page, `class="sidebar-nav`)
	require.GreaterOrEqual(t, home, 0)
	require.Contains(t, page[home:home+200], `data-dock-path="/"`, "the first menu entry is the dashboard")
	require.Less(t, home, strings.Index(page, `data-dock-path="/workspaces"`), "the dashboard is above Workspaces")
}

// Rule from source: the Edit button of a diff card loads its editor from /tasks/{id}/file. The route must be mounted
// by the tasks module; when it falls through to the not-migrated catch-all, the whole shell is rendered inside the card.
func TestTheFileEditorOfTheDiffAnswersAFragmentNotTheShell(t *testing.T) {
	handler := newRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/tasks/0b6b7a1e-5f5e-4c44-9a4e-2f1d0c9a7b11/file?scope=s1&path=src%2Fa.rs", nil)
	request.Header.Set("HX-Request", "true")
	request.AddCookie(&http.Cookie{Name: i18n.LocaleCookie, Value: "es"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, request)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "<html")
	require.NotContains(t, rec.Body.String(), `id="dock`)
}

// Rule from source: closing and saving the editor are served by the same module routes as loading it; each answers a
// fragment through the composed router, never the shell of the not-migrated catch-all.
func TestTheOtherFileEditorRoutesOfTheDiffAnswerFragmentsNotTheShell(t *testing.T) {
	const task = "/tasks/0b6b7a1e-5f5e-4c44-9a4e-2f1d0c9a7b11"
	cases := []struct {
		name   string
		method string
		target string
	}{
		{"close", http.MethodGet, task + "/file/close?scope=s1&path=src%2Fa.rs"},
		{"state", http.MethodPost, task + "/file/state"},
		{"save", http.MethodPost, task + "/file"},
	}
	handler := newRouter(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.target, strings.NewReader("scope=s1&path=src%2Fa.rs&content=x&base=b&eol=lf"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("HX-Request", "true")
			request.AddCookie(&http.Cookie{Name: i18n.LocaleCookie, Value: "es"})
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, request)

			require.Equal(t, http.StatusOK, rec.Code)
			require.NotContains(t, rec.Body.String(), "<html")
			require.NotContains(t, rec.Body.String(), `id="dock`)
		})
	}
}

func TestShellHasNoCommandPaletteAndKeepsTheSidebarHelpers(t *testing.T) {
	rec := get(t, newRouter(t), "/")

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.NotContains(t, body, `id="command-palette"`)
	require.NotContains(t, body, `data-palette-open`)
	require.NotContains(t, body, `/static/js/command_palette.js`)
	require.Contains(t, body, `/static/js/sidebar_active.js`)
}

func TestShellDeclaresTheWebAppManifestAndItsIcons(t *testing.T) {
	body := get(t, newRouter(t), "/").Body.String()

	require.Contains(t, body, `rel="manifest" href="/static/manifest.webmanifest"`)
	require.Contains(t, body, `name="theme-color"`)
	require.Contains(t, body, `rel="apple-touch-icon" href="/static/icons/icon-192.png"`)
	require.Contains(t, body, `rel="icon" type="image/svg+xml" href="/static/icons/icon.svg"`)
}

// engramTaskID is the task the Engram app tests open; insertWorkspaceAndTask stores it in workspace "demo".
const engramTaskID = "0b6b7a1e-5f5e-4c44-9a4e-2f1d0c9a7b11"

// engramWorkspaceID is the UUID of workspace "demo"; the repository rejects other IDs.
const engramWorkspaceID = "6f1c3d2a-8b7e-4a51-9c20-3e4f5a6b7c8d"

// composeWithEngram composes the server on db with the given IWF_ENGRAM_URL value and returns its router and the
// log it wrote.
func composeWithEngram(t *testing.T, db *sql.DB, engramURL string) (http.Handler, *bytes.Buffer) {
	t.Helper()
	static := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(static, "app.css"), []byte("body{}"), 0o600))
	var logs bytes.Buffer
	application, err := app.New(context.Background(), app.Deps{
		DB:        db,
		Config:    app.NewTestConfig(t),
		Logger:    slog.New(slog.NewTextHandler(&logs, nil)),
		StaticDir: static,
		Listen:    listen.Info{Addr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 3000)},
		Restart:   restart.NewHandle(),
		Version:   "9.8.7",
		EngramURL: engramURL,
	})
	require.NoError(t, err)
	return application.Router(), &logs
}

// insertWorkspaceAndTask stores workspace "demo" at /repos/demo and one backlog task of it.
func insertWorkspaceAndTask(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES (?, 'demo', '/repos/demo', 'main', '2026-01-01T00:00:00Z')`, engramWorkspaceID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES (?, ?, 'memoria', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, engramTaskID, engramWorkspaceID)
	require.NoError(t, err)
}

// An unset, empty or invalid IWF_ENGRAM_URL starts the app on the saved address (or Engram's default): only an invalid
// value logs a warning, and only one. The test saves an unreachable address, so it stays hermetic: Workspaces says
// Engram is not available and the task has no Memory panel.
func TestEngramIsUnavailableWhenNothingAnswersAtTheConfiguredAddress(t *testing.T) {
	cases := map[string]struct {
		url      string
		warnings int
	}{
		"unset":     {url: "", warnings: 0},
		"spaces":    {url: "   ", warnings: 0},
		"not a url": {url: "not a url", warnings: 1},
		"ftp":       {url: "ftp://127.0.0.1:7437", warnings: 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			db := dbtest.New(t)
			insertWorkspaceAndTask(t, db)
			_, err := db.Exec(`INSERT INTO app_preferences (key, value) VALUES ('engram_url', 'http://127.0.0.1:1')`)
			require.NoError(t, err)
			handler, logs := composeWithEngram(t, db, tc.url)

			assert.Equal(t, tc.warnings, strings.Count(logs.String(), "level=WARN"), "one warning for an invalid value, none otherwise")

			workspaces := get(t, handler, "/dock/pane?path=/workspaces")
			require.Equal(t, http.StatusOK, workspaces.Code)
			assert.Contains(t, workspaces.Body.String(), "demo", "the workspace is still listed")
			assert.Contains(t, workspaces.Body.String(), "Engram no disponible")

			detail := get(t, handler, "/dock/pane?path=/tasks/"+engramTaskID)
			require.Equal(t, http.StatusOK, detail.Code)
			assert.NotContains(t, detail.Body.String(), "data-task-memory", "no Memory panel on the task")
		})
	}
}

// With a valid URL the feature is on: the Workspaces page asks Engram for its projects once, and the task shows
// its Memory panel. This is the control for the disabled cases above.
func TestEngramFeatureIsOnWithAValidURLAndReadsTheProjectsOnce(t *testing.T) {
	var projectsHits, recentHits atomic.Int32
	engram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			projectsHits.Add(1)
			_, _ = w.Write([]byte(`{"projects":[{"name":"demo","observation_count":2,"directories":["/repos/demo"]}]}`))
		case "/observations/recent":
			recentHits.Add(1)
			_, _ = w.Write([]byte(`[{"id":1,"type":"decision","title":"Usar WAL","last_seen_at":"2026-11-10 09:00:00"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(engram.Close)
	db := dbtest.New(t)
	insertWorkspaceAndTask(t, db)
	handler, logs := composeWithEngram(t, db, engram.URL)

	workspaces := get(t, handler, "/dock/pane?path=/workspaces")
	require.Equal(t, http.StatusOK, workspaces.Code)
	assert.Contains(t, workspaces.Body.String(), "Engram · 2 memorias")
	assert.Equal(t, int32(1), projectsHits.Load(), "one GET /projects for the page, however many rows")
	assert.Equal(t, int32(0), recentHits.Load(), "the chips read no recent observations")
	assert.Zero(t, strings.Count(logs.String(), "level=WARN"))

	detail := get(t, handler, "/dock/pane?path=/tasks/"+engramTaskID)
	require.Equal(t, http.StatusOK, detail.Code)
	assert.Contains(t, detail.Body.String(), "data-task-memory", "the Memory panel is shown")
	assert.Contains(t, detail.Body.String(), "Usar WAL")
}
