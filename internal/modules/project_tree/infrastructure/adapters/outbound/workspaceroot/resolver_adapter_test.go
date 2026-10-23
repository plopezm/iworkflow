package workspaceroot

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
)

// fakeLookup returns a fixed answer and records the identifiers it was asked for.
type fakeLookup struct {
	root  *models.WorkspaceRoot
	err   error
	asked []uuid.UUID
}

func (f *fakeLookup) Find(_ context.Context, id uuid.UUID) (*models.WorkspaceRoot, error) {
	f.asked = append(f.asked, id)
	return f.root, f.err
}

func TestResolveReturnsTheRootWhenFound(t *testing.T) {
	root := models.WorkspaceRoot{Name: "demo", Path: "/tmp/x"}
	adapter := NewResolverAdapter(&fakeLookup{root: &root}, &fakeLookup{})

	result, err := adapter.Resolve(context.Background(), models.WorkspaceScope(uuid.New()))

	require.NoError(t, err)
	require.Equal(t, root, result)
}

func TestResolveMapsAnUnknownIdentifierToWorkspaceNotFound(t *testing.T) {
	adapter := NewResolverAdapter(&fakeLookup{}, &fakeLookup{})

	_, err := adapter.Resolve(context.Background(), models.WorkspaceScope(uuid.New()))

	require.Equal(t, treeerrors.WorkspaceNotFound(), err)
}

func TestResolveMapsALookupFailureToWorkspaceLookup(t *testing.T) {
	adapter := NewResolverAdapter(&fakeLookup{err: errors.New("boom")}, &fakeLookup{})

	_, err := adapter.Resolve(context.Background(), models.WorkspaceScope(uuid.New()))

	require.Equal(t, treeerrors.WorkspaceLookup("boom"), err)
}

func TestTaskScopeUsesTheTaskLookupOnly(t *testing.T) {
	taskRoot := models.WorkspaceRoot{Name: "task", Path: "/tmp/wt"}
	workspaces := &fakeLookup{}
	tasks := &fakeLookup{root: &taskRoot}
	adapter := NewResolverAdapter(workspaces, tasks)
	id := uuid.New()

	result, err := adapter.Resolve(context.Background(), models.TaskScope(id))

	require.NoError(t, err)
	require.Equal(t, taskRoot, result)
	require.Equal(t, []uuid.UUID{id}, tasks.asked)
	require.Empty(t, workspaces.asked)

	_, err = adapter.Resolve(context.Background(), models.WorkspaceScope(id))
	require.Equal(t, treeerrors.WorkspaceNotFound(), err)
}

func TestMissingLookupIsAnErrorNotAPanic(t *testing.T) {
	adapter := NewResolverAdapter(nil, nil)

	_, err := adapter.Resolve(context.Background(), models.WorkspaceScope(uuid.New()))

	require.Equal(t, treeerrors.WorkspaceLookup("no hay búsqueda configurada"), err)
}
