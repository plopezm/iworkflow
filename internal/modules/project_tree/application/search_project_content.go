//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// SearchProjectContent finds the project files whose content matches a query.
type SearchProjectContent struct {
	roots    outbound.WorkspaceRootPort
	searcher outbound.ContentSearchPort
}

// NewSearchProjectContent builds the use case.
func NewSearchProjectContent(roots outbound.WorkspaceRootPort, searcher outbound.ContentSearchPort) *SearchProjectContent {
	return &SearchProjectContent{roots: roots, searcher: searcher}
}

// Execute resolves the project root and searches it.
func (u *SearchProjectContent) Execute(ctx context.Context, scope models.ProjectScope, query models.ContentQuery) (models.ContentResult, error) {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return models.ContentResult{}, err
	}
	return u.searcher.Search(ctx, root.Path, query)
}
