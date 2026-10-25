package app

import (
	"context"
	"io"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/listen"
	"iworkflow/internal/core/restart"
	workspacesmodels "iworkflow/internal/modules/workspaces/domain/models"
	workspacesdi "iworkflow/internal/modules/workspaces/infrastructure/di"
)

// noWorkspaceTasks is the workspace cascade of the tests that compose only the workspaces module: no task exists.
type noWorkspaceTasks struct{}

func (noWorkspaceTasks) DeleteAll(context.Context, workspacesmodels.WorkspaceID) error { return nil }

// newWorkspaces builds the workspaces module over a migrated temporary database, with no workspace.
func newWorkspaces(t *testing.T) *workspacesdi.Module {
	t.Helper()
	return workspacesdi.NewModule(workspacesdi.Deps{
		DB:      dbtest.New(t),
		HomeDir: t.TempDir(),
		Tasks:   noWorkspaceTasks{},
	})
}

// composeWithTasks composes the whole server over a migrated temporary database, so that the tasks bridges read the
// real tasks module. No agent is started.
func composeWithTasks(t *testing.T) *App {
	t.Helper()
	application, err := New(Deps{
		DB:        dbtest.New(t),
		Config:    NewTestConfig(t),
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		StaticDir: t.TempDir(),
		Listen:    listen.Info{Addr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 3000)},
		Restart:   restart.NewHandle(),
	})
	require.NoError(t, err)
	return application
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

// The tasks lookups find nothing for an identifier no task has, as the Rust resolvers do: the ports answer nil and no
// error, so the screens show the task as missing instead of failing.
func TestTaskLookupsFindNothingForAnUnknownTask(t *testing.T) {
	ctx := context.Background()
	application := composeWithTasks(t)

	root, err := (&taskRootLookup{get: application.tasks.Get}).Find(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, root)

	target, err := taskTargets(application.tasks.Get)(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, target)

	source, err := (&taskSourceLookup{get: application.tasks.Get, workspaces: newWorkspaces(t).Get}).Find(ctx, uuid.New().String())
	require.NoError(t, err)
	require.Nil(t, source)
}

// A task identifier that is not a UUID is unknown too, not an error: the merge request panel passes raw text.
func TestTaskSourceLookupTreatsAMalformedIdentifierAsUnknown(t *testing.T) {
	application := composeWithTasks(t)
	source := &taskSourceLookup{get: application.tasks.Get, workspaces: newWorkspaces(t).Get}

	found, err := source.Find(context.Background(), "not-a-uuid")

	require.NoError(t, err)
	assert.Nil(t, found)
}

// The workspace lookup of the tasks module answers nil for an unknown or malformed identifier, and lists nothing
// when there is no workspace.
func TestTaskWorkspaceLookupFindsNothingWithoutWorkspaces(t *testing.T) {
	ctx := context.Background()
	workspaces := newWorkspaces(t)
	lookup := &taskWorkspaceLookup{get: workspaces.Get, list: workspaces.List}

	found, err := lookup.Find(ctx, uuid.New().String())
	require.NoError(t, err)
	assert.Nil(t, found)

	malformed, err := lookup.Find(ctx, "not-a-uuid")
	require.NoError(t, err)
	assert.Nil(t, malformed)

	all, err := lookup.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, all)
}

// Deleting the tasks of a workspace that has none succeeds, so the workspace delete is not blocked by the cascade.
func TestWorkspaceTasksBridgeDeletesNothingForAWorkspaceWithoutTasks(t *testing.T) {
	application := composeWithTasks(t)
	bridge := &workspaceTasksBridge{deleteAll: application.tasks.DeleteWorkspace}

	err := bridge.DeleteAll(context.Background(), workspacesmodels.WorkspaceIDFromUUID(uuid.New()))

	require.NoError(t, err)
}
