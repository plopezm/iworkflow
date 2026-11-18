//go:build windows

package process

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processParents maps every process of a Toolhelp snapshot to its parent pid. The snapshot is taken once, so it
// is consistent with itself; a parent that exited before the snapshot keeps its pid as the recorded parent.
func processParents(ctx context.Context) (map[int]int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshot the processes: %w", err)
	}
	defer windows.CloseHandle(snapshot) //nolint:errcheck // nothing to recover from a failed close
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	parents := map[int]int{}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("scan the processes: %w", ctx.Err())
		}
		parents[int(entry.ProcessID)] = int(entry.ParentProcessID)
	}
	// Enumeration ends with ERROR_NO_MORE_FILES; any other error means the list is incomplete.
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("list the processes: %w", err)
	}
	return parents, nil
}
