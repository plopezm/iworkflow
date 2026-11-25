//go:build windows

package desktop

import (
	"os/exec"
	"strconv"
	"syscall"
)

// killTree ends the process and everything it started. On Windows a child keeps running (and keeps its files
// locked) when only its parent is killed, which would make the update unable to replace the install folder.
func killTree(pid int) {
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)) //nolint:gosec,noctx // fixed system tool, numeric pid
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	_ = cmd.Run() //nolint:errcheck // best effort: the plain Kill that follows covers a missing taskkill
}
