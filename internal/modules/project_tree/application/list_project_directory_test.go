package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/application"
	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestListProjectDirectoryResolvesRootThenLists(t *testing.T) {
	scope := anyScope()
	contents := models.DirectoryContents{
		Path:    "src",
		Entries: []models.TreeEntry{{Name: "main.rs", Path: "src/main.rs", Kind: models.TreeEntryFile}},
	}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}
	reader := &fakeDirectoryReader{contents: contents}

	result, err := application.NewListProjectDirectory(roots, reader).Execute(context.Background(), scope, "src")

	require.NoError(t, err)
	require.Equal(t, models.ProjectDirectory{WorkspaceName: "demo", Contents: contents}, result)
	require.Equal(t, []models.ProjectScope{scope}, roots.calls)
	require.Equal(t, []string{"/tmp/demo"}, reader.roots)
	require.Equal(t, []string{"src"}, reader.relative)
}

func TestListProjectDirectoryPropagatesWorkspaceNotFoundWithoutListing(t *testing.T) {
	roots := &fakeWorkspaceRoots{err: unknownScopeErr()}
	reader := &fakeDirectoryReader{}

	_, err := application.NewListProjectDirectory(roots, reader).Execute(context.Background(), anyScope(), "")

	require.Equal(t, treeerrors.WorkspaceNotFound(), err)
	require.Empty(t, reader.roots, "the reader must not run for an unknown scope")
}

func TestListProjectDirectoryPropagatesReaderErrors(t *testing.T) {
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}
	reader := &fakeDirectoryReader{err: treeerrors.PathNotFound("x")}

	_, err := application.NewListProjectDirectory(roots, reader).Execute(context.Background(), anyScope(), "x")

	require.Equal(t, treeerrors.PathNotFound("x"), err)
}
