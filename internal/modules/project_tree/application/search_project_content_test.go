package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/application"
	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestSearchProjectContentResolvesRootThenSearches(t *testing.T) {
	want := models.ContentResult{Files: []models.ContentFile{{Path: "a.go", Total: 1}}}
	roots := &fakeWorkspaceRoots{root: workspaceRoot()}
	searcher := &fakeContentSearch{result: want}
	query, err := models.NewContentQuery("foo", false, false, false)
	require.NoError(t, err)

	got, err := application.NewSearchProjectContent(roots, searcher).Execute(context.Background(), anyScope(), query)

	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, []string{"/tmp/demo"}, searcher.roots)
	require.Equal(t, []models.ContentQuery{query}, searcher.queries)
}

func TestSearchProjectContentUnknownScopeNeverSearches(t *testing.T) {
	roots := &fakeWorkspaceRoots{err: unknownScopeErr()}
	searcher := &fakeContentSearch{}
	query, err := models.NewContentQuery("foo", false, false, false)
	require.NoError(t, err)

	_, err = application.NewSearchProjectContent(roots, searcher).Execute(context.Background(), anyScope(), query)

	require.Error(t, err)
	require.Empty(t, searcher.roots)
}
