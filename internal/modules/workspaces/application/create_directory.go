//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"path/filepath"
	"strings"
	"unicode"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// maxDirectoryNameBytes is the longest folder name accepted, in bytes (the usual filesystem limit).
const maxDirectoryNameBytes = 255

// CreateDirectory validates the name of a new folder and delegates its creation to the browser port.
type CreateDirectory struct {
	browser outbound.DirectoryBrowser
}

// NewCreateDirectory builds the use case.
func NewCreateDirectory(browser outbound.DirectoryBrowser) *CreateDirectory {
	return &CreateDirectory{browser: browser}
}

// Execute creates name inside parent and returns the new path.
func (u *CreateDirectory) Execute(ctx context.Context, parent string, name string) (string, error) {
	if !filepath.IsAbs(parent) {
		return "", werrors.PathNotAbsolute(parent)
	}
	name = strings.TrimSpace(name)
	if !isValidDirectoryName(name) {
		return "", werrors.InvalidDirectoryName(name)
	}
	return u.browser.CreateDirectory(ctx, parent, name)
}

// isValidDirectoryName accepts one path segment: no separators, no "."/"..", no control characters.
func isValidDirectoryName(name string) bool {
	if name == "" || len(name) > maxDirectoryNameBytes || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
