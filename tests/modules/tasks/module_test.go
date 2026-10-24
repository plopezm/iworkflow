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

// nopStarter is the start use case of slices E and F; the create tests here never ask to start a task.
type nopStarter struct{}

func (nopStarter) Start(context.Context, models.TaskID) (models.Task, error) {
	return models.Task{}, nil
}

// nopLayout places worktrees under /worktrees; no worktree is created here.
type nopLayout struct{}

func (nopLayout) WorktreePath(workspaceName string, dirName string) string {
	return "/worktrees/" + workspaceName + "/" + dirName
}

// nopStorage is the attachment storage of slice D. These tests store no images, so only the
// task-folder cleanup of a deletion reaches it.
type nopStorage struct {
	outbound.AttachmentStorage
}

func (nopStorage) DeleteTaskDir(context.Context, models.TaskID) error { return nil }

// nopAttachments is the attachment metadata repository of slice D, unused here.
type nopAttachments struct {
	outbound.TaskAttachmentRepository
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
	module, err := di.NewModule(di.Deps{
		DB:          repo.DB,
		Workspaces:  workspaceLookup{repo: repo},
		Attachments: nopAttachments{},
		Storage:     nopStorage{},
		Worktrees:   nopLayout{},
		Start:       nopStarter{},
	})
	require.NoError(t, err)
	t.Cleanup(module.ShutdownEvents)
	return module
}

func TestModuleRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	_, err := di.NewModule(di.Deps{DB: repo.DB})
	assert.ErrorContains(t, err, "missing dependency: Workspaces")
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
