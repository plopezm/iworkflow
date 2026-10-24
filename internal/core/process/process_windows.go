//go:build windows

package process

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// createNoWindow stops taskkill from opening a console window.
const createNoWindow uint32 = 0x08000000

// KillTree force-kills pid and all its descendants with taskkill. The process may have exited
// already, so callers doing best-effort cleanup can ignore the error.
func KillTree(pid uint32) error {
	return KillTreeContext(context.Background(), pid)
}

// KillTreeContext is KillTree bounded by ctx: a cancelled context stops the taskkill run.
func KillTreeContext(ctx context.Context, pid uint32) error {
	cmd := exec.CommandContext(ctx, "taskkill", taskkillArgs(pid)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("taskkill %d: %w", pid, err)
	}
	return nil
}
