//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// UnstageProjectPaths unstages paths of a project so they are left out of the next commit. No
// paths means all of them.
type UnstageProjectPaths struct {
	roots outbound.WorkspaceRootPort
	git   outbound.GitStatusPort
}

// NewUnstageProjectPaths builds the use case.
func NewUnstageProjectPaths(roots outbound.WorkspaceRootPort, git outbound.GitStatusPort) *UnstageProjectPaths {
	return &UnstageProjectPaths{roots: roots, git: git}
}

// Execute resolves the project root, then unstages paths.
func (u *UnstageProjectPaths) Execute(ctx context.Context, scope models.ProjectScope, paths []string) error {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return err
	}
	return u.git.Unstage(ctx, root.Path, paths)
}
