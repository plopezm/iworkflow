//go:build !windows

package desktop

import (
	"fmt"
	"os/exec"
	"syscall"
)

// StartDetached starts exe as an independent process: its own session, so it survives the parent and does
// not receive the signals sent to the parent's terminal. It returns once the process has started.
func StartDetached(exe string, args ...string) error {
	return StartDetachedIn("", exe, args...)
}

// StartDetachedIn is StartDetached with an explicit working directory. The updater helper must not run inside
// the install folder: on Windows a process whose working directory is a folder keeps it from being renamed.
func StartDetachedIn(dir string, exe string, args ...string) error {
	cmd := exec.Command(exe, args...) //nolint:gosec,noctx // exe is the shipped helper or the relaunched app
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", exe, err)
	}
	// The child is never waited for: release it so it is not a zombie of this process.
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("releasing %s: %w", exe, err)
	}
	return nil
}
