//go:build unix && !linux

package process

import (
	"os/exec"
	"strconv"
	"strings"
)

// processStartStamp returns the start time of pid as printed by ps, or "" when the process is gone.
func processStartStamp(pid int) string {
	out, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output() //nolint:noctx // bounded, local, instant
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
