package inbound_test

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/task_hooks/application"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/system"
	"iworkflow/internal/modules/task_hooks/infrastructure/di"
)

// staticProjects lists the two projects the tests seed.
type staticProjects struct{}

func (staticProjects) Projects(context.Context) ([]application.ProjectOption, error) {
	return []application.ProjectOption{{ID: "ws-1", Name: "alpha"}, {ID: "ws-2", Name: "beta"}}, nil
}

// dirLookup gives every workspace the same directory, where a test of a hook runs.
type dirLookup struct{ dir string }

func (d dirLookup) Path(context.Context, string) (string, error) { return d.dir, nil }

// fixture is the Hooks screen composed over a temporary database, served by the routes of the module.
type fixture struct {
	t      *testing.T
	db     *sql.DB
	module *di.Module
	router chi.Router
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := dbtest.New(t)
	for _, ws := range []struct{ id, name string }{{"ws-1", "alpha"}, {"ws-2", "beta"}} {
		_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES (?, ?, ?, 'main', '2026-01-01T00:00:00Z')`,
			ws.id, ws.name, t.TempDir())
		require.NoError(t, err)
	}
	module := di.NewModule(di.Deps{
		DB:         db,
		Clock:      system.NewClock(),
		Workspaces: dirLookup{dir: t.TempDir()},
		Projects:   staticProjects{},
	})
	router := chi.NewRouter()
	module.Routes(router)
	return &fixture{t: t, db: db, module: module, router: router}
}

// send posts or gets the path with the form, in English, the way the screen does.
func (f *fixture) send(method string, path string, form url.Values) *httptest.ResponseRecorder {
	f.t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	request = request.WithContext(i18n.WithState(request.Context(), i18n.State{Locale: i18n.En, Chosen: true}))
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	return recorder
}

// saveScript creates a script hook with the name and the command, and returns its id.
func (f *fixture) saveScript(name string, command string) string {
	f.t.Helper()
	rec := f.send(http.MethodPost, "/hooks", url.Values{
		dto.FieldName: {name}, dto.FieldKind: {"script"}, dto.FieldTimeout: {"30"}, dto.FieldCommand: {command},
	})
	require.Equal(f.t, http.StatusOK, rec.Code)
	var id string
	require.NoError(f.t, f.db.QueryRow(`SELECT id FROM hooks WHERE name = ?`, name).Scan(&id))
	return id
}

// assign adds the hook to a project, event and moment, and returns the answer of the screen.
func (f *fixture) assign(hookID string, project string, event string, phase string) *httptest.ResponseRecorder {
	return f.send(http.MethodPost, "/hooks/"+hookID+"/assignments", url.Values{
		dto.FieldProject: {project}, dto.FieldEvent: {event}, dto.FieldPhase: {phase},
		dto.FieldOnFailure: {"block"}, dto.FieldEnabled: {"on"},
	})
}

// assignmentID returns the id of the one assignment of the hook for the project, event and moment.
func (f *fixture) assignmentID(hookID string, project string, event string, phase string) string {
	f.t.Helper()
	var id string
	require.NoError(f.t, f.db.QueryRow(`SELECT id FROM hook_assignments WHERE hook_id = ? AND workspace_id = ? AND event = ? AND phase = ?`,
		hookID, project, event, phase).Scan(&id))
	return id
}

// seedRun stores one run of the hook for a task of the first workspace.
func (f *fixture) seedRun(id string, hookID string, status string, output string, started string) {
	f.t.Helper()
	_, err := f.db.Exec(`INSERT OR IGNORE INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES ('t1', 'ws-1', 't', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(f.t, err)
	_, err = f.db.Exec(`INSERT INTO task_hook_runs (id, task_id, hook_id, assignment_id, hook_name, workspace_id, phase, event, status, exit_code, output, started_at, finished_at)
		VALUES (?, 't1', ?, 'a', 'lint', 'ws-1', 'pre', 'task_start', ?, 2, ?, ?, ?)`, id, hookID, status, output, started, started)
	require.NoError(f.t, err)
}

// The library lists the hooks with their assignments and their latest error; names are escaped.
func TestLibraryListsTheHooksWithTheirLastError(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("<b>lint</b>", "make")
	f.seedRun("r-old", hookID, "failed", "old", "2026-01-01T09:00:00Z")
	f.seedRun("r-new", hookID, "timeout", "slow", "2026-01-02T09:00:00Z")

	rec := f.send(http.MethodGet, "/hooks/library", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `data-hook="`+hookID+`"`)
	assert.Contains(t, body, "&lt;b&gt;lint&lt;/b&gt;", "the name is escaped")
	assert.NotContains(t, body, "<b>lint</b>")
	assert.Contains(t, body, "Timed out · 2026-01-02 09:00", "the latest failure is the timeout, the newest one")
	assert.Contains(t, body, "hook="+hookID, "the error opens the history of the hook")
}

// A new hook needs a name: the screen stays open with the values as typed and the reason.
func TestSaveWithoutANameKeepsTheEditorOpen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	rec := f.send(http.MethodPost, "/hooks", url.Values{dto.FieldKind: {"script"}, dto.FieldTimeout: {"30"}, dto.FieldCommand: {"make"}})

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "The action failed:")
	assert.Contains(t, body, "the name is required")
	assert.Contains(t, body, `data-hook-editor`, "the editor stays open")
	assert.Contains(t, body, `value="make"`, "the command is kept as typed")
}

