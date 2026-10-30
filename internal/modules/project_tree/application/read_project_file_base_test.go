package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/application"
)

func TestReadProjectFileBaseReturnsHeadContent(t *testing.T) {
	old := "old"
	git := &fakeGitStatus{base: &old}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}

	result, err := application.NewReadProjectFileBase(roots, git).Execute(context.Background(), anyScope(), "a.rs")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "old", *result)
	require.Equal(t, "/tmp/demo", git.baseRoot)
	require.Equal(t, "a.rs", git.baseRel)
}

func TestReadProjectFileBaseUntrackedFileHasNoBase(t *testing.T) {
	git := &fakeGitStatus{}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}

	result, err := application.NewReadProjectFileBase(roots, git).Execute(context.Background(), anyScope(), "new.rs")

	require.NoError(t, err)
	require.Nil(t, result)
}
