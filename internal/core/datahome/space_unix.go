//go:build !windows

package datahome

import (
	"fmt"
	"syscall"
)

// FreeSpace returns the bytes available to the current user on the disk that holds path.
func FreeSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil //nolint:gosec // Both values are non-negative sizes.
}
