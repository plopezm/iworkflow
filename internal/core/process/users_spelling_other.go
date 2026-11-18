//go:build !windows

package process

// extraSpellings has nothing to add outside Windows, where a path has no 8.3 short form.
func extraSpellings(string) []string { return nil }
