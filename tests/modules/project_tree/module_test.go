package projecttree_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/infrastructure/di"
)

// mapLookup answers Find from a fixed table, the way the workspaces and tasks modules will.
type mapLookup map[uuid.UUID]models.WorkspaceRoot

func (m mapLookup) Find(_ context.Context, id uuid.UUID) (*models.WorkspaceRoot, error) {
	root, ok := m[id]
	if !ok {
		return nil, nil
	}
	return &root, nil
}

// TestModuleWiresWorkspaceAndTaskScopesToTheirOwnRoots checks the composition root end to end on the
// real filesystem: a workspace scope and a task scope list different directories.
func TestModuleWiresWorkspaceAndTaskScopesToTheirOwnRoots(t *testing.T) {
	workspaceDir := t.TempDir()
	taskDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, "ws.txt"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "task.txt"), nil, 0o600))
	workspaceID := uuid.New()
	taskID := uuid.New()
	module := di.NewModule(di.Deps{
		Workspaces: mapLookup{workspaceID: {Name: "ws", Path: workspaceDir}},
		Tasks:      mapLookup{taskID: {Name: "task", Path: taskDir}},
	})
	ctx := context.Background()

	workspaceListing, err := module.ListDirectory.Execute(ctx, models.WorkspaceScope(workspaceID), "")
	require.NoError(t, err)
	require.Equal(t, "ws", workspaceListing.WorkspaceName)
	require.Equal(t, "ws.txt", workspaceListing.Contents.Entries[0].Name)

	taskListing, err := module.ListDirectory.Execute(ctx, models.TaskScope(taskID), "")
	require.NoError(t, err)
	require.Equal(t, "task", taskListing.WorkspaceName)
	require.Equal(t, "task.txt", taskListing.Contents.Entries[0].Name)

	_, err = module.ListDirectory.Execute(ctx, models.TaskScope(workspaceID), "")
	require.Equal(t, treeerrors.WorkspaceNotFound(), err, "a workspace id is unknown as a task")
}

// TestModuleUseCasesReachGitAndFileStore checks that the other ports are wired too.
func TestModuleUseCasesReachGitAndFileStore(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hola"), 0o600))
	id := uuid.New()
	module := di.NewModule(di.Deps{
		Workspaces: mapLookup{id: {Name: "ws", Path: dir}},
		Tasks:      mapLookup{},
	})
	ctx := context.Background()
	scope := models.WorkspaceScope(id)

	file, err := module.ReadFile.Execute(ctx, scope, "a.txt")
	require.NoError(t, err)
	require.Equal(t, "hola", file.Content)

	require.NoError(t, module.WriteFile.Execute(ctx, scope, "a.txt", "adios"))
	content, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "adios", string(content))

	base, err := module.FileBase.Execute(ctx, scope, "a.txt")
	require.NoError(t, err)
	require.Nil(t, base, "a folder that is not a repository has no HEAD content")

	changes, err := module.GitChanges.Execute(ctx, scope)
	require.NoError(t, err)
	require.Empty(t, changes)

	found, err := module.SearchFiles.Execute(ctx, scope, "a")
	require.NoError(t, err)
	require.Empty(t, found, "not a git repository lists no files")

	err = module.CommitChanges.Execute(ctx, scope, "  ")
	require.Equal(t, treeerrors.EmptyCommitMessage(), err)
}
