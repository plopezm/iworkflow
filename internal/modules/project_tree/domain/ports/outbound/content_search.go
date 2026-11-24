package outbound

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// ContentSearchPort looks for text inside the files of a project.
type ContentSearchPort interface {
	// Search returns the files under root (git-ignored and binary files excluded) whose content matches
	// query, ordered by path, with at most models.MaxContentFiles files and models.MaxMatchesPerFile
	// matches per file.
	Search(ctx context.Context, root string, query models.ContentQuery) (models.ContentResult, error)
}
