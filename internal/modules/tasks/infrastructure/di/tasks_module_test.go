package di_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/di"
)

// noWorkspaces knows no workspace; the wiring tests never look one up.
type noWorkspaces struct{}

func (noWorkspaces) Find(context.Context, string) (*models.WorkspaceRef, error) { return nil, nil }

func (noWorkspaces) List(context.Context) ([]models.WorkspaceRef, error) { return nil, nil }

// stubSettings only satisfies the port; the wiring never calls it.
type stubSettings struct {
	outbound.LaunchSettings
}

// testBinaries are the executables of the runtimes. The wiring only builds the adapters, so they are never run.
var testBinaries = di.RuntimeBinaries{
	GentleShell:       "gentle-shell",
	Pi:                "pi",
	Claude:            "claude",
	Cursor:            "agent",
	CursorSessionsDir: "cursor-sessions",
}

// baseDeps is the complete composition the module needs.
func baseDeps(t *testing.T) di.Deps {
	t.Helper()
	return di.Deps{
		DB:             dbtest.New(t),
		Workspaces:     noWorkspaces{},
		AttachmentsDir: t.TempDir(),
		WorktreesDir:   t.TempDir(),
		Binaries:       testBinaries,
		LaunchSettings: stubSettings{},
	}
}

// Rule from source: tasks_context.rs builds every use case the presentation and the background loops call. The
// composition must expose them all, so that no route or loop is left without its use case.
func TestNewModuleWiresEveryUseCaseTheRoutesAndLoopsNeed(t *testing.T) {
	t.Parallel()

	module, err := di.NewModule(baseDeps(t))

	require.NoError(t, err)
	assert.NotNil(t, module.Create)
	assert.NotNil(t, module.Start)
	assert.NotNil(t, module.Send)
	assert.NotNil(t, module.SetAgentMode)
	assert.NotNil(t, module.Abort)
	assert.NotNil(t, module.AnswerDialog)
	assert.NotNil(t, module.GetSubagents)
	assert.NotNil(t, module.Finish)
	assert.NotNil(t, module.Reset)
	assert.NotNil(t, module.Discard)
	assert.NotNil(t, module.Reconcile)
	assert.NotNil(t, module.SweepWorktrees)
	assert.NotNil(t, module.Successors)
	assert.NotNil(t, module.ResumeTask)
	assert.NotNil(t, module.ResumeRateLimited)
	assert.NotNil(t, module.Streams)
	assert.NotNil(t, module.Attachments)
	assert.NotNil(t, module.EventSink)
}

// Rule from source: a missing dependency is reported by name, before any adapter is built, so that a partial
// composition cannot start.
func TestNewModuleRejectsEveryMissingRequiredDependency(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		breakDep func(*di.Deps)
		want     string
	}{
		{name: "database", breakDep: func(d *di.Deps) { d.DB = nil }, want: "DB"},
		{name: "workspaces", breakDep: func(d *di.Deps) { d.Workspaces = nil }, want: "Workspaces"},
		{name: "attachments folder", breakDep: func(d *di.Deps) { d.AttachmentsDir = "" }, want: "AttachmentsDir"},
		{name: "worktrees folder", breakDep: func(d *di.Deps) { d.WorktreesDir = "" }, want: "WorktreesDir"},
		{name: "gentle-shell binary", breakDep: func(d *di.Deps) { d.Binaries.GentleShell = "" }, want: "Binaries.GentleShell"},
		{name: "pi binary", breakDep: func(d *di.Deps) { d.Binaries.Pi = "" }, want: "Binaries.Pi"},
		{name: "claude binary", breakDep: func(d *di.Deps) { d.Binaries.Claude = "" }, want: "Binaries.Claude"},
		{name: "cursor binary", breakDep: func(d *di.Deps) { d.Binaries.Cursor = "" }, want: "Binaries.Cursor"},
		{name: "cursor sessions folder", breakDep: func(d *di.Deps) { d.Binaries.CursorSessionsDir = "" }, want: "Binaries.CursorSessionsDir"},
		{name: "launch settings", breakDep: func(d *di.Deps) { d.LaunchSettings = nil }, want: "LaunchSettings"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			deps := baseDeps(t)
			testCase.breakDep(&deps)

			module, err := di.NewModule(deps)

			require.ErrorContains(t, err, testCase.want)
			assert.Nil(t, module)
		})
	}
}

// Rule from source: the live streams and the attachment content are served by the module itself, under the
// composition root's middlewares. The routes are the Rust paths.
func TestRoutesMountTheStreamsAndTheAttachmentsAtTheRustPaths(t *testing.T) {
	t.Parallel()
	module, err := di.NewModule(baseDeps(t))
	require.NoError(t, err)
	router := chi.NewRouter()

	module.Routes(router)

	got := map[string]string{}
	require.NoError(t, chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = route
		return nil
	}))
	assert.Contains(t, got, http.MethodGet+" /api/tasks/{id}/events")
	assert.Contains(t, got, http.MethodGet+" /api/tasks/events")
	assert.Contains(t, got, http.MethodGet+" /api/tasks/{task_id}/attachments/{attachment_id}")
}

// Rule from source: the agents of the module are stopped through the runtime the module built, so the
// shutdown reaches every agent the use cases started.
func TestStopAgentsIsSafeWithoutAnyAgent(t *testing.T) {
	t.Parallel()
	module, err := di.NewModule(baseDeps(t))
	require.NoError(t, err)

	assert.NotPanics(t, func() { module.StopAgents(context.Background()) })
}

// Rule from source: the answer of a dialog reaches the agent through the module, so the composer must hold the use
// case that answers; without it the route is not mounted and the dialogs of the transcript cannot be answered.
func TestRoutesMountTheAnswerOfTheDialogs(t *testing.T) {
	t.Parallel()
	module, err := di.NewModule(baseDeps(t))
	require.NoError(t, err)
	router := chi.NewRouter()

	module.Routes(router)

	got := map[string]bool{}
	require.NoError(t, chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got[method+" "+route] = true
		return nil
	}))
	assert.True(t, got[http.MethodPost+" /tasks/{id}/dialogs/{dialogId}/answer"])
}
