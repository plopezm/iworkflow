package inbound_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
)

const workspace = "w1"

// memHooks keeps the hooks in memory, in position order, with the semantics of the storage the use cases rely on.
type memHooks struct{ hooks []models.Hook }

func (m *memHooks) List(_ context.Context, workspaceID string) ([]models.Hook, error) {
	var out []models.Hook
	for _, h := range m.hooks {
		if h.WorkspaceID() == workspaceID {
			out = append(out, h)
		}
	}
	return out, nil
}

func (m *memHooks) ListFor(_ context.Context, _ string, event models.Event, phase models.Phase) ([]models.Hook, error) {
	var out []models.Hook
	for _, h := range m.hooks {
		if h.Event() == event && h.Phase() == phase {
			out = append(out, h)
		}
	}
	return out, nil
}

func (m *memHooks) Get(_ context.Context, id string) (models.Hook, error) {
	for _, h := range m.hooks {
		if h.ID() == id {
			return h, nil
		}
	}
	return models.Hook{}, hookerrors.ErrHookNotFound
}

func (m *memHooks) Save(_ context.Context, hook models.Hook) error {
	for i, h := range m.hooks {
		if h.ID() == hook.ID() {
			m.hooks[i] = hook
			return nil
		}
	}
	m.hooks = append(m.hooks, hook)
	return nil
}

func (m *memHooks) Delete(_ context.Context, id string) error {
	for i, h := range m.hooks {
		if h.ID() == id {
			m.hooks = append(m.hooks[:i], m.hooks[i+1:]...)
			return nil
		}
	}
	return hookerrors.ErrHookNotFound
}

func (m *memHooks) Reorder(_ context.Context, workspaceID string, ids []string) error {
	byID := map[string]models.Hook{}
	var others []models.Hook
	for _, h := range m.hooks {
		if h.WorkspaceID() == workspaceID {
			byID[h.ID()] = h
		} else {
			others = append(others, h)
		}
	}
	var ordered []models.Hook
	for _, id := range ids {
		ordered = append(ordered, byID[id])
	}
	m.hooks = append(others, ordered...)
	return nil
}

func (m *memHooks) order(workspaceID string) []string {
	var ids []string
	for _, h := range m.hooks {
		if h.WorkspaceID() == workspaceID {
			ids = append(ids, h.ID())
		}
	}
	return ids
}

// scripts answers every script run with a fixed outcome and counts the runs.
type scripts struct {
	outcome models.RunOutcome
	err     error
	runs    int
}

func (s *scripts) Run(context.Context, models.Hook, models.RunContext) (models.RunOutcome, error) {
	s.runs++
	return s.outcome, s.err
}

type fixedWorkspace struct{}

func (fixedWorkspace) Path(context.Context, string) (string, error) { return "/work/w1", nil }

type sequentialIDs struct{ n int }

func (s *sequentialIDs) NewID() string { s.n++; return fmt.Sprintf("new-%d", s.n) }

type rig struct {
	router http.Handler
	hooks  *memHooks
	runner *scripts
}

// newRig mounts the handlers over the real use cases, with the given hooks stored.
func newRig(t *testing.T, stored ...models.Hook) rig {
	t.Helper()
	hooks := &memHooks{hooks: stored}
	runner := &scripts{outcome: models.RunOutcome{Status: models.RunOK, ExitCode: 0, Output: "hello\n"}}
	ids := &sequentialIDs{}
	handlers := inbound.NewHandlers(inbound.UseCases{
		List:       application.NewListHooks(hooks),
		Save:       application.NewSaveHook(hooks, ids),
		Delete:     application.NewDeleteHook(hooks),
		Reorder:    application.NewReorderHooks(hooks),
		SetEnabled: application.NewSetHookEnabled(hooks),
		Test:       application.NewTestHook(hooks, fixedWorkspace{}, runner, &scripts{}),
	})
	r := chi.NewRouter()
	handlers.Mount(r)
	return rig{router: r, hooks: hooks, runner: runner}
}

