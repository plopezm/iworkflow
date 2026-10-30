package outbound

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
)

// DirectoryBrowser lists and creates folders on the local filesystem.
type DirectoryBrowser interface {
	// List lists the contents of path. When path does not exist (for example the path of a workspace
	// not created yet), it lists the nearest existing ancestor. It fails with PathNotFound when neither
	// the folder nor any ancestor exists, or with IO on other read errors.
	List(ctx context.Context, path string) (models.DirectoryListing, error)
	// CreateDirectory creates name inside parent and returns its path. parent must exist and stay
	// inside the adapter's allowed root after resolving ".." and symlinks. It fails with PathNotFound
	// when parent does not exist, PathOutsideAllowedRoot when it escapes the root, or IO when the
	// folder already exists or cannot be created.
	CreateDirectory(ctx context.Context, parent string, name string) (string, error)
}
