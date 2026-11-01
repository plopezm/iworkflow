// Package filesystem lists the folders of the server for the file picker.
package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/domain/ports/outbound"
)

// Browser reads the real file system of the server.
type Browser struct{}

var _ outbound.FileBrowser = Browser{}

// NewBrowser builds the browser.
func NewBrowser() Browser { return Browser{} }

// Browse lists a folder. An empty path is the home folder (the root when there is none); the path of a file
// lists the folder that holds it. Symbolic links are listed as what they point to; a broken one is skipped.
func (Browser) Browse(ctx context.Context, path string) (models.Listing, error) {
	if err := ctx.Err(); err != nil {
		return models.Listing{}, fmt.Errorf("%w: %w", openerrors.ErrBrowseFailed, err)
	}
	dir, err := resolveDir(path)
	if err != nil {
		return models.Listing{}, err
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return models.Listing{}, fmt.Errorf("%w: %w", openerrors.ErrBrowseFailed, err)
	}
	entries := make([]models.FileEntry, 0, len(items))
	for _, item := range items {
		full := filepath.Join(dir, item.Name())
		info, statErr := os.Stat(full)
		if statErr != nil {
			continue
		}
		entries = append(entries, models.FileEntry{Name: item.Name(), Path: full, IsDir: info.IsDir()})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}
	return models.Listing{Path: dir, Parent: parent, Entries: entries}, nil
}

func resolveDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return string(filepath.Separator), nil //nolint:nilerr // no home folder: the root is the sensible start
		}
		return home, nil
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: %q is not an absolute path", openerrors.ErrBrowseFailed, path)
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", openerrors.ErrBrowseFailed, err)
	}
	if !info.IsDir() {
		return filepath.Dir(path), nil
	}
	return path, nil
}