func hook(t *testing.T, id string, phase models.Phase, event models.Event, command string) models.Hook {
	t.Helper()
	h, err := models.NewHook(models.HookParams{
		ID: id, WorkspaceID: workspace, Phase: phase, Event: event, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureBlock, Timeout: time.Minute,
		Script: models.ScriptSpec{Command: command},
	})
	require.NoError(t, err)
	return h
}

func do(t *testing.T, h http.Handler, method string, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	body := ""
	if form != nil {
		body = form.Encode()
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func scriptForm(command string) url.Values {
	return url.Values{
		dto.FieldEvent: {"task_start"}, dto.FieldPhase: {"pre"}, dto.FieldKind: {"script"},
		dto.FieldEnabled: {"on"}, dto.FieldOnFailure: {"warn"}, dto.FieldTimeout: {"90"},
		dto.FieldCommand: {command}, dto.FieldArgs: {"--fast"}, dto.FieldEnv: {"MODE=ci"},
	}
}

func panelPath(suffix string) string { return "/workspaces/" + workspace + "/hooks" + suffix }

// The save of a valid form creates the hook in its group and answers the tab with the notice; nothing else changes.
func TestSaveCreatesTheHookAndAnswersTheTab(t *testing.T) {
	t.Parallel()
	r := newRig(t)

	rec := do(t, r.router, http.MethodPost, panelPath(""), scriptForm("./setup.sh"))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "Hook saved.")
	assert.Contains(t, rec.Body.String(), `data-hook="new-1"`)
	require.Len(t, r.hooks.hooks, 1)
	stored := r.hooks.hooks[0]
	assert.Equal(t, models.PhasePre, stored.Phase())
	assert.Equal(t, models.FailureWarn, stored.OnFailure())
	assert.Equal(t, 90*time.Second, stored.Timeout())
	assert.Equal(t, []string{"--fast"}, stored.Script().Args)
}

// A rejected form is answered inside the tab with status 200: the message is shown, the editor stays with the values
// as typed, and nothing is stored.
func TestSaveRejectedKeepsTheEditorAndTheTypedValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(url.Values)
		message string
	}{
		{name: "empty command", mutate: func(f url.Values) { f[dto.FieldCommand] = []string{""} }, message: "the command is required"},
		{name: "timeout is not a number", mutate: func(f url.Values) { f[dto.FieldTimeout] = []string{"soon"} }, message: "whole number of seconds"},
		{name: "variable without equals", mutate: func(f url.Values) { f[dto.FieldEnv] = []string{"NOPE"} }, message: "NAME=VALUE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t)
			form := scriptForm("./typed.sh")
			tt.mutate(form)

			rec := do(t, r.router, http.MethodPost, panelPath(""), form)

			require.Equal(t, http.StatusOK, rec.Code)
			body := rec.Body.String()
			assert.Contains(t, body, tt.message)
			assert.Contains(t, body, `data-hook-editor`, "the editor stays open")
			assert.Empty(t, r.hooks.hooks, "a rejected hook is not stored")
		})
	}
}

