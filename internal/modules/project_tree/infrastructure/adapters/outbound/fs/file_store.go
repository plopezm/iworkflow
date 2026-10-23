//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package fs

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"unicode/utf8"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// maxFileBytes is the largest file the viewer and the editor load, in bytes.
const maxFileBytes int64 = 2 * 1024 * 1024

// FileStore reads and overwrites existing text files of the local filesystem, confined to the
// project root.
type FileStore struct{}

var _ outbound.FileStorePort = (*FileStore)(nil)

// NewFileStore builds the adapter.
func NewFileStore() *FileStore {
	return &FileStore{}
}

// Read returns the UTF-8 content of the regular file at relative.
func (s *FileStore) Read(_ context.Context, root string, relative string) (models.ProjectFile, error) {
	target, err := resolveFile(root, relative)
	if err != nil {
		return models.ProjectFile{}, err
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return models.ProjectFile{}, ioError(err)
	}
	if !utf8.Valid(content) {
		return models.ProjectFile{}, treeerrors.NotText(relative)
	}
	return models.ProjectFile{Name: path.Base(relative), Path: relative, Content: string(content)}, nil
}

// Write overwrites an existing regular file. It never creates files: a missing path is an error.
func (s *FileStore) Write(_ context.Context, root string, relative string, content string) error {
	if int64(len(content)) > maxFileBytes {
		return treeerrors.FileTooLarge(relative)
	}
	target, err := resolveFile(root, relative)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return ioError(err)
	}
	return nil
}

// resolveFile returns the canonical path of an existing regular file inside root that is not
// larger than maxFileBytes.
func resolveFile(root string, relative string) (string, error) {
	if relative == "" {
		return "", treeerrors.NotAFile("")
	}
	if err := models.ValidateRelative(relative); err != nil {
		return "", err
	}
	canonicalRoot, err := canonicalize(root, "")
	if err != nil {
		return "", err
	}
	target, err := canonicalize(filepath.Join(root, relative), relative)
	if err != nil {
		return "", err
	}
	if !models.IsWithin(canonicalRoot, target) {
		return "", treeerrors.PathOutsideRoot(relative)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", ioError(err)
	}
	if !info.Mode().IsRegular() {
		return "", treeerrors.NotAFile(relative)
	}
	if info.Size() > maxFileBytes {
		return "", treeerrors.FileTooLarge(relative)
	}
	return target, nil
}
