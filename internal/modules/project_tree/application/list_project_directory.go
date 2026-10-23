//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// ListProjectDirectory lists one directory level of a workspace or task project.
type ListProjectDirectory struct {
	roots  outbound.WorkspaceRootPort
	reader outbound.DirectoryReaderPort
}

// NewListProjectDirectory builds the use case.
func NewListProjectDirectory(roots outbound.WorkspaceRootPort, reader outbound.DirectoryReaderPort) *ListProjectDirectory {
	return &ListProjectDirectory{roots: roots, reader: reader}
}

// Execute resolves the project root, then lists relative (empty means the root).
func (u *ListProjectDirectory) Execute(ctx context.Context, scope models.ProjectScope, relative string) (models.ProjectDirectory, error) {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return models.ProjectDirectory{}, err
	}
	contents, err := u.reader.List(ctx, root.Path, relative)
	if err != nil {
		return models.ProjectDirectory{}, err
	}
	return models.ProjectDirectory{WorkspaceName: root.Name, Contents: contents}, nil
}
