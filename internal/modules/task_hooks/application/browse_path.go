package application

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// BrowsePathInput is the folder to list. An empty or relative Path starts from the home folder of the user.
type BrowsePathInput struct {
	Path string
}

// BrowsePath lists a folder for the file and folder pickers of the hook editor. It writes nothing.
type BrowsePath struct {
	browser outbound.PathBrowser
}

// NewBrowsePath builds the use case.
func NewBrowsePath(browser outbound.PathBrowser) *BrowsePath {
	return &BrowsePath{browser: browser}
}

// Execute lists the folder. A path that is not absolute has no folder of its own to be read from, so the browser
// starts at the home folder, the same as for an empty one.
func (u *BrowsePath) Execute(ctx context.Context, in BrowsePathInput) (models.Listing, error) {
	dir := strings.TrimSpace(in.Path)
	if !filepath.IsAbs(dir) {
		dir = ""
	}
	listing, err := u.browser.List(ctx, dir)
	if err != nil {
		return models.Listing{}, fmt.Errorf("browse %q: %w", dir, err)
	}
	return listing, nil
}
