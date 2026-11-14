package models

import (
	"path/filepath"
	"regexp"
	"strings"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// positionSuffix is the :line or :line:col that an editor-style reference ends with.
var positionSuffix = regexp.MustCompile(`:\d+(?::\d+)?$`)

// launchableExtensions are the files that run when the system opens them. They are never opened from the chat, in any
// operating system: a path written by an agent must not become a way to execute code.
var launchableExtensions = map[string]struct{}{
	".exe": {}, ".com": {}, ".bat": {}, ".cmd": {}, ".msi": {}, ".scr": {}, ".pif": {}, ".lnk": {}, ".url": {},
	".ps1": {}, ".psm1": {}, ".vbs": {}, ".vbe": {}, ".js": {}, ".jse": {}, ".wsf": {}, ".wsh": {}, ".hta": {},
	".reg": {}, ".jar": {}, ".app": {}, ".command": {}, ".desktop": {}, ".sh": {}, ".bash": {}, ".zsh": {},
	".fish": {}, ".run": {}, ".appimage": {}, ".workflow": {}, ".scpt": {}, ".action": {}, ".dll": {}, ".cpl": {},
}

// ResolveOpenPath turns a path written in the chat into the absolute path to open. It drops a trailing :line:col,
// expands a leading ~ with home and resolves a relative path against the worktree of the task. A path that is not
// absolute needs a worktree.
func ResolveOpenPath(raw string, worktree string, home string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" || strings.ContainsRune(path, 0) {
		return "", werrors.ErrOpenPathInvalid
	}
	path = positionSuffix.ReplaceAllString(path, "")
	switch {
	case path == "~":
		path = home
	case strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`):
		path = filepath.Join(home, filepath.FromSlash(strings.ReplaceAll(path[2:], `\`, "/")))
	}
	if path == "" {
		return "", werrors.ErrOpenPathInvalid
	}
	if !filepath.IsAbs(path) {
		if worktree == "" {
			return "", werrors.ErrOpenPathInvalid
		}
		path = filepath.Join(worktree, path)
	}
	path = filepath.Clean(path)
	if _, blocked := launchableExtensions[strings.ToLower(filepath.Ext(path))]; blocked {
		return "", werrors.ErrOpenPathBlocked
	}
	return path, nil
}
