//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible.
package application

import (
	"context"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// DiscardProjectPaths throws away the changes of paths: tracked files return to HEAD and new files
// are deleted. The paths are mandatory, so a request without them can never wipe the whole project.
type DiscardProjectPaths struct {
	roots outbound.WorkspaceRootPort
	git   outbound.GitStatusPort
}

// NewDiscardProjectPaths builds the use case.
func NewDiscardProjectPaths(roots outbound.WorkspaceRootPort, git outbound.GitStatusPort) *DiscardProjectPaths {
	return &DiscardProjectPaths{roots: roots, git: git}
}

// Execute rejects an empty list, resolves the project root, then discards paths.
func (u *DiscardProjectPaths) Execute(ctx context.Context, scope models.ProjectScope, paths []string) error {
	if len(paths) == 0 {
		return treeerrors.InvalidPath("")
	}
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return err
	}
	return u.git.Discard(ctx, root.Path, paths)
}
