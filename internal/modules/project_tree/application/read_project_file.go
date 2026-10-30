//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// ReadProjectFile reads one text file of a workspace or task project.
type ReadProjectFile struct {
	roots outbound.WorkspaceRootPort
	files outbound.FileStorePort
}

// NewReadProjectFile builds the use case.
func NewReadProjectFile(roots outbound.WorkspaceRootPort, files outbound.FileStorePort) *ReadProjectFile {
	return &ReadProjectFile{roots: roots, files: files}
}

// Execute resolves the project root, then reads relative.
func (u *ReadProjectFile) Execute(ctx context.Context, scope models.ProjectScope, relative string) (models.ProjectFile, error) {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return models.ProjectFile{}, err
	}
	return u.files.Read(ctx, root.Path, relative)
}
