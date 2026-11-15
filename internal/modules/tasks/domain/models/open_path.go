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

// MaxPreviewImageBytes bounds the size of an image that the chat previews in the page.
const MaxPreviewImageBytes int64 = 20 << 20

// previewImageTypes are the images the page can preview, by extension. SVG is left out on purpose: it can carry scripts.
var previewImageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
}

// PreviewImage is an image of the disk that the chat shows in the page.
type PreviewImage struct {
	ContentType string
	Data        []byte
}

// ResolvePreviewImage resolves a path of the chat like ResolveOpenPath and returns, with it, the content type of the
// image. A path that is not one of the previewable image kinds is ErrOpenPathBlocked.
func ResolvePreviewImage(raw string, worktree string, home string) (string, string, error) {
	path, err := ResolveOpenPath(raw, worktree, home)
	if err != nil {
		return "", "", err
	}
	contentType, ok := previewImageTypes[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return "", "", werrors.ErrOpenPathBlocked
	}
	return path, contentType, nil
}
