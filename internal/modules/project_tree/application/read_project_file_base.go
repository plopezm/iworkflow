//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// ReadProjectFileBase reads a project file as it is at git HEAD: the baseline for the editor
// change bars.
type ReadProjectFileBase struct {
	roots outbound.WorkspaceRootPort
	git   outbound.GitStatusPort
}

// NewReadProjectFileBase builds the use case.
func NewReadProjectFileBase(roots outbound.WorkspaceRootPort, git outbound.GitStatusPort) *ReadProjectFileBase {
	return &ReadProjectFileBase{roots: roots, git: git}
}

// Execute resolves the project root, then returns the HEAD content of relative, or nil when git
// does not know the file.
func (u *ReadProjectFileBase) Execute(ctx context.Context, scope models.ProjectScope, relative string) (*string, error) {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return nil, err
	}
	return u.git.BaseContent(ctx, root.Path, relative)
}
