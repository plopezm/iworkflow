//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"path/filepath"
	"strings"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
)

// ValidateRelative accepts the empty string (the project root) and plain `/`-separated names only:
// no absolute paths, no empty segments, no `.` or `..` segments and no volume names. This is the
// first containment barrier; the adapters still check the resolved path against the root.
func ValidateRelative(relative string) error {
	if relative == "" {
		return nil
	}
	for _, segment := range strings.Split(relative, "/") {
		if segment == "." || !filepath.IsLocal(segment) {
			return treeerrors.InvalidPath(relative)
		}
	}
	return nil
}

// IsWithin reports whether target is root itself or lies below it. The comparison works on whole
// path components, so "/work/app2" is not inside "/work/app". Both paths must already be
// canonical (absolute, without symlinks).
func IsWithin(root string, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
