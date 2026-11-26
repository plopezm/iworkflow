package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	tasksapp "iworkflow/internal/modules/tasks/application"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
	brancheserrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	workspacesapp "iworkflow/internal/modules/workspaces/application"
	workspacespersistence "iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
)

// activeTasksRepo answers only ListActive; any other call panics on the nil embedded interface.
type activeTasksRepo struct {
	tasksoutbound.TaskRepository
	tasks []tasksmodels.Task
	err   error
}

func (r activeTasksRepo) ListActive(context.Context, *string) ([]tasksmodels.Task, error) {
	return r.tasks, r.err
}

func TestBranchTaskBranchesKeepsOnlyTheBranchesOfOpenTasks(t *testing.T) {
	t.Parallel()
	repo := activeTasksRepo{tasks: []tasksmodels.Task{
		{Branch: "iwf/live", Status: tasksmodels.StatusBacklog},
		{Branch: "", Status: tasksmodels.StatusBacklog},
		{Branch: "iwf/merged", Status: tasksmodels.StatusMerged},
		{Branch: "iwf/discarded", Status: tasksmodels.StatusDiscarded},
	}}
	bridge := &branchTaskBranches{list: tasksapp.NewListTasks(repo)}

	names, err := bridge.InUse(context.Background(), "ws")

	require.NoError(t, err)
	assert.Equal(t, []string{"iwf/live"}, names)
}

func TestBranchTaskBranchesWrapsTheListFailure(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	bridge := &branchTaskBranches{list: tasksapp.NewListTasks(activeTasksRepo{err: boom})}

	names, err := bridge.InUse(context.Background(), "ws")

	require.ErrorIs(t, err, boom)
	assert.Nil(t, names)
}

func TestBranchWorkspacesResolvesAndRefusesUnknownWorkspaces(t *testing.T) {
	t.Parallel()
	db := dbtest.New(t)
	const knownID = "6f1c9d2e-3b4a-4c5d-8e9f-0a1b2c3d4e70"
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES (?, 'one', '/projects/one', 'develop', '2026-01-01T00:00:00Z')`, knownID)
	require.NoError(t, err)
	bridge := &branchWorkspaces{get: workspacesapp.NewGetWorkspace(workspacespersistence.NewWorkspaceRepository(db))}

	target, err := bridge.Resolve(context.Background(), knownID)
	require.NoError(t, err)
	assert.Equal(t, "/projects/one", target.Path)
	assert.Equal(t, "develop", target.BaseBranch)
	assert.Equal(t, "one", target.Name)

	for _, id := range []string{"not-a-uuid", "11111111-2222-4333-8444-555555555555"} {
		_, err := bridge.Resolve(context.Background(), id)
		assert.ErrorIs(t, err, brancheserrors.ErrWorkspaceNotFound, id)
	}
}
