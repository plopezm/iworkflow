// Package fs lists and creates folders on the local filesystem.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package fs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// dotPrefix hides dot entries from the listing.
const dotPrefix = "."

// gitDirName marks a folder as a git repository.
const gitDirName = ".git"

// DirectoryBrowser lists folders and creates them. Creation is confined to allowedRoot.
type DirectoryBrowser struct {
	allowedRoot string
}

var _ outbound.DirectoryBrowser = (*DirectoryBrowser)(nil)

// NewDirectoryBrowser builds the browser. allowedRoot is the only folder CreateDirectory may write into.
func NewDirectoryBrowser(allowedRoot string) *DirectoryBrowser {
	return &DirectoryBrowser{allowedRoot: allowedRoot}
}

// List lists path, or its nearest existing ancestor when path does not exist. Only folders are listed,
// hidden ones excluded, sorted case-insensitively.
func (b *DirectoryBrowser) List(_ context.Context, path string) (models.DirectoryListing, error) {
	target, ok := nearestExistingAncestor(path)
	if !ok {
		return models.DirectoryListing{}, werrors.PathNotFound(path)
	}
	entries, err := listEntries(target)
	if err != nil {
		return models.DirectoryListing{}, err
	}
	parent := filepath.Dir(target)
	if parent == target {
		parent = ""
	}
	return models.DirectoryListing{Path: target, Parent: parent, Entries: entries}, nil
}

// CreateDirectory creates name inside parent once both are resolved through symlinks, and only when
// the result stays inside allowedRoot.
func (b *DirectoryBrowser) CreateDirectory(_ context.Context, parent string, name string) (string, error) {
	root, err := filepath.EvalSymlinks(b.allowedRoot)
	if err != nil {
		return "", ioError(b.allowedRoot, err)
	}
	canonicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", werrors.PathNotFound(parent)
		}
		return "", ioError(parent, err)
	}
	if !isWithin(root, canonicalParent) {
		return "", werrors.PathOutsideAllowedRoot(parent)
	}
	target := filepath.Join(canonicalParent, name)
	if err := os.Mkdir(target, 0o777); err != nil {
		return "", ioError(target, err)
	}
	return target, nil
}

// nearestExistingAncestor walks up from path until it finds a folder.
func nearestExistingAncestor(path string) (string, bool) {
	current := path
	for {
		if info, err := os.Stat(current); err == nil && info.IsDir() {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

// listEntries returns the visible sub-folders of dir. Entries that cannot be stat-ed are skipped, as
// the Rust adapter does; a failure to read the folder itself is an IO error.
func listEntries(dir string) ([]models.DirectoryEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, ioError(dir, err)
	}
	entries := make([]models.DirectoryEntry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		if strings.HasPrefix(name, dotPrefix) {
			continue
		}
		entryPath := filepath.Join(dir, name)
		info, err := os.Stat(entryPath)
		if err != nil || !info.IsDir() {
			continue
		}
		_, gitErr := os.Stat(filepath.Join(entryPath, gitDirName))
		entries = append(entries, models.DirectoryEntry{Name: name, Path: entryPath, IsGitRepo: gitErr == nil})
	}
	sort.SliceStable(entries, func(i int, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// isWithin reports whether candidate is root or lies below it, comparing path segments.
func isWithin(root string, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// ioError keeps the Rust wording: "<path>: <error>".
func ioError(path string, err error) error {
	return werrors.IO(fmt.Sprintf("%s: %v", path, err))
}
