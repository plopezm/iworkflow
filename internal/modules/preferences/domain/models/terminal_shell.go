package models

import (
	"strings"
	"unicode"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
)

// TerminalShell is the program the terminals start. The zero value means "no choice": the platform
// default applies (PowerShell on Windows, $SHELL elsewhere).
type TerminalShell struct {
	path string
}

// ParseTerminalShell validates what the user typed or picked. Blank input is the zero value (the
// default). Surrounding quotes are dropped, as Windows adds them when copying a path.
func ParseTerminalShell(raw string) (TerminalShell, error) {
	path := strings.TrimSpace(raw)
	if len(path) >= 2 && strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) {
		path = strings.TrimSpace(path[1 : len(path)-1])
	}
	if strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return TerminalShell{}, &domainerrors.InvalidShellError{Path: path}
	}
	return TerminalShell{path: path}, nil
}

// Path returns the program path; empty means the platform default.
func (s TerminalShell) Path() string {
	return s.path
}
