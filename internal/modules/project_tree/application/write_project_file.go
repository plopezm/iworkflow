//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// WriteProjectFile saves the edited content of one text file of a workspace or task project.
type WriteProjectFile struct {
	roots outbound.WorkspaceRootPort
	files outbound.FileStorePort
}

// NewWriteProjectFile builds the use case.
func NewWriteProjectFile(roots outbound.WorkspaceRootPort, files outbound.FileStorePort) *WriteProjectFile {
	return &WriteProjectFile{roots: roots, files: files}
}

// Execute resolves the project root, then overwrites relative with content.
func (u *WriteProjectFile) Execute(ctx context.Context, scope models.ProjectScope, relative string, content string) error {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return err
	}
	return u.files.Write(ctx, root.Path, relative, content)
}
