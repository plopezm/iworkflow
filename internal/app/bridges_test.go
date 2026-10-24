package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	workspacespersistence "iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
	workspacesdi "iworkflow/internal/modules/workspaces/infrastructure/di"
)

// newWorkspaces builds the workspaces module over a migrated temporary database, with no workspace.
func newWorkspaces(t *testing.T) *workspacesdi.Module {
	t.Helper()
	db := dbtest.New(t)
	return workspacesdi.NewModule(workspacesdi.Deps{
		DB:      db,
		HomeDir: t.TempDir(),
		Tasks:   workspacespersistence.NewTemporaryWorkspaceTasks(db),
	})
}

func TestWorkspaceRootsReturnNilForAnUnknownWorkspace(t *testing.T) {
	roots := &workspaceRoots{get: newWorkspaces(t).Get}

	root, err := roots.Find(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Nil(t, root)
}

func TestWorkspaceTargetsReturnNilForAnUnknownWorkspace(t *testing.T) {
	resolve := workspaceTargets(newWorkspaces(t).Get)

	target, err := resolve(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Nil(t, target)
}

// TODO(tasks): remove these cases with the placeholders when the tasks module is migrated.
func TestPendingTaskLookupsFindNothing(t *testing.T) {
	ctx := context.Background()

	root, err := taskRootsPending{}.Find(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, root)

	target, err := taskTargetsPending(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, target)

	source, err := taskSourcesPending{}.Find(ctx, uuid.New().String())
	require.NoError(t, err)
	require.Nil(t, source)
}
