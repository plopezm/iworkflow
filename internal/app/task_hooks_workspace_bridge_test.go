package app

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	workspacesapp "iworkflow/internal/modules/workspaces/application"
	workspacespersistence "iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
)

const bridgeWorkspaceID = "6f1c9d2e-3b4a-4c5d-8e9f-0a1b2c3d4e5f"

// seedBridgeWorkspace stores one workspace the bridge can find.
func seedBridgeWorkspace(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES (?, 'one', '/projects/one', 'main', '2026-01-01T00:00:00Z')`, bridgeWorkspaceID)
	require.NoError(t, err)
}

// The task hooks find the directory of a workspace through the workspaces module, where a test of a hook runs.
func TestTaskHooksWorkspaceBridgeReturnsTheWorkspacePath(t *testing.T) {
	t.Parallel()
	db := dbtest.New(t)
	seedBridgeWorkspace(t, db)
	bridge := &taskHooksWorkspaces{get: workspacesapp.NewGetWorkspace(workspacespersistence.NewWorkspaceRepository(db))}

	path, err := bridge.Path(context.Background(), bridgeWorkspaceID)

	require.NoError(t, err)
	assert.Equal(t, "/projects/one", path)
}

// An unknown or malformed id is an error, so a test never runs in a guessed directory.
func TestTaskHooksWorkspaceBridgeRefusesUnknownWorkspaces(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		id   string
	}{
		{name: "unknown workspace", id: "11111111-2222-4333-8444-555555555555"},
		{name: "malformed id", id: "not-a-uuid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			db := dbtest.New(t)
			seedBridgeWorkspace(t, db)
			bridge := &taskHooksWorkspaces{get: workspacesapp.NewGetWorkspace(workspacespersistence.NewWorkspaceRepository(db))}

			path, err := bridge.Path(context.Background(), tt.id)

			require.Error(t, err)
			assert.Empty(t, path)
		})
	}
}

// The Hooks screen lists the workspaces as the projects a hook can be assigned to.
func TestTaskHooksProjectsListsTheWorkspaces(t *testing.T) {
	t.Parallel()
	db := dbtest.New(t)
	seedBridgeWorkspace(t, db)
	bridge := &taskHooksProjects{list: workspacesapp.NewListWorkspaces(workspacespersistence.NewWorkspaceRepository(db))}

	projects, err := bridge.Projects(context.Background())

	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, bridgeWorkspaceID, projects[0].ID)
	assert.Equal(t, "one", projects[0].Name)
}
