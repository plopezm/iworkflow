//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// StageProjectPaths stages paths of a project so they are part of the next commit. No paths means
// all of them.
type StageProjectPaths struct {
	roots outbound.WorkspaceRootPort
	git   outbound.GitStatusPort
}

// NewStageProjectPaths builds the use case.
func NewStageProjectPaths(roots outbound.WorkspaceRootPort, git outbound.GitStatusPort) *StageProjectPaths {
	return &StageProjectPaths{roots: roots, git: git}
}

// Execute resolves the project root, then stages paths.
func (u *StageProjectPaths) Execute(ctx context.Context, scope models.ProjectScope, paths []string) error {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return err
	}
	return u.git.Stage(ctx, root.Path, paths)
}
