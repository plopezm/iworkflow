package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/application"
	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
)

func TestStageProjectPathsResolvesRootThenStagesThePaths(t *testing.T) {
	git := &fakeGitStatus{}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}

	err := application.NewStageProjectPaths(roots, git).Execute(context.Background(), anyScope(), []string{"a.rs"})

	require.NoError(t, err)
	require.Equal(t, "/tmp/demo", git.stageRoot)
	require.Equal(t, []string{"a.rs"}, git.staged)
}

func TestStageProjectPathsUnknownScopeNeverTouchesGit(t *testing.T) {
	roots := &fakeWorkspaceRoots{err: unknownScopeErr()}
	git := &fakeGitStatus{}

	err := application.NewStageProjectPaths(roots, git).Execute(context.Background(), anyScope(), nil)

	require.Equal(t, treeerrors.WorkspaceNotFound(), err)
	require.Zero(t, git.calls)
}

func TestUnstageProjectPathsResolvesRootThenUnstagesThePaths(t *testing.T) {
	git := &fakeGitStatus{}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}

	err := application.NewUnstageProjectPaths(roots, git).Execute(context.Background(), anyScope(), []string{"a.rs"})

	require.NoError(t, err)
	require.Equal(t, "/tmp/demo", git.unstageRoot)
	require.Equal(t, []string{"a.rs"}, git.unstaged)
}

func TestUnstageProjectPathsUnknownScopeNeverTouchesGit(t *testing.T) {
	roots := &fakeWorkspaceRoots{err: unknownScopeErr()}
	git := &fakeGitStatus{}

	err := application.NewUnstageProjectPaths(roots, git).Execute(context.Background(), anyScope(), nil)

	require.Equal(t, treeerrors.WorkspaceNotFound(), err)
	require.Zero(t, git.calls)
}

func TestCommitProjectChangesCommitsTheTrimmedMessage(t *testing.T) {
	git := &fakeGitStatus{}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}

	err := application.NewCommitProjectChanges(roots, git).Execute(context.Background(), anyScope(), "  fix: x \n")

	require.NoError(t, err)
	require.Equal(t, "/tmp/demo", git.commitRoot)
	require.Equal(t, "fix: x", git.commitMsg)
}

func TestCommitProjectChangesBlankMessageNeverReachesGit(t *testing.T) {
	roots := &fakeWorkspaceRoots{}
	git := &fakeGitStatus{}

	err := application.NewCommitProjectChanges(roots, git).Execute(context.Background(), anyScope(), "  \n")

	require.Equal(t, treeerrors.EmptyCommitMessage(), err)
	require.Empty(t, roots.calls)
	require.Zero(t, git.calls)
}

func TestCommitProjectChangesUnknownScopeNeverTouchesGit(t *testing.T) {
	roots := &fakeWorkspaceRoots{err: unknownScopeErr()}
	git := &fakeGitStatus{}

	err := application.NewCommitProjectChanges(roots, git).Execute(context.Background(), anyScope(), "msg")

	require.Equal(t, treeerrors.WorkspaceNotFound(), err)
	require.Zero(t, git.calls)
}
