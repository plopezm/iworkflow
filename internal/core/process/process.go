// Package process kills whole process trees: a process group on Unix, `taskkill /T` on Windows.
package process

import "strconv"

// taskkillArgs returns the taskkill arguments that force-kill pid and all its descendants.
// It is pure, so it is tested on every platform even though only Windows runs it.
func taskkillArgs(pid uint32) []string {
	return []string{"/PID", strconv.FormatUint(uint64(pid), 10), "/T", "/F"}
}
