package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	agentsettingsapp "iworkflow/internal/modules/agent_settings/application"
	agentsettingsmodels "iworkflow/internal/modules/agent_settings/domain/models"
	workspacesmodels "iworkflow/internal/modules/workspaces/domain/models"
	workspacespersistence "iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
	workspacesdi "iworkflow/internal/modules/workspaces/infrastructure/di"
)

// stubRuntimeAvailability reports as installed only the runtimes it holds.
type stubRuntimeAvailability struct {
	installed map[agentsettingsmodels.AgentRuntime]bool
}

func (s stubRuntimeAvailability) Installed(_ context.Context, runtime agentsettingsmodels.AgentRuntime) bool {
	return s.installed[runtime]
}

// listRuntimesWith builds the agent list of the app over the given installed runtimes.
func listRuntimesWith(installed ...agentsettingsmodels.AgentRuntime) *agentsettingsapp.ListRuntimes {
	set := map[agentsettingsmodels.AgentRuntime]bool{}
	for _, runtime := range installed {
		set[runtime] = true
	}
	return agentsettingsapp.NewListRuntimes(stubRuntimeAvailability{installed: set})
}

// seedWorkspace stores a workspace with the given default agent in db and returns it. The workspace list and
// lookup read the same rows, so the lookup under test sees the default the workspaces module stored.
func seedWorkspace(t *testing.T, db *sql.DB, defaultRuntime *string) workspacesmodels.Workspace {
	t.Helper()
	id, err := workspacesmodels.NewWorkspaceID()
	require.NoError(t, err)
	workspace := workspacesmodels.Workspace{
		ID:             id,
		Name:           "demo",
		Path:           "/repos/" + id.String(),
		BaseBranch:     "main",
		CreatedAt:      time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC),
		DefaultRuntime: defaultRuntime,
	}
	require.NoError(t, workspacespersistence.NewWorkspaceRepository(db).Save(context.Background(), workspace))
	return workspace
}

// workspacesOn builds the workspaces module over db, as the composition root does for the lookup.
func workspacesOn(t *testing.T, db *sql.DB) *workspacesdi.Module {
	t.Helper()
	return workspacesdi.NewModule(workspacesdi.Deps{DB: db, HomeDir: t.TempDir(), Tasks: noWorkspaceTasks{}})
}

// Spec workspace-default-agent S3: the tasks only see a workspace default while that agent is installed. A default
// whose agent disappeared is dropped, so the launch falls back to the system agent.
func TestTaskWorkspaceLookupKeepsDefaultRuntimeOnlyWhileInstalled(t *testing.T) {
	claude := "claude"
	cases := []struct {
		name      string
		stored    *string
		runtimes  *agentsettingsapp.ListRuntimes
		wantFound *string
	}{
		{
			name:      "installed default is kept",
			stored:    &claude,
			runtimes:  listRuntimesWith(agentsettingsmodels.RuntimeClaude, agentsettingsmodels.RuntimePi),
			wantFound: &claude,
		},
		{
			name:      "uninstalled default is dropped",
			stored:    &claude,
			runtimes:  listRuntimesWith(agentsettingsmodels.RuntimePi),
			wantFound: nil,
		},
		{
			name:      "no installed agents drops the default",
			stored:    &claude,
			runtimes:  listRuntimesWith(),
			wantFound: nil,
		},
		{
			name:      "nil runtime list drops the default",
			stored:    &claude,
			runtimes:  nil,
			wantFound: nil,
		},
		{
			name:      "workspace without default stays without default",
			stored:    nil,
			runtimes:  listRuntimesWith(agentsettingsmodels.RuntimeClaude),
			wantFound: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := dbtest.New(t)
			workspaces := workspacesOn(t, db)
			stored := seedWorkspace(t, db, tc.stored)
			lookup := &taskWorkspaceLookup{get: workspaces.Get, list: workspaces.List, listRuntimes: tc.runtimes}

			found, err := lookup.Find(ctx, stored.ID.String())

			require.NoError(t, err)
			require.NotNil(t, found)
			assert.Equal(t, stored.ID.String(), found.ID)
			assert.Equal(t, tc.wantFound, found.DefaultRuntime)
		})
	}
}

// The lookup answers nil, without error, for an unknown workspace even when a runtime list is bound.
func TestTaskWorkspaceLookupUnknownWorkspaceHasNoDefault(t *testing.T) {
	ctx := context.Background()
	db := dbtest.New(t)
	workspaces := workspacesOn(t, db)
	lookup := &taskWorkspaceLookup{get: workspaces.Get, list: workspaces.List, listRuntimes: listRuntimesWith(agentsettingsmodels.RuntimeClaude)}

	found, err := lookup.Find(ctx, uuid.New().String())

	require.NoError(t, err)
	assert.Nil(t, found)
}

// Spec workspace-default-agent S1/S2: the workspaces module offers the installed agents in selector order, and an
// empty non-nil list before the agent settings module is bound.
func TestWorkspaceRuntimesInstalledReturnsInstalledIDsInOrder(t *testing.T) {
	cases := []struct {
		name     string
		runtimes *agentsettingsapp.ListRuntimes
		want     []string
	}{
		{
			name:     "unbound list offers no agent",
			runtimes: nil,
			want:     []string{},
		},
		{
			name:     "none installed offers no agent",
			runtimes: listRuntimesWith(),
			want:     []string{},
		},
		{
			name:     "installed agents follow selector order",
			runtimes: listRuntimesWith(agentsettingsmodels.RuntimeClaude, agentsettingsmodels.RuntimeGentleShell),
			want:     []string{"gentle-shell", "claude"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &workspaceRuntimes{list: tc.runtimes}

			got := bridge.Installed(context.Background())

			require.NotNil(t, got)
			assert.Equal(t, tc.want, got)
		})
	}
}
