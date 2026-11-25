//go:build windows

package desktop

import (
	"fmt"
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// StartDetached starts exe as an independent process that survives the parent and has no console.
// It returns once the process has started.
func StartDetached(exe string, args ...string) error {
	return StartDetachedIn("", exe, args...)
}

// StartDetachedIn is StartDetached with an explicit working directory. The updater helper must not run inside
// the install folder: on Windows a process whose working directory is a folder keeps it from being renamed.
func StartDetachedIn(dir string, exe string, args ...string) error {
	cmd := exec.Command(exe, args...) //nolint:gosec,noctx // exe is the shipped helper or the relaunched app
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", exe, err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("releasing %s: %w", exe, err)
	}
	return nil
}
