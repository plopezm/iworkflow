package application

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// BrowsePathInput is the folder to list. An empty Path, or a relative one, starts from the workspace folder.
type BrowsePathInput struct {
	WorkspaceID string
	Path        string
}

// BrowsePath lists a folder for the file and folder pickers of the hook editor. It writes nothing.
type BrowsePath struct {
	workspaces outbound.WorkspaceLookup
	browser    outbound.PathBrowser
}

// NewBrowsePath builds the use case.
func NewBrowsePath(workspaces outbound.WorkspaceLookup, browser outbound.PathBrowser) *BrowsePath {
	return &BrowsePath{workspaces: workspaces, browser: browser}
}

// Execute lists the folder, resolving an empty or relative path against the workspace folder.
func (u *BrowsePath) Execute(ctx context.Context, in BrowsePathInput) (models.Listing, error) {
	dir := strings.TrimSpace(in.Path)
	if dir == "" || !filepath.IsAbs(dir) {
		base, err := u.workspaces.Path(ctx, in.WorkspaceID)
		if err != nil {
			return models.Listing{}, fmt.Errorf("find workspace %s: %w", in.WorkspaceID, err)
		}
		dir = filepath.Join(base, dir)
	}
	listing, err := u.browser.List(ctx, dir)
	if err != nil {
		return models.Listing{}, fmt.Errorf("browse %s: %w", dir, err)
	}
	return listing, nil
}
