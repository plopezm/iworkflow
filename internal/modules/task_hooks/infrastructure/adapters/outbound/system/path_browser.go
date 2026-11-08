package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"iworkflow/internal/modules/task_hooks/domain/models"
)

// maxEntries bounds one listing so a huge folder cannot flood the page.
const maxEntries = 500

// PathBrowser lists folders of the local file system.
type PathBrowser struct{}

// NewPathBrowser builds the browser.
func NewPathBrowser() PathBrowser { return PathBrowser{} }

// List returns the entries of dir, folders first and each group by name. Hidden entries are skipped.
func (PathBrowser) List(ctx context.Context, dir string) (models.Listing, error) {
	if err := ctx.Err(); err != nil {
		return models.Listing{}, fmt.Errorf("list %s: %w", dir, err)
	}
	clean := filepath.Clean(dir)
	items, err := os.ReadDir(clean)
	if err != nil {
		return models.Listing{}, fmt.Errorf("list %s: %w", clean, err)
	}
	entries := make([]models.DirEntry, 0, len(items))
	for _, item := range items {
		if len(item.Name()) > 0 && item.Name()[0] == '.' {
			continue
		}
		path := filepath.Join(clean, item.Name())
		isDir := item.IsDir()
		if !isDir && item.Type()&os.ModeSymlink != 0 {
			if info, statErr := os.Stat(path); statErr == nil {
				isDir = info.IsDir()
			}
		}
		entries = append(entries, models.DirEntry{Name: item.Name(), Path: path, IsDir: isDir})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}
	listing := models.Listing{Path: clean, Entries: entries}
	if parent := filepath.Dir(clean); parent != clean {
		listing.Parent = parent
	}
	return listing, nil
}
