package application

import (
	"context"
	"fmt"
	"strings"

	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/domain/ports/outbound"
)

// maxBrowseEntries bounds the entries of a listing; a folder like /usr/bin holds thousands.
const maxBrowseEntries = 500

// BrowseFiles lists a folder of the server so the user can pick an executable.
type BrowseFiles struct {
	browser outbound.FileBrowser
}

// NewBrowseFiles builds the use case.
func NewBrowseFiles(browser outbound.FileBrowser) *BrowseFiles {
	return &BrowseFiles{browser: browser}
}

// Execute lists the folder. Hidden entries (a leading dot) are left out unless the filter itself starts with a
// dot; a non-empty filter keeps the entries whose name contains it, ignoring case.
func (u *BrowseFiles) Execute(ctx context.Context, path string, filter string) (models.Listing, error) {
	listing, err := u.browser.Browse(ctx, path)
	if err != nil {
		return models.Listing{}, fmt.Errorf("browse %q: %w", path, err)
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	showHidden := strings.HasPrefix(filter, ".")
	kept := make([]models.FileEntry, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		if !showHidden && strings.HasPrefix(entry.Name, ".") {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(entry.Name), filter) {
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) > maxBrowseEntries {
		kept = kept[:maxBrowseEntries]
		listing.Truncated = true
	}
	listing.Entries = kept
	return listing, nil
}
