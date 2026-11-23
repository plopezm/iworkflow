//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// ReadProjectImage reads one image file of a workspace or task project.
type ReadProjectImage struct {
	roots  outbound.WorkspaceRootPort
	images outbound.ImageReaderPort
}

// NewReadProjectImage builds the use case.
func NewReadProjectImage(roots outbound.WorkspaceRootPort, images outbound.ImageReaderPort) *ReadProjectImage {
	return &ReadProjectImage{roots: roots, images: images}
}

// Execute resolves the project root, then reads the image at relative.
func (u *ReadProjectImage) Execute(ctx context.Context, scope models.ProjectScope, relative string) (models.ProjectImage, error) {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return models.ProjectImage{}, err
	}
	return u.images.ReadImage(ctx, root.Path, relative)
}

// Check resolves the project root and reports why the image at relative cannot be shown, without reading it.
func (u *ReadProjectImage) Check(ctx context.Context, scope models.ProjectScope, relative string) error {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return err
	}
	return u.images.CheckImage(ctx, root.Path, relative)
}
