package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/application"
	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestReadProjectFileResolvesRootThenReads(t *testing.T) {
	file := models.ProjectFile{Name: "main.rs", Path: "src/main.rs", Content: "fn main() {}"}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}
	files := &fakeFileStore{file: file}

	result, err := application.NewReadProjectFile(roots, files).Execute(context.Background(), anyScope(), "src/main.rs")

	require.NoError(t, err)
	require.Equal(t, file, result)
	require.Equal(t, 1, files.reads)
}

func TestReadProjectFileUnknownWorkspaceNeverReads(t *testing.T) {
	roots := &fakeWorkspaceRoots{err: unknownScopeErr()}
	files := &fakeFileStore{}

	_, err := application.NewReadProjectFile(roots, files).Execute(context.Background(), anyScope(), "a.txt")

	require.Equal(t, treeerrors.WorkspaceNotFound(), err)
	require.Zero(t, files.reads)
}
