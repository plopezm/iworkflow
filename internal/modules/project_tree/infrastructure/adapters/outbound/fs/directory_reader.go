//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package fs

import (
	"cmp"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// hiddenEntry is the one entry never listed: git's own metadata.
const hiddenEntry string = ".git"

// DirectoryReader lists one directory level of the local filesystem, confined to the project root.
type DirectoryReader struct{}

var _ outbound.DirectoryReaderPort = (*DirectoryReader)(nil)

// NewDirectoryReader builds the adapter.
func NewDirectoryReader() *DirectoryReader {
	return &DirectoryReader{}
}

// List lists one level of relative (empty means the root). Directories come first, then files,
// both sorted case-insensitively. Symlinks are followed: broken links and links that leave the
// root are skipped.
func (r *DirectoryReader) List(_ context.Context, root string, relative string) (models.DirectoryContents, error) {
	if err := models.ValidateRelative(relative); err != nil {
		return models.DirectoryContents{}, err
	}
	canonicalRoot, err := canonicalize(root, "")
	if err != nil {
		return models.DirectoryContents{}, err
	}
	target, err := canonicalize(filepath.Join(root, relative), relative)
	if err != nil {
		return models.DirectoryContents{}, err
	}
	if !models.IsWithin(canonicalRoot, target) {
		return models.DirectoryContents{}, treeerrors.PathOutsideRoot(relative)
	}
	info, err := os.Stat(target)
	if err != nil {
		return models.DirectoryContents{}, ioError(err)
	}
	if !info.IsDir() {
		return models.DirectoryContents{}, treeerrors.NotADirectory(relative)
	}

	items, err := os.ReadDir(target)
	if err != nil {
		return models.DirectoryContents{}, ioError(err)
	}
	entries := make([]models.TreeEntry, 0, len(items))
	for _, item := range items {
		name := strings.ToValidUTF8(item.Name(), "\uFFFD")
		if name == hiddenEntry {
			continue
		}
		entryPath := filepath.Join(target, item.Name())
		resolved, err := filepath.EvalSymlinks(entryPath)
		if err != nil || !models.IsWithin(canonicalRoot, resolved) {
			continue
		}
		entryInfo, err := os.Stat(entryPath)
		if err != nil {
			continue
		}
		kind := models.TreeEntryFile
		if entryInfo.IsDir() {
			kind = models.TreeEntryDirectory
		}
		entries = append(entries, models.TreeEntry{Name: name, Path: joinRelative(relative, name), Kind: kind})
	}
	slices.SortFunc(entries, compareEntries)
	return models.DirectoryContents{Path: relative, Entries: entries}, nil
}

// joinRelative appends name to a `/`-separated relative directory.
func joinRelative(relative string, name string) string {
	if relative == "" {
		return name
	}
	return relative + "/" + name
}

// entryRank puts directories before files.
func entryRank(kind models.TreeEntryKind) int {
	if kind == models.TreeEntryDirectory {
		return 0
	}
	return 1
}

// compareEntries orders directories first, then by name ignoring case, then by exact name.
func compareEntries(a models.TreeEntry, b models.TreeEntry) int {
	return cmp.Or(
		cmp.Compare(entryRank(a.Kind), entryRank(b.Kind)),
		strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
		strings.Compare(a.Name, b.Name),
	)
}
