//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"path/filepath"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// BrowseDirectories resolves which folder to list (the home folder by default) and delegates the
// listing to the browser port.
type BrowseDirectories struct {
	browser outbound.DirectoryBrowser
	homeDir string
}

// NewBrowseDirectories builds the use case.
func NewBrowseDirectories(browser outbound.DirectoryBrowser, homeDir string) *BrowseDirectories {
	return &BrowseDirectories{browser: browser, homeDir: homeDir}
}

// Execute lists path. An empty path means the home folder; any other path must be absolute.
func (u *BrowseDirectories) Execute(ctx context.Context, path string) (models.DirectoryListing, error) {
	target := u.homeDir
	if path != "" {
		if !filepath.IsAbs(path) {
			return models.DirectoryListing{}, werrors.PathNotAbsolute(path)
		}
		target = path
	}
	return u.browser.List(ctx, target)
}
