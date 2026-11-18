//go:build linux || windows

package process

// withinAny reports whether path is inside any of the roots.
func withinAny(roots []string, path string) bool {
	for _, root := range roots {
		if within(root, path) {
			return true
		}
	}
	return false
}
