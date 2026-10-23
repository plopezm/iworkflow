package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/application"
	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestListProjectGitChangesResolvesRootThenAsksGit(t *testing.T) {
	change := models.GitChange{Path: "a.rs", Kind: models.GitChangeModified, Staged: false, Unstaged: true}
	git := &fakeGitStatus{changes: []models.GitChange{change}}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}

	result, err := application.NewListProjectGitChanges(roots, git).Execute(context.Background(), anyScope())

	require.NoError(t, err)
	require.Equal(t, []models.GitChange{change}, result)
	require.Equal(t, "/tmp/demo", git.changesRoot)
}

func TestListProjectGitChangesUnknownScopeNeverAsksGit(t *testing.T) {
	roots := &fakeWorkspaceRoots{err: unknownScopeErr()}
	git := &fakeGitStatus{}

	_, err := application.NewListProjectGitChanges(roots, git).Execute(context.Background(), anyScope())

	require.Equal(t, treeerrors.WorkspaceNotFound(), err)
	require.Zero(t, git.calls)
}
