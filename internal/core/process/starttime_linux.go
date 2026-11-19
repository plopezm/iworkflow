//go:build linux

package process

import (
	"os"
	"strconv"
	"strings"
)

// processStartStamp returns the start time of pid in clock ticks since boot (field 22 of /proc/<pid>/stat), or ""
// when the process is gone. Together with the pid it identifies a process even after the pid is reused.
func processStartStamp(pid int) string {
	raw, err := os.ReadFile(procRoot + "/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	return parseStartStamp(string(raw))
}

// parseStartStamp extracts field 22 from a /proc/<pid>/stat line, or "" when the line is malformed. comm
// may hold spaces and parentheses, so the line is split after the last ')'.
func parseStartStamp(stat string) string {
	closing := strings.LastIndexByte(stat, ')')
	if closing < 0 {
		return ""
	}
	fields := strings.Fields(stat[closing+1:])
	// fields[0] is field 3 (state), so field 22 is fields[19].
	if len(fields) < 20 {
		return ""
	}
	return fields[19]
}
