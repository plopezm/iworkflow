package tasks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/di"
)

// stubSettings only satisfies the launch settings port; the create tests here never start a task.
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

// workspaceLookup reads the workspaces table through the fixture row inserted by insertWorkspace.
type workspaceLookup struct {
	repo *Repo
}

func (w workspaceLookup) Find(ctx context.Context, workspaceID string) (*models.WorkspaceRef, error) {
	var ref models.WorkspaceRef
	err := w.repo.DB.QueryRowContext(ctx, "SELECT id, name, path, base_branch FROM workspaces WHERE id = ?", workspaceID).
		Scan(&ref.ID, &ref.Name, &ref.Path, &ref.BaseBranch)
	if err != nil {
		return nil, nil
	}
	return &ref, nil
}

func (w workspaceLookup) List(context.Context) ([]models.WorkspaceRef, error) {
	return nil, nil
}

func newModule(t *testing.T, repo *Repo) *di.Module {
	t.Helper()
	return newModuleIn(t, repo, t.TempDir())
}

// newModuleIn builds the module with the attachments stored in attachmentsDir.
func newModuleIn(t *testing.T, repo *Repo, attachmentsDir string) *di.Module {
	t.Helper()
	module, err := di.NewModule(di.Deps{
		DB:             repo.DB,
		Workspaces:     workspaceLookup{repo: repo},
		AttachmentsDir: attachmentsDir,
		WorktreesDir:   t.TempDir(),
		Binaries:       testBinaries,
		LaunchSettings: stubSettings{},
	})
	require.NoError(t, err)
	t.Cleanup(module.ShutdownEvents)
	return module
}

func TestModuleRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	complete := func() di.Deps {
		return di.Deps{
			DB:             repo.DB,
			Workspaces:     workspaceLookup{repo: repo},
			AttachmentsDir: t.TempDir(),
			WorktreesDir:   t.TempDir(),
			Binaries:       testBinaries,
			LaunchSettings: stubSettings{},
		}
	}
	// Each required field, cleared on its own, must be reported by its own name.
	cases := map[string]func(*di.Deps){
		"DB":             func(d *di.Deps) { d.DB = nil },
		"Workspaces":     func(d *di.Deps) { d.Workspaces = nil },
		"AttachmentsDir": func(d *di.Deps) { d.AttachmentsDir = "" },
		"WorktreesDir":   func(d *di.Deps) { d.WorktreesDir = "" },
		"LaunchSettings": func(d *di.Deps) { d.LaunchSettings = nil },
	}
	for field, clear := range cases {
		deps := complete()
		clear(&deps)
		_, err := di.NewModule(deps)
		assert.ErrorContains(t, err, "missing dependency: "+field, field)
	}
	// Remote and Successors are optional: the module builds without them.
	_, err := di.NewModule(complete())
	assert.NoError(t, err)
}

func TestModuleCreatesListsEditsAndDeletesABacklogTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/module")
	module := newModule(t, repo)
	ctx := context.Background()

	created, err := module.Create.Execute(ctx, models.NewTask{WorkspaceID: workspaceID, Title: "", Prompt: "arregla el login\nmás detalles"})
	require.NoError(t, err)
	assert.Equal(t, models.StatusBacklog, created.Task.Status)
	assert.Equal(t, "arregla el login", created.Task.Title)
	assert.Equal(t, "main", created.Task.BaseBranch)

	active, err := module.List.Execute(ctx, &workspaceID)
	require.NoError(t, err)
	require.Len(t, active, 1)

	loaded, err := module.Get.Execute(ctx, created.Task.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Task.ID, loaded.ID)

	updated, err := module.Update.Execute(ctx, created.Task.ID, editInput("nuevo título", "nuevo prompt"))
	require.NoError(t, err)
	assert.Equal(t, "nuevo título", updated.Title)
	assert.Equal(t, "nuevo título", found(t, repo, created.Task.ID).Title)

	require.NoError(t, module.Delete.Execute(ctx, created.Task.ID))
	gone, err := repo.Tasks.Find(ctx, created.Task.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

func TestModuleReportsNotFoundForUnknownTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	module := newModule(t, repo)

	_, err := module.Get.Execute(context.Background(), newTaskID(t))

	assert.ErrorContains(t, err, "tarea no encontrada")
}
