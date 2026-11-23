package models

import (
	"path"
	"strings"
)

// ProjectImage is an image file of a project: its bytes and its media type.
type ProjectImage struct {
	ContentType string
	Content     []byte
}

// ImageContentType returns the media type of an image path, judged by its extension, and false when
// the path is not an image the viewer shows (pure).
func ImageContentType(relative string) (string, bool) {
	switch strings.ToLower(path.Ext(relative)) {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".gif":
		return "image/gif", true
	case ".webp":
		return "image/webp", true
	case ".avif":
		return "image/avif", true
	case ".bmp":
		return "image/bmp", true
	case ".ico":
		return "image/x-icon", true
	case ".svg":
		return "image/svg+xml", true
	default:
		return "", false
	}
}
