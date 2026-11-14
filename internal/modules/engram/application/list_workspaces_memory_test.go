package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
)

func TestListWorkspacesMemory(t *testing.T) {
	shared := models.NewMemoryProject("shared", 5, []string{"/w/shared"})
	tests := []struct {
		name       string
		store      *fakeMemoryStore
		workspaces []models.Workspace
		want       []models.WorkspaceMemory
		wantErr    error
	}{
		{
			name:       "two workspaces matching the same project share its count",
			store:      &fakeMemoryStore{projects: []models.MemoryProject{shared}},
			workspaces: []models.Workspace{models.NewWorkspace("a", "/w/shared"), models.NewWorkspace("b", "/w/shared")},
			want: []models.WorkspaceMemory{
				{State: models.StateAvailable, Count: 5},
				{State: models.StateAvailable, Count: 5},
			},
		},
		{
			name:       "name match ignores case and a workspace without a project is no-match",
			store:      &fakeMemoryStore{projects: []models.MemoryProject{models.NewMemoryProject("IWorkFlow", 2, nil)}},
			workspaces: []models.Workspace{models.NewWorkspace("iworkflow", "/x"), models.NewWorkspace("other", "/y")},
			want: []models.WorkspaceMemory{
				{State: models.StateAvailable, Count: 2},
				{State: models.StateNoMatch},
			},
		},
		{
			name:       "a matching project without memories is available with zero",
			store:      &fakeMemoryStore{projects: []models.MemoryProject{models.NewMemoryProject("empty", 0, []string{"/w/empty"})}},
			workspaces: []models.Workspace{models.NewWorkspace("empty", "/w/empty")},
			want:       []models.WorkspaceMemory{{State: models.StateAvailable, Count: 0}},
		},
		{
			name:       "unreachable projects endpoint marks every workspace unavailable without an error",
			store:      &fakeMemoryStore{projectsErr: fmt.Errorf("down: %w", engramerrors.ErrUnavailable)},
			workspaces: []models.Workspace{models.NewWorkspace("a", "/w/shared"), models.NewWorkspace("b", "/w/none")},
			want: []models.WorkspaceMemory{
				{State: models.StateUnavailable},
				{State: models.StateUnavailable},
			},
		},
		{
			name:       "other projects errors are returned wrapped",
			store:      &fakeMemoryStore{projectsErr: errBoom},
			workspaces: []models.Workspace{models.NewWorkspace("a", "/w/shared")},
			wantErr:    errBoom,
		},
		{
			name:  "results keep the input order",
			store: &fakeMemoryStore{projects: []models.MemoryProject{shared}},
			workspaces: []models.Workspace{
				models.NewWorkspace("none", "/nowhere"),
				models.NewWorkspace("matched", "/w/shared"),
				models.NewWorkspace("also-none", "/elsewhere"),
			},
			want: []models.WorkspaceMemory{
				{State: models.StateNoMatch},
				{State: models.StateAvailable, Count: 5},
				{State: models.StateNoMatch},
			},
		},
		{
			name:       "no workspaces means no request at all",
			store:      &fakeMemoryStore{projects: []models.MemoryProject{shared}},
			workspaces: nil,
			want:       []models.WorkspaceMemory{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			useCase := NewListWorkspacesMemory(tc.store)

			got, err := useCase.Execute(context.Background(), tc.workspaces)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Empty(t, tc.store.recentCalls, "the batch never reads recent observations")
			if len(tc.workspaces) == 0 {
				assert.Zero(t, tc.store.projectsCalls, "no workspace, no request")
			}
		})
	}
}

func TestListWorkspacesMemoryAsksProjectsOncePerRead(t *testing.T) {
	store := &fakeMemoryStore{projects: []models.MemoryProject{models.NewMemoryProject("alpha", 1, []string{"/w/a"})}}
	workspaces := []models.Workspace{
		models.NewWorkspace("a1", "/w/a"),
		models.NewWorkspace("a2", "/w/a"),
		models.NewWorkspace("x", "/nowhere"),
	}

	_, err := NewListWorkspacesMemory(store).Execute(context.Background(), workspaces)

	require.NoError(t, err)
	assert.Equal(t, 1, store.projectsCalls, "one GET /projects for the whole page")
}
