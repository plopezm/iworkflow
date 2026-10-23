//go:build unix

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// errInvalidGroup rejects pid 0 and negative values: kill(-0) would signal the caller's own group.
var errInvalidGroup = errors.New("invalid process group")

// SetProcessGroup makes cmd lead a new process group, so KillGroup reaches it and every child it
// starts. Call it before cmd.Start.
func SetProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pgid = 0
}

// KillGroup sends SIGKILL to every member of process group pgid. A group that is already gone is
// not an error.
func KillGroup(pgid int) error {
	if pgid <= 0 {
		return errInvalidGroup
	}
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("kill process group %d: %w", pgid, err)
	}
	return nil
}

// GroupAlive reports whether process group pgid still has at least one member. It sends signal 0,
// which checks existence without delivering anything.
func GroupAlive(pgid int) bool {
	if pgid <= 0 {
		return false
	}
	return syscall.Kill(-pgid, 0) == nil
}
