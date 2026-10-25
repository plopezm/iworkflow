package tasks_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/di"
)

// taskDetailRoute is the name of the dock route of the detail page of a task.
const taskDetailRoute = "task_detail"

// renderDetailTab renders the detail tab of a task in the given language, as the shell does when it opens the address.
func renderDetailTab(t *testing.T, module *di.Module, locale i18n.Locale, id string) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	for _, route := range module.Tabs() {
		if route.Name != taskDetailRoute {
			continue
		}
		var out bytes.Buffer
		require.NoError(t, route.Render(ctx, dock.Params{"id": id}).Render(ctx, &out))
		return out.String()
	}
	t.Fatalf("no dock route named %q", taskDetailRoute)
	return ""
}

// newIDForTest is the identifier of a task that does not exist.
func newIDForTest(t *testing.T) string {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return id.String()
}

func TestDetailTabOpensTheTaskByItsIdentifier(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	task := newTask(t, workspaceID, "detalle-uno", models.StatusBacklog)
	insertTask(t, repo, task)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.En, task.ID.String())

	assert.Contains(t, body, "detalle-uno")
	assert.NotContains(t, body, "no longer exists")
}

func TestDetailTabSaysSoForATaskThatNoLongerExists(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.En, newIDForTest(t))

	assert.Contains(t, body, "This task no longer exists.")
}

func TestDetailTabTellsTheGoneTaskInTheRequestLanguage(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	module, _ := boardRouter(t, repo)
	missing := newIDForTest(t)

	assert.Contains(t, renderDetailTab(t, module, i18n.Es, missing), "Esta tarea ya no existe.")
	assert.Contains(t, renderDetailTab(t, module, i18n.De, missing), "Diese Aufgabe existiert nicht mehr.")
}

func TestDetailTabRejectsAnIdentifierThatIsNotOne(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.En, "not-a-task")

	assert.NotContains(t, body, "no longer exists")
	assert.Contains(t, body, "not-a-task")
}

func TestDetailRouteTakesPrecedenceOverTheNotMigratedTasksCatchAll(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	module, _ := boardRouter(t, repo)
	// The composition root registers the module tabs before the not-migrated catch-all of /tasks/{rest...}.
	catchAll := dock.Route{
		Name:    "not_migrated",
		Pattern: "/tasks/{rest...}",
		Render:  func(context.Context, dock.Params) templ.Component { return templ.NopComponent },
	}
	registry, err := dock.NewRegistry(append(module.Tabs(), catchAll)...)
	require.NoError(t, err)
	id := newIDForTest(t)

	route, params, ok := registry.Match("/tasks/" + id)
	require.True(t, ok)
	assert.Equal(t, taskDetailRoute, route.Name)
	assert.Equal(t, id, params["id"])

	diff, _, ok := registry.Match("/tasks/" + id + "/diff")
	require.True(t, ok)
	assert.Equal(t, "not_migrated", diff.Name, "diff, terminal and the rest stay not migrated")
}
