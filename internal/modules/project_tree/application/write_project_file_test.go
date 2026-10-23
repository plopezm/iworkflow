package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/application"
	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
)

func TestWriteProjectFileResolvesRootThenWrites(t *testing.T) {
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}
	files := &fakeFileStore{}

	err := application.NewWriteProjectFile(roots, files).Execute(context.Background(), anyScope(), "a.txt", "hola")

	require.NoError(t, err)
	require.Equal(t, []fileWrite{{root: "/tmp/demo", relative: "a.txt", content: "hola"}}, files.writes)
}

func TestWriteProjectFileStoreErrorIsPropagated(t *testing.T) {
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}
	files := &fakeFileStore{err: treeerrors.NotAFile("d")}

	err := application.NewWriteProjectFile(roots, files).Execute(context.Background(), anyScope(), "d", "x")

	require.Equal(t, treeerrors.NotAFile("d"), err)
}