// A malformed form is the only bad request; every other refusal is answered with the screen.
func TestMalformedFormIsTheOnlyBadRequest(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	request := httptest.NewRequest(http.MethodPost, "/hooks", strings.NewReader("name=%zz"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	f.router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// The same hook cannot run twice for one project, event and moment: the second assignment is refused on the screen,
// which keeps the hook open.
func TestAssignmentRefusesTheSameGroupTwice(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("lint", "make")

	first := f.assign(hookID, "ws-1", "task_start", "pre")
	require.Equal(t, http.StatusOK, first.Code)
	assert.Contains(t, first.Body.String(), "Assignment added.")

	second := f.assign(hookID, "ws-1", "task_start", "pre")

	require.Equal(t, http.StatusOK, second.Code)
	body := second.Body.String()
	assert.Contains(t, body, "The action failed:")
	assert.Contains(t, body, "invalid hook")
	assert.Contains(t, body, `data-hook-assignments="`+hookID+`"`, "the assignments of the hook stay open")
}

// A toggle posts the stored values; without the enabled flag the assignment is disabled, and a delete detaches it while
// the hook stays in the library.
func TestToggleAndDeleteAnAssignment(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("lint", "make")
	require.Equal(t, http.StatusOK, f.assign(hookID, "ws-1", "task_start", "pre").Code)
	assignmentID := f.assignmentID(hookID, "ws-1", "task_start", "pre")

	toggled := f.send(http.MethodPost, "/hooks/"+hookID+"/assignments/"+assignmentID, url.Values{
		dto.FieldID: {assignmentID}, dto.FieldProject: {"ws-1"}, dto.FieldEvent: {"task_start"}, dto.FieldPhase: {"pre"},
		dto.FieldOnFailure: {"block"},
	})

	require.Equal(t, http.StatusOK, toggled.Code)
	assert.Contains(t, toggled.Body.String(), "Assignment updated.")
	assert.Contains(t, toggled.Body.String(), `data-assignment="`+assignmentID+`"`)
	assert.Contains(t, toggled.Body.String(), ">Enable<", "the assignment is off and offers to enable it")

	deleted := f.send(http.MethodPost, "/hooks/"+hookID+"/assignments/"+assignmentID+"/delete", nil)

	require.Equal(t, http.StatusOK, deleted.Code)
	assert.Contains(t, deleted.Body.String(), "Assignment removed.")
	assert.NotContains(t, deleted.Body.String(), `data-assignment="`+assignmentID+`"`)
	assert.Contains(t, f.send(http.MethodGet, "/hooks/library", nil).Body.String(), `data-hook="`+hookID+`"`, "the hook stays")
}

// A test that cannot start the hook shows the run as an error in its result, with the reason.
func TestTestShowsARunThatCannotStart(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("missing", "/does/not/exist/hook")

	rec := f.send(http.MethodPost, "/hooks/"+hookID+"/test", url.Values{
		dto.FieldProject: {"ws-1"}, dto.FieldEvent: {"task_start"}, dto.FieldPhase: {"pre"},
	})

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `data-hook-test="error"`)
	assert.Contains(t, body, "Could not run")
	assert.Contains(t, body, "could not start the hook")
}

// The history shows only the failures until the filter is applied with the failed box unchecked.
func TestHistoryIsFailedOnlyUntilAFilterIsApplied(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("lint", "make")
	f.seedRun("r-failed", hookID, "failed", "boom", "2026-01-01T09:00:00Z")
	f.seedRun("r-ok", hookID, "ok", "fine", "2026-01-01T10:00:00Z")

	defaults := f.send(http.MethodGet, "/hooks/history", nil)

	require.Equal(t, http.StatusOK, defaults.Code)
	assert.Contains(t, defaults.Body.String(), `data-run="r-failed"`)
	assert.NotContains(t, defaults.Body.String(), `data-run="r-ok"`)
	assert.Contains(t, defaults.Body.String(), "checked", "the failed-only box is on")

	all := f.send(http.MethodGet, "/hooks/history?applied=1&hook="+hookID+"&project=", nil)

	require.Equal(t, http.StatusOK, all.Code)
	assert.Contains(t, all.Body.String(), `data-run="r-ok"`, "an applied filter with the box unchecked shows every run")
	assert.Contains(t, all.Body.String(), "boom", "the output of a failure is kept and shown")
}

// The order must name each assignment of the group: a partial order is refused and nothing changes.
func TestReorderNeedsEveryAssignmentOfTheGroup(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	first := f.saveScript("first", "make")
	second := f.saveScript("second", "make")
	require.Equal(t, http.StatusOK, f.assign(first, "ws-1", "task_start", "pre").Code)
	require.Equal(t, http.StatusOK, f.assign(second, "ws-1", "task_start", "pre").Code)

	rec := f.send(http.MethodPost, "/hooks/order", url.Values{
		dto.FieldProject: {"ws-1"}, dto.FieldEvent: {"task_start"}, dto.FieldPhase: {"pre"},
		dto.FieldOrder: {f.assignmentID(first, "ws-1", "task_start", "pre")},
	})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "must name the 2 assignments of the group")
	assert.Contains(t, rec.Body.String(), `data-group-row=`, "the group is still shown")
}

// Deleting a hook removes it and its assignments from the library.
func TestDeleteRemovesTheHookFromTheLibrary(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	hookID := f.saveScript("doomed", "make")

	rec := f.send(http.MethodPost, "/hooks/"+hookID+"/delete", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Hook deleted.")
	assert.NotContains(t, rec.Body.String(), `data-hook="`+hookID+`"`)
}

// Switching the kind keeps what was typed in the editor, and the fields of the new kind show.
func TestEditorKindSwitchKeepsWhatWasTyped(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	rec := f.send(http.MethodGet, "/hooks/editor?kind=agent&name=review&prompt=check+the+diff&timeout=60", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `value="review"`)
	assert.Contains(t, body, "check the diff", "the prompt is kept")
	assert.NotContains(t, body, `name="command"`, "a script field is not shown for an agent")
}

// The dock tab renders the same screen as the htmx answers, with the library in it.
func TestDockTabRendersTheScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	require.Len(t, f.module.Tabs(), 1)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})

	var out bytes.Buffer
	require.NoError(t, f.module.Tabs()[0].Render(ctx, dock.Params{}).Render(ctx, &out))

	assert.Contains(t, out.String(), "data-hooks-screen")
	assert.Equal(t, "/hooks", f.module.Tabs()[0].Pattern)
	assert.True(t, regexp.MustCompile(`data-hooks-library`).MatchString(out.String()))
}
