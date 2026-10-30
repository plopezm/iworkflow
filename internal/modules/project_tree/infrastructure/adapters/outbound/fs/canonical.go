// Package fs reads and overwrites project files on the local filesystem. Every operation is
// confined to the project root: relative paths are validated, and the resolved target (after
// following symlinks) must stay inside the canonical root.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package fs

import (
	"errors"
	"io/fs"
	"path/filepath"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
)

// canonicalize resolves path to an absolute path without symlinks. label names the path in the
// PathNotFound error, as the Rust version did (the root is reported with an empty label).
func canonicalize(path string, label string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", ioError(err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", treeerrors.PathNotFound(label)
		}
		return "", ioError(err)
	}
	return resolved, nil
}

// ioError maps a filesystem failure to the domain error.
func ioError(err error) error {
	return treeerrors.IO(err.Error())
}
