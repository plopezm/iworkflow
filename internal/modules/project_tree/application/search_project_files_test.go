package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/application"
)

func TestSearchProjectFilesResolvesRootThenSearches(t *testing.T) {
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}
	search := &fakeFileSearch{files: []string{"src/main.rs", "README.md"}}

	found, err := application.NewSearchProjectFiles(roots, search).Execute(context.Background(), anyScope(), "read")

	require.NoError(t, err)
	require.Equal(t, []string{"README.md"}, found)
	require.Equal(t, []string{"/tmp/demo"}, search.roots)
}

func TestSearchProjectFilesUnknownScopeNeverSearches(t *testing.T) {
	roots := &fakeWorkspaceRoots{err: unknownScopeErr()}
	search := &fakeFileSearch{}

	_, err := application.NewSearchProjectFiles(roots, search).Execute(context.Background(), anyScope(), "x")

	require.Error(t, err)
	require.Empty(t, search.roots)
}
