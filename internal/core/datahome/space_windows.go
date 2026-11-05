//go:build windows

package datahome

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// FreeSpace returns the bytes available to the current user on the disk that holds path.
func FreeSpace(path string) (uint64, error) {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("path %s: %w", path, err)
	}
	var free uint64
	if err := windows.GetDiskFreeSpaceEx(pointer, &free, nil, nil); err != nil {
		return 0, fmt.Errorf("free space of %s: %w", path, err)
	}
	return free, nil
}