func TestSaveWithAMalformedFormIsBadRequest(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	req := httptest.NewRequest(http.MethodPost, panelPath(""), strings.NewReader("command=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	r.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, r.hooks.hooks)
}

// Saving with the id of a stored hook edits it, and the id of another workspace is refused in the tab.
func TestSaveEditsTheHookOfTheWorkspaceOnly(t *testing.T) {
	t.Parallel()
	stored := hook(t, "h1", models.PhasePre, models.EventTaskStart, "old")
	other, err := models.NewHook(models.HookParams{
		ID: "h2", WorkspaceID: "w2", Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureBlock, Timeout: time.Minute, Script: models.ScriptSpec{Command: "theirs"},
	})
	require.NoError(t, err)
	r := newRig(t, stored, other)

	edit := scriptForm("new")
	edit.Set(dto.FieldID, "h1")
	rec := do(t, r.router, http.MethodPost, panelPath(""), edit)
	assert.Contains(t, rec.Body.String(), "Hook updated.")
	assert.Equal(t, "new", r.hooks.hooks[0].Script().Command)

	steal := scriptForm("hijack")
	steal.Set(dto.FieldID, "h2")
	rec = do(t, r.router, http.MethodPost, panelPath(""), steal)
	assert.Contains(t, rec.Body.String(), "belongs to another workspace")
	assert.Equal(t, "theirs", r.hooks.hooks[1].Script().Command)
}

// The editor of a stored hook is filled with its values; a changed kind keeps what was typed.
func TestEditorLoadsTheStoredHookAndKeepsTypedValuesOnKindChange(t *testing.T) {
	t.Parallel()
	r := newRig(t, hook(t, "h1", models.PhasePre, models.EventTaskStart, "make"))

	loaded := do(t, r.router, http.MethodGet, panelPath("/editor?hook=h1"), nil)
	require.Equal(t, http.StatusOK, loaded.Code)
	assert.Contains(t, loaded.Body.String(), `name="id" value="h1"`)
	assert.Contains(t, loaded.Body.String(), `value="make"`)

	changed := do(t, r.router, http.MethodGet, panelPath("/editor?"+url.Values{
		dto.FieldID: {"h1"}, dto.FieldKind: {"agent"}, dto.FieldPrompt: {"keep me"}, dto.FieldTimeout: {"30"},
	}.Encode()), nil)
	assert.Contains(t, changed.Body.String(), `name="prompt"`)
	assert.Contains(t, changed.Body.String(), "keep me")
	assert.NotContains(t, changed.Body.String(), `name="command"`)
}

func TestEditorOfAnUnknownHookShowsTheFailure(t *testing.T) {
	t.Parallel()
	r := newRig(t)

	rec := do(t, r.router, http.MethodGet, panelPath("/editor?hook=missing"), nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "hook not found")
	assert.NotContains(t, rec.Body.String(), "data-hook-editor")
}

// A new hook opened from a group starts in that group, with the defaults of the form.
func TestEditorOfANewHookStartsInItsGroup(t *testing.T) {
	t.Parallel()
	r := newRig(t)

	rec := do(t, r.router, http.MethodGet, panelPath("/editor?event=task_finish&phase=post"), nil)

	body := rec.Body.String()
	assert.Contains(t, body, "New hook")
	assert.Contains(t, body, `<option value="task_finish" selected>`)
	assert.Contains(t, body, `<option value="post" selected>`)
	assert.Contains(t, body, `name="timeout" class="input" value="600"`)
}

// The order submitted is the whole workspace order; a partial one is refused and nothing moves.
func TestReorderStoresTheWholeOrderOrRefusesIt(t *testing.T) {
	t.Parallel()
	a := hook(t, "a", models.PhasePre, models.EventTaskStart, "a")
	b := hook(t, "b", models.PhasePre, models.EventTaskStart, "b")
	c := hook(t, "c", models.PhasePost, models.EventTaskFinish, "c")
	r := newRig(t, a, b, c)

	ok := do(t, r.router, http.MethodPost, panelPath("/reorder"), url.Values{dto.FieldOrder: {"b,a,c"}})
	assert.Contains(t, ok.Body.String(), "Order saved.")
	assert.Equal(t, []string{"b", "a", "c"}, r.hooks.order(workspace))

	partial := do(t, r.router, http.MethodPost, panelPath("/reorder"), url.Values{dto.FieldOrder: {"a,c"}})
	assert.Contains(t, partial.Body.String(), "must name the 3 hooks")
	assert.Equal(t, []string{"b", "a", "c"}, r.hooks.order(workspace))
}

func TestEnabledFollowsTheFormCheckbox(t *testing.T) {
	t.Parallel()
	r := newRig(t, hook(t, "h1", models.PhasePre, models.EventTaskStart, "x"))

	do(t, r.router, http.MethodPost, panelPath("/h1/enabled"), nil)
	assert.False(t, r.hooks.hooks[0].Enabled())

	do(t, r.router, http.MethodPost, panelPath("/h1/enabled"), url.Values{dto.FieldEnabled: {"on"}})
	assert.True(t, r.hooks.hooks[0].Enabled())
}

func TestDeleteRemovesTheHookOfTheWorkspace(t *testing.T) {
	t.Parallel()
	r := newRig(t, hook(t, "h1", models.PhasePre, models.EventTaskStart, "x"))

	rec := do(t, r.router, http.MethodPost, panelPath("/h1/delete"), nil)

	assert.Contains(t, rec.Body.String(), "Hook deleted.")
	assert.Empty(t, r.hooks.hooks)
}

// Probar runs the hook and shows its status, exit code and escaped output in its row.
func TestTestShowsTheOutcomeInTheRowEscaped(t *testing.T) {
	t.Parallel()
	r := newRig(t, hook(t, "h1", models.PhasePre, models.EventTaskStart, "echo"))
	r.runner.outcome = models.RunOutcome{Status: models.RunFailed, ExitCode: 2, Output: "<b>boom</b>"}

	rec := do(t, r.router, http.MethodPost, panelPath("/h1/test"), nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, r.runner.runs)
	body := rec.Body.String()
	assert.Contains(t, body, `data-hook-test="failed"`)
	assert.Contains(t, body, "Failed · exit code 2")
	assert.Contains(t, body, "&lt;b&gt;boom&lt;/b&gt;")
	assert.NotContains(t, body, "<b>boom</b>")
}

// A test that could not start is shown in the row with its reason, not as a failure of the tab.
func TestTestThatCannotStartIsShownInItsRow(t *testing.T) {
	t.Parallel()
	r := newRig(t, hook(t, "h1", models.PhasePre, models.EventTaskStart, "echo"))
	r.runner.err = fmt.Errorf("%w: missing directory", hookerrors.ErrRunNotStarted)

	rec := do(t, r.router, http.MethodPost, panelPath("/h1/test"), nil)

	body := rec.Body.String()
	assert.Contains(t, body, `data-hook-test="error"`)
	assert.Contains(t, body, "missing directory")
}

func TestTestOfAnUnknownHookIsTheTabFailure(t *testing.T) {
	t.Parallel()
	r := newRig(t)

	rec := do(t, r.router, http.MethodPost, panelPath("/nope/test"), nil)

	assert.Equal(t, 0, r.runner.runs)
	assert.Contains(t, rec.Body.String(), "hook not found")
}

// The test runs real commands, so the endpoint is POST only.
func TestTestIsNotAvailableWithGet(t *testing.T) {
	t.Parallel()
	r := newRig(t, hook(t, "h1", models.PhasePre, models.EventTaskStart, "echo"))

	rec := do(t, r.router, http.MethodGet, panelPath("/h1/test"), nil)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, 0, r.runner.runs)
}

// The tab is the same fragment whether or not the request came from htmx: the dock loads it and the actions swap it.
func TestPanelIsTheSameFragmentWithOrWithoutHtmx(t *testing.T) {
	t.Parallel()
	r := newRig(t, hook(t, "h1", models.PhasePre, models.EventTaskStart, "echo"))

	plain := do(t, r.router, http.MethodGet, panelPath("/panel"), nil)
	req := httptest.NewRequest(http.MethodGet, panelPath("/panel"), nil)
	req.Header.Set("HX-Request", "true")
	htmx := httptest.NewRecorder()
	r.router.ServeHTTP(htmx, req)

	assert.Equal(t, plain.Body.String(), htmx.Body.String())
	assert.Contains(t, plain.Body.String(), `data-hook="h1"`)
}

// The dock tab renders the same panel, and a failure to read the hooks is shown in the tab.
func TestTabRendersThePanelOfTheRouteWorkspace(t *testing.T) {
	t.Parallel()
	r := newRig(t, hook(t, "h1", models.PhasePre, models.EventTaskStart, "echo"))
	handlers := inbound.NewHandlers(inbound.UseCases{List: application.NewListHooks(r.hooks)})

	var out strings.Builder
	component := handlers.Tab(context.Background(), dock.Params{"id": workspace})
	require.NoError(t, component.Render(context.Background(), &out))

	assert.Contains(t, out.String(), `id="workspace-hooks-w1"`)
	assert.Contains(t, out.String(), `data-hook="h1"`)
}
